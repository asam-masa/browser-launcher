package chromewindowplacement

import (
	"context"
	"errors"
	"fmt"

	launchcondition "github.com/asam-masa/browser-launcher/internal/application/launchcondition"
	domain "github.com/asam-masa/browser-launcher/internal/domain/launchcondition"
)

var (
	ErrInvalidRequest  = errors.New("chrome window placement request is invalid")
	ErrPlacementFailed = errors.New("chrome window placement failed")
	ErrBoundsMismatch  = errors.New("actual window bounds do not match requested bounds")
	ErrInvalidResult   = errors.New("chrome window placer returned an invalid result")
	ErrFinalValidation = errors.New("final Chrome window bounds are invalid for the target work area")
)

type Service struct {
	placer Placer
}

func NewService(placer Placer) Service {
	return Service{placer: placer}
}

func (s Service) Place(ctx context.Context, request Request) (Result, error) {
	result := Result{Stage: StageValidation, Requested: request.InitialTarget}
	if ctx == nil {
		return result, errors.Join(ErrInvalidRequest, errors.New("context is nil"))
	}
	if err := validateRequest(request); err != nil {
		return result, errors.Join(ErrInvalidRequest, err)
	}
	if s.placer == nil {
		return result, errors.Join(ErrPlacementFailed, errors.New("placer is not configured"))
	}

	resolver := finalBoundsResolver(request.LogicalRequested)
	result, err := s.placer.Place(ctx, request, resolver)
	if result.Requested == (Rectangle{}) {
		result.Requested = request.InitialTarget
	}
	if err != nil {
		result.Matched = false
		if !isFailureStage(result.Stage) {
			return result, errors.Join(ErrPlacementFailed, ErrInvalidResult, err)
		}
		if stageRequiresActualBounds(result.Stage) && !result.ActualKnown {
			return result, errors.Join(
				ErrPlacementFailed,
				ErrInvalidResult,
				err,
				errors.New("last measured bounds are not available"),
			)
		}
		if result.ActualKnown {
			if boundsErr := validateRectangle(result.Actual); boundsErr != nil {
				return result, errors.Join(ErrPlacementFailed, ErrInvalidResult, err, boundsErr)
			}
		}
		return result, errors.Join(ErrPlacementFailed, err)
	}

	if result.Stage != StageFinalMeasurement {
		return result, errors.Join(
			ErrPlacementFailed,
			ErrInvalidResult,
			errors.New("final measurement was not completed"),
		)
	}
	if !result.ActualKnown {
		return result, errors.Join(ErrPlacementFailed, ErrInvalidResult, errors.New("actual bounds are not available"))
	}
	if err := validateRectangle(result.Actual); err != nil {
		return result, errors.Join(ErrPlacementFailed, ErrInvalidResult, err)
	}
	if result.Actual != result.Requested {
		result.Stage = StageFinalMeasurement
		result.Matched = false
		return result, errors.Join(ErrPlacementFailed, ErrBoundsMismatch)
	}

	result.Stage = StageCompleted
	result.Matched = true
	result.PartiallyChanged = false
	return result, nil
}

func finalBoundsResolver(condition domain.Condition) FinalBoundsResolver {
	return func(context FinalPlacementContext) (Rectangle, []ValidationError, error) {
		bounds, fieldErrors, err := launchcondition.ResolvePhysicalBounds(condition, launchcondition.PrimaryWorkArea{
			MonitorLeft: context.MonitorLeft,
			MonitorTop:  context.MonitorTop,
			WorkLeft:    context.WorkLeft,
			WorkTop:     context.WorkTop,
			WorkWidth:   context.WorkWidth,
			WorkHeight:  context.WorkHeight,
			DPI:         context.DPI,
		})
		validationErrors := make([]ValidationError, len(fieldErrors))
		for index, fieldError := range fieldErrors {
			validationErrors[index] = ValidationError{
				Field:   string(fieldError.Field),
				Message: fieldError.Message,
			}
		}
		if err != nil {
			return Rectangle{}, validationErrors, errors.Join(ErrFinalValidation, err)
		}
		if len(validationErrors) > 0 {
			return Rectangle{}, validationErrors, ErrFinalValidation
		}
		return Rectangle{
			X:      bounds.X,
			Y:      bounds.Y,
			Width:  bounds.Width,
			Height: bounds.Height,
		}, nil, nil
	}
}

func isFailureStage(stage Stage) bool {
	switch stage {
	case StageInitialMeasurement,
		StageMoveToTarget,
		StageWaitForStability,
		StageFinalPlacement,
		StageFinalMeasurement:
		return true
	default:
		return false
	}
}

func stageRequiresActualBounds(stage Stage) bool {
	switch stage {
	case StageMoveToTarget,
		StageWaitForStability,
		StageFinalPlacement,
		StageFinalMeasurement:
		return true
	default:
		return false
	}
}

func validateRequest(request Request) error {
	if request.Window.Handle == 0 {
		return errors.New("window handle must be greater than zero")
	}
	if request.Window.ProcessID == 0 {
		return errors.New("process ID must be greater than zero")
	}
	if request.Window.ProcessStartTime == 0 {
		return errors.New("process start time must be greater than zero")
	}
	if request.LogicalRequested.Width.Value() < 1 || request.LogicalRequested.Height.Value() < 1 ||
		request.LogicalRequested.X.Value() < 0 || request.LogicalRequested.Y.Value() < 0 {
		return errors.New("logical requested bounds are invalid")
	}
	if err := validateRectangle(request.InitialTarget); err != nil {
		return fmt.Errorf("validate initial target bounds: %w", err)
	}
	return nil
}

func validateRectangle(rectangle Rectangle) error {
	_, err := domain.NewBounds(rectangle.X, rectangle.Y, rectangle.Width, rectangle.Height)
	return err
}
