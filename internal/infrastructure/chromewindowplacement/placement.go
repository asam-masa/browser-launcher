package chromewindowplacement

import (
	"context"
	"errors"
	"fmt"

	placement "github.com/asam-masa/browser-launcher/internal/application/chromewindowplacement"
	tracking "github.com/asam-masa/browser-launcher/internal/application/chromewindowtracking"
)

const (
	minWindowsCoordinate = -1 << 31
	maxWindowsCoordinate = 1<<31 - 1
)

type measurement struct {
	bounds      placement.Rectangle
	dpi         uint32
	monitor     uintptr
	monitorLeft int
	monitorTop  int
	workLeft    int
	workTop     int
	workWidth   int
	workHeight  int
}

type targetMonitor struct {
	handle uintptr
	workX  int
	workY  int
}

type windowSystem interface {
	BeginDPIAware() (func() error, error)
	TargetMonitor(placement.Rectangle) (targetMonitor, error)
	Measure(uint64) (measurement, error)
	Move(uint64, placement.Rectangle, bool) error
}

func (p Provider) Place(ctx context.Context, request placement.Request, resolveFinalBounds placement.FinalBoundsResolver) (result placement.Result, resultErr error) {
	result = placement.Result{Stage: placement.StageInitialMeasurement, Requested: request.InitialTarget}
	if ctx == nil {
		return result, errors.Join(ErrPlacementFailed, errors.New("context is nil"))
	}
	if err := p.validateProvider(resolveFinalBounds); err != nil {
		return result, errors.Join(ErrPlacementFailed, err)
	}

	restore, err := p.system.BeginDPIAware()
	if err != nil {
		return result, errors.Join(ErrPlacementFailed, fmt.Errorf("enter DPI-aware context: %w", err))
	}
	defer func() {
		if restoreErr := restore(); restoreErr != nil {
			resultErr = errors.Join(resultErr, ErrPlacementFailed, fmt.Errorf("restore DPI-aware context: %w", restoreErr))
		}
	}()

	if err := p.validateIdentity(request.Window); err != nil {
		return result, errors.Join(ErrPlacementFailed, err)
	}
	current, err := p.measure(request.Window)
	if err != nil {
		return result, errors.Join(ErrPlacementFailed, fmt.Errorf("measure initial bounds: %w", err))
	}
	setActual(&result, current)

	target, err := p.system.TargetMonitor(request.InitialTarget)
	if err != nil {
		return result, errors.Join(ErrPlacementFailed, fmt.Errorf("resolve target monitor: %w", err))
	}

	result.Stage = placement.StageMoveToTarget
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if err := p.validateIdentity(request.Window); err != nil {
		return result, errors.Join(ErrPlacementFailed, err)
	}
	moveBounds := current.bounds
	moveBounds.X = target.workX
	moveBounds.Y = target.workY
	if err := ctx.Err(); err != nil {
		return result, err
	}
	result.PartiallyChanged = true
	if err := p.system.Move(request.Window.Handle, moveBounds, false); err != nil {
		return result, errors.Join(ErrPlacementFailed, fmt.Errorf("move to target monitor: %w", err))
	}

	result.Stage = placement.StageWaitForStability
	stable, err := p.waitForStability(ctx, request.Window, target.handle, &result)
	if err != nil {
		return result, errors.Join(ErrPlacementFailed, fmt.Errorf("wait after target move: %w", err))
	}
	setActual(&result, stable)

	result.Stage = placement.StageFinalPlacement
	if err := ctx.Err(); err != nil {
		return result, err
	}
	finalBounds, validationErrors, err := resolveFinalBounds(placement.FinalPlacementContext{
		MonitorLeft: stable.monitorLeft,
		MonitorTop:  stable.monitorTop,
		WorkLeft:    stable.workLeft,
		WorkTop:     stable.workTop,
		WorkWidth:   stable.workWidth,
		WorkHeight:  stable.workHeight,
		DPI:         int(stable.dpi),
	})
	result.ValidationErrors = validationErrors
	if err != nil {
		return result, errors.Join(ErrPlacementFailed, fmt.Errorf("resolve final bounds: %w", err))
	}
	result.Requested = finalBounds
	if err := p.validateIdentity(request.Window); err != nil {
		return result, errors.Join(ErrPlacementFailed, err)
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if err := p.system.Move(request.Window.Handle, finalBounds, true); err != nil {
		return result, errors.Join(ErrPlacementFailed, fmt.Errorf("apply final bounds: %w", err))
	}

	result.Stage = placement.StageFinalMeasurement
	final, err := p.waitForStability(ctx, request.Window, target.handle, &result)
	if err != nil {
		return result, errors.Join(ErrPlacementFailed, fmt.Errorf("wait after final placement: %w", err))
	}
	setActual(&result, final)
	return result, nil
}

func (p Provider) validateProvider(resolveFinalBounds placement.FinalBoundsResolver) error {
	if p.validator == nil || p.system == nil || p.wait == nil ||
		resolveFinalBounds == nil || p.stabilityInterval <= 0 || p.stabilityTimeout < p.stabilityInterval {
		return errProviderUninitialized
	}
	return nil
}

func (p Provider) validateIdentity(window tracking.Window) error {
	valid, err := p.validator.Validate(window)
	if err != nil {
		return fmt.Errorf("validate window identity: %w", err)
	}
	if !valid {
		return ErrIdentityChanged
	}
	return nil
}

func (p Provider) measure(window tracking.Window) (measurement, error) {
	if err := p.validateIdentity(window); err != nil {
		return measurement{}, err
	}
	return p.system.Measure(window.Handle)
}

func (p Provider) waitForStability(ctx context.Context, window tracking.Window, target uintptr, result *placement.Result) (measurement, error) {
	maxSamples := int(p.stabilityTimeout / p.stabilityInterval)
	var previous measurement
	stableSamples := 0
	for sample := 0; sample < maxSamples; sample++ {
		if err := ctx.Err(); err != nil {
			return measurement{}, err
		}
		measured, err := p.measure(window)
		if err != nil {
			return measurement{}, err
		}
		setActual(result, measured)

		if measured.monitor == target && measured == previous {
			stableSamples++
		} else if measured.monitor == target {
			stableSamples = 1
		} else {
			stableSamples = 0
		}
		if stableSamples >= requiredStableSamples {
			return measured, nil
		}
		previous = measured
		if err := p.wait(ctx, p.stabilityInterval); err != nil {
			return measurement{}, err
		}
	}
	return measurement{}, ErrStabilityTimeout
}

func setActual(result *placement.Result, measured measurement) {
	result.Actual = measured.bounds
	result.ActualKnown = true
}

func validateWindowsRectangle(bounds placement.Rectangle) error {
	if bounds.X < minWindowsCoordinate || bounds.X > maxWindowsCoordinate ||
		bounds.Y < minWindowsCoordinate || bounds.Y > maxWindowsCoordinate ||
		bounds.Width < 1 || bounds.Width > maxWindowsCoordinate ||
		bounds.Height < 1 || bounds.Height > maxWindowsCoordinate ||
		bounds.X > maxWindowsCoordinate-bounds.Width ||
		bounds.Y > maxWindowsCoordinate-bounds.Height {
		return ErrBoundsOutOfRange
	}
	return nil
}
