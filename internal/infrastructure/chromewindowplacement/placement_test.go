package chromewindowplacement

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	placement "github.com/asam-masa/browser-launcher/internal/application/chromewindowplacement"
	tracking "github.com/asam-masa/browser-launcher/internal/application/chromewindowtracking"
)

func TestProviderPlaceMovesWithoutResizeThenAppliesFinalBounds(t *testing.T) {
	t.Parallel()

	requested := placement.Rectangle{X: 100, Y: 200, Width: 1000, Height: 700}
	initial := measurement{bounds: placement.Rectangle{X: 2000, Y: 0, Width: 1500, Height: 1050}, dpi: 144, monitor: 2}
	intermediate := measurement{bounds: placement.Rectangle{X: 0, Y: 0, Width: 1500, Height: 1050}, dpi: 96, monitor: 1}
	final := measurement{bounds: requested, dpi: 96, monitor: 1}
	system := &stubWindowSystem{
		target:       targetMonitor{handle: 1, workX: 0, workY: 0},
		measurements: []measurement{initial, intermediate, intermediate, intermediate, final, final, final},
	}
	validator := &stubIdentityValidator{valid: true}
	provider := testProvider(validator, system)

	result, err := provider.Place(context.Background(), placement.Request{Window: testWindow(), InitialTarget: requested}, fixedBoundsResolver(requested))
	if err != nil {
		t.Fatalf("Place() error = %v", err)
	}
	if result.Stage != placement.StageFinalMeasurement || !result.ActualKnown || result.Actual != requested {
		t.Fatalf("Place() result = %+v", result)
	}
	wantMoves := []moveCall{
		{handle: 10, bounds: placement.Rectangle{X: 0, Y: 0, Width: 1500, Height: 1050}, resize: false},
		{handle: 10, bounds: requested, resize: true},
	}
	if !reflect.DeepEqual(system.moves, wantMoves) {
		t.Fatalf("Move() calls = %+v, want %+v", system.moves, wantMoves)
	}
	if !system.restored {
		t.Fatal("DPI-aware context was not restored")
	}
}

func TestProviderPlaceRejectsIdentityChangeBeforeMutation(t *testing.T) {
	t.Parallel()

	validator := &stubIdentityValidator{validSequence: []bool{true, true, false}}
	system := &stubWindowSystem{
		target:       targetMonitor{handle: 1},
		measurements: []measurement{{bounds: placement.Rectangle{X: 1, Y: 2, Width: 3, Height: 4}, dpi: 96, monitor: 1}},
	}
	provider := testProvider(validator, system)

	requested := placement.Rectangle{X: 10, Y: 20, Width: 30, Height: 40}
	result, err := provider.Place(context.Background(), placement.Request{Window: testWindow(), InitialTarget: requested}, fixedBoundsResolver(requested))
	if !errors.Is(err, ErrIdentityChanged) {
		t.Fatalf("Place() error = %v, want ErrIdentityChanged", err)
	}
	if len(system.moves) != 0 {
		t.Fatalf("Move() calls = %d, want 0", len(system.moves))
	}
	if result.PartiallyChanged {
		t.Fatal("PartiallyChanged = true before mutation")
	}
}

func TestProviderPlaceDoesNotMoveWhenContextEndsDuringIdentityValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		cancelAtCall  int
		measurements  []measurement
		wantMoveCount int
		wantStage     placement.Stage
		wantPartial   bool
	}{
		{
			name:         "before target move",
			cancelAtCall: 3,
			measurements: []measurement{
				{bounds: placement.Rectangle{X: 2000, Y: 0, Width: 500, Height: 400}, dpi: 144, monitor: 2},
			},
			wantStage: placement.StageMoveToTarget,
		},
		{
			name:         "before final move",
			cancelAtCall: 7,
			measurements: []measurement{
				{bounds: placement.Rectangle{X: 2000, Y: 0, Width: 500, Height: 400}, dpi: 144, monitor: 2},
				{bounds: placement.Rectangle{X: 0, Y: 0, Width: 500, Height: 400}, dpi: 96, monitor: 1},
				{bounds: placement.Rectangle{X: 0, Y: 0, Width: 500, Height: 400}, dpi: 96, monitor: 1},
				{bounds: placement.Rectangle{X: 0, Y: 0, Width: 500, Height: 400}, dpi: 96, monitor: 1},
			},
			wantMoveCount: 1,
			wantStage:     placement.StageFinalPlacement,
			wantPartial:   true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			validator := &stubIdentityValidator{valid: true}
			validator.afterValidate = func(call int) {
				if call == test.cancelAtCall {
					cancel()
				}
			}
			system := &stubWindowSystem{
				target:       targetMonitor{handle: 1},
				measurements: test.measurements,
			}
			provider := testProvider(validator, system)
			requested := placement.Rectangle{X: 100, Y: 100, Width: 500, Height: 400}

			result, err := provider.Place(ctx, placement.Request{Window: testWindow(), InitialTarget: requested}, fixedBoundsResolver(requested))

			if !errors.Is(err, context.Canceled) {
				t.Fatalf("Place() error = %v, want context.Canceled", err)
			}
			if result.Stage != test.wantStage {
				t.Fatalf("Place() stage = %q, want %q", result.Stage, test.wantStage)
			}
			if len(system.moves) != test.wantMoveCount {
				t.Fatalf("Move() calls = %d, want %d", len(system.moves), test.wantMoveCount)
			}
			if result.PartiallyChanged != test.wantPartial {
				t.Fatalf("PartiallyChanged = %t, want %t", result.PartiallyChanged, test.wantPartial)
			}
		})
	}
}

func TestProviderPlacePreservesLastMeasurementOnStabilityTimeout(t *testing.T) {
	t.Parallel()

	initial := measurement{bounds: placement.Rectangle{X: 10, Y: 10, Width: 500, Height: 400}, dpi: 144, monitor: 2}
	last := measurement{bounds: placement.Rectangle{X: 0, Y: 0, Width: 500, Height: 400}, dpi: 96, monitor: 2}
	system := &stubWindowSystem{
		target:       targetMonitor{handle: 1},
		measurements: []measurement{initial, last, last, last, last},
	}
	provider := testProvider(&stubIdentityValidator{valid: true}, system)
	provider.stabilityTimeout = 4 * time.Millisecond
	provider.stabilityInterval = time.Millisecond

	requested := placement.Rectangle{X: 100, Y: 100, Width: 500, Height: 400}
	result, err := provider.Place(context.Background(), placement.Request{Window: testWindow(), InitialTarget: requested}, fixedBoundsResolver(requested))
	if !errors.Is(err, ErrStabilityTimeout) {
		t.Fatalf("Place() error = %v, want ErrStabilityTimeout", err)
	}
	if result.Stage != placement.StageWaitForStability || !result.PartiallyChanged || result.Actual != last.bounds {
		t.Fatalf("Place() result = %+v", result)
	}
	if len(system.moves) != 1 {
		t.Fatalf("Move() calls = %d, want 1", len(system.moves))
	}
}

func TestProviderPlaceStopsStabilityWaitWhenContextIsCancelled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	initial := measurement{bounds: placement.Rectangle{X: 10, Y: 10, Width: 500, Height: 400}, dpi: 144, monitor: 2}
	afterMove := measurement{bounds: placement.Rectangle{X: 0, Y: 0, Width: 500, Height: 400}, dpi: 96, monitor: 1}
	system := &stubWindowSystem{
		target:       targetMonitor{handle: 1},
		measurements: []measurement{initial, afterMove},
	}
	provider := testProvider(&stubIdentityValidator{valid: true}, system)
	provider.wait = func(context.Context, time.Duration) error {
		cancel()
		return ctx.Err()
	}

	requested := placement.Rectangle{X: 100, Y: 100, Width: 500, Height: 400}
	result, err := provider.Place(ctx, placement.Request{Window: testWindow(), InitialTarget: requested}, fixedBoundsResolver(requested))

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Place() error = %v, want context.Canceled", err)
	}
	if result.Stage != placement.StageWaitForStability || !result.PartiallyChanged ||
		!result.ActualKnown || result.Actual != afterMove.bounds {
		t.Fatalf("Place() result = %+v, want partial stability result", result)
	}
	if len(system.moves) != 1 {
		t.Fatalf("Move() calls = %d, want 1", len(system.moves))
	}
}

func TestProviderPlaceTreatsFailedMutationAsPartialChange(t *testing.T) {
	t.Parallel()

	system := &stubWindowSystem{
		target:       targetMonitor{handle: 1},
		measurements: []measurement{{bounds: placement.Rectangle{X: 10, Y: 10, Width: 500, Height: 400}, dpi: 96, monitor: 1}},
		moveErr:      errors.New("access denied"),
	}
	provider := testProvider(&stubIdentityValidator{valid: true}, system)

	requested := placement.Rectangle{X: 100, Y: 100, Width: 500, Height: 400}
	result, err := provider.Place(context.Background(), placement.Request{Window: testWindow(), InitialTarget: requested}, fixedBoundsResolver(requested))
	if err == nil {
		t.Fatal("Place() error = nil, want failure")
	}
	if !result.PartiallyChanged || !result.ActualKnown {
		t.Fatalf("Place() result = %+v", result)
	}
}

func TestProviderPlaceRejectsFinalBoundsBeforeFinalMove(t *testing.T) {
	t.Parallel()

	requested := placement.Rectangle{X: 100, Y: 100, Width: 500, Height: 400}
	initial := measurement{bounds: placement.Rectangle{X: 2000, Y: 0, Width: 500, Height: 400}, dpi: 144, monitor: 2}
	stable := measurement{
		bounds:      placement.Rectangle{X: 0, Y: 0, Width: 500, Height: 400},
		dpi:         96,
		monitor:     1,
		monitorLeft: 0,
		monitorTop:  0,
		workLeft:    0,
		workTop:     48,
		workWidth:   1920,
		workHeight:  1032,
	}
	system := &stubWindowSystem{
		target:       targetMonitor{handle: 1},
		measurements: []measurement{initial, stable, stable, stable},
	}
	provider := testProvider(&stubIdentityValidator{valid: true}, system)
	validationErrors := []placement.ValidationError{{Field: "width", Message: "幅を小さくしてください。"}}
	resolverErr := errors.New("outside work area")
	resolver := func(context placement.FinalPlacementContext) (placement.Rectangle, []placement.ValidationError, error) {
		wantContext := placement.FinalPlacementContext{
			MonitorLeft: 0,
			MonitorTop:  0,
			WorkLeft:    0,
			WorkTop:     48,
			WorkWidth:   1920,
			WorkHeight:  1032,
			DPI:         96,
		}
		if context != wantContext {
			t.Fatalf("final placement context = %+v, want %+v", context, wantContext)
		}
		return placement.Rectangle{}, validationErrors, resolverErr
	}

	result, err := provider.Place(context.Background(), placement.Request{Window: testWindow(), InitialTarget: requested}, resolver)
	if !errors.Is(err, resolverErr) {
		t.Fatalf("Place() error = %v, want resolver error", err)
	}
	if result.Stage != placement.StageFinalPlacement || !result.PartiallyChanged || result.Actual != stable.bounds {
		t.Fatalf("Place() result = %+v, want final validation failure", result)
	}
	if !reflect.DeepEqual(result.ValidationErrors, validationErrors) {
		t.Fatalf("validation errors = %+v, want %+v", result.ValidationErrors, validationErrors)
	}
	if len(system.moves) != 1 || system.moves[0].resize {
		t.Fatalf("Move() calls = %+v, want only the initial no-resize move", system.moves)
	}
}

func TestProviderPlaceJoinsRestoreFailure(t *testing.T) {
	t.Parallel()

	restoreErr := errors.New("restore failed")
	system := &stubWindowSystem{
		restoreErr:   restoreErr,
		target:       targetMonitor{handle: 1},
		measurements: []measurement{{bounds: placement.Rectangle{X: 1, Y: 1, Width: 100, Height: 100}, dpi: 96, monitor: 1}},
		moveErr:      errors.New("move failed"),
	}
	provider := testProvider(&stubIdentityValidator{valid: true}, system)

	requested := placement.Rectangle{X: 2, Y: 2, Width: 100, Height: 100}
	_, err := provider.Place(context.Background(), placement.Request{Window: testWindow(), InitialTarget: requested}, fixedBoundsResolver(requested))
	if !errors.Is(err, restoreErr) {
		t.Fatalf("Place() error = %v, want restore failure", err)
	}
}

func TestProviderPlaceReportsOperationFailureStage(t *testing.T) {
	t.Parallel()

	operationErr := errors.New("operation failed")
	initial := measurement{bounds: placement.Rectangle{X: 10, Y: 10, Width: 500, Height: 400}, dpi: 144, monitor: 2}
	intermediate := measurement{bounds: placement.Rectangle{X: 0, Y: 0, Width: 500, Height: 400}, dpi: 96, monitor: 1}
	requested := placement.Rectangle{X: 100, Y: 100, Width: 500, Height: 400}

	tests := []struct {
		name        string
		configure   func(*stubWindowSystem)
		wantStage   placement.Stage
		wantActual  bool
		wantPartial bool
	}{
		{
			name:      "begin DPI awareness",
			configure: func(system *stubWindowSystem) { system.beginErr = operationErr },
			wantStage: placement.StageInitialMeasurement,
		},
		{
			name:      "initial measurement",
			configure: func(system *stubWindowSystem) { system.measureErrors = map[int]error{0: operationErr} },
			wantStage: placement.StageInitialMeasurement,
		},
		{
			name:       "target monitor",
			configure:  func(system *stubWindowSystem) { system.targetErr = operationErr },
			wantStage:  placement.StageInitialMeasurement,
			wantActual: true,
		},
		{
			name:        "first move",
			configure:   func(system *stubWindowSystem) { system.moveErrors = map[int]error{0: operationErr} },
			wantStage:   placement.StageMoveToTarget,
			wantActual:  true,
			wantPartial: true,
		},
		{
			name:        "first stability measurement",
			configure:   func(system *stubWindowSystem) { system.measureErrors = map[int]error{1: operationErr} },
			wantStage:   placement.StageWaitForStability,
			wantActual:  true,
			wantPartial: true,
		},
		{
			name:        "final move",
			configure:   func(system *stubWindowSystem) { system.moveErrors = map[int]error{1: operationErr} },
			wantStage:   placement.StageFinalPlacement,
			wantActual:  true,
			wantPartial: true,
		},
		{
			name:        "final measurement",
			configure:   func(system *stubWindowSystem) { system.measureErrors = map[int]error{4: operationErr} },
			wantStage:   placement.StageFinalMeasurement,
			wantActual:  true,
			wantPartial: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			system := &stubWindowSystem{
				target: targetMonitor{handle: 1},
				measurements: []measurement{
					initial,
					intermediate, intermediate, intermediate,
					{bounds: requested, dpi: 96, monitor: 1},
					{bounds: requested, dpi: 96, monitor: 1},
					{bounds: requested, dpi: 96, monitor: 1},
				},
			}
			tt.configure(system)
			provider := testProvider(&stubIdentityValidator{valid: true}, system)

			result, err := provider.Place(context.Background(), placement.Request{Window: testWindow(), InitialTarget: requested}, fixedBoundsResolver(requested))
			if !errors.Is(err, operationErr) {
				t.Fatalf("Place() error = %v, want operation error", err)
			}
			if result.Stage != tt.wantStage || result.ActualKnown != tt.wantActual || result.PartiallyChanged != tt.wantPartial {
				t.Fatalf("Place() result = %+v, want stage=%s actual=%t partial=%t", result, tt.wantStage, tt.wantActual, tt.wantPartial)
			}
		})
	}
}

func TestValidateWindowsRectangle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		bounds  placement.Rectangle
		wantErr bool
	}{
		{name: "minimum origin", bounds: placement.Rectangle{X: minWindowsCoordinate, Y: minWindowsCoordinate, Width: 1, Height: 1}},
		{name: "maximum edges", bounds: placement.Rectangle{X: maxWindowsCoordinate - 1, Y: maxWindowsCoordinate - 1, Width: 1, Height: 1}},
		{name: "x below minimum", bounds: placement.Rectangle{X: minWindowsCoordinate - 1, Width: 1, Height: 1}, wantErr: true},
		{name: "y below minimum", bounds: placement.Rectangle{Y: minWindowsCoordinate - 1, Width: 1, Height: 1}, wantErr: true},
		{name: "x above maximum", bounds: placement.Rectangle{X: maxWindowsCoordinate + 1, Width: 1, Height: 1}, wantErr: true},
		{name: "y above maximum", bounds: placement.Rectangle{Y: maxWindowsCoordinate + 1, Width: 1, Height: 1}, wantErr: true},
		{name: "width above maximum", bounds: placement.Rectangle{Width: maxWindowsCoordinate + 1, Height: 1}, wantErr: true},
		{name: "height above maximum", bounds: placement.Rectangle{Width: 1, Height: maxWindowsCoordinate + 1}, wantErr: true},
		{name: "right edge above maximum", bounds: placement.Rectangle{X: maxWindowsCoordinate, Width: 1, Height: 1}, wantErr: true},
		{name: "bottom edge above maximum", bounds: placement.Rectangle{Y: maxWindowsCoordinate, Width: 1, Height: 1}, wantErr: true},
		{name: "zero width", bounds: placement.Rectangle{Height: 1}, wantErr: true},
		{name: "zero height", bounds: placement.Rectangle{Width: 1}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateWindowsRectangle(tt.bounds)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateWindowsRectangle(%+v) error = %v, wantErr %t", tt.bounds, err, tt.wantErr)
			}
			if tt.wantErr && !errors.Is(err, ErrBoundsOutOfRange) {
				t.Fatalf("validateWindowsRectangle(%+v) error = %v, want ErrBoundsOutOfRange", tt.bounds, err)
			}
		})
	}
}

func testProvider(validator IdentityValidator, system windowSystem) Provider {
	return Provider{
		validator:         validator,
		system:            system,
		wait:              func(context.Context, time.Duration) error { return nil },
		stabilityInterval: time.Millisecond,
		stabilityTimeout:  10 * time.Millisecond,
	}
}

func fixedBoundsResolver(bounds placement.Rectangle) placement.FinalBoundsResolver {
	return func(placement.FinalPlacementContext) (placement.Rectangle, []placement.ValidationError, error) {
		return bounds, nil, nil
	}
}

func testWindow() tracking.Window {
	return tracking.Window{Handle: 10, ProcessID: 20, ProcessStartTime: 30}
}

type stubIdentityValidator struct {
	valid         bool
	validSequence []bool
	calls         int
	afterValidate func(int)
}

func (s *stubIdentityValidator) Validate(tracking.Window) (bool, error) {
	index := s.calls
	s.calls++
	if s.afterValidate != nil {
		s.afterValidate(s.calls)
	}
	if len(s.validSequence) > 0 {
		if index >= len(s.validSequence) {
			return s.validSequence[len(s.validSequence)-1], nil
		}
		return s.validSequence[index], nil
	}
	return s.valid, nil
}

type moveCall struct {
	handle uint64
	bounds placement.Rectangle
	resize bool
}

type stubWindowSystem struct {
	target        targetMonitor
	targetErr     error
	measurements  []measurement
	measureErr    error
	measureErrors map[int]error
	measureCalls  int
	moveErr       error
	moveErrors    map[int]error
	moves         []moveCall
	beginErr      error
	restoreErr    error
	restored      bool
}

func (s *stubWindowSystem) BeginDPIAware() (func() error, error) {
	if s.beginErr != nil {
		return nil, s.beginErr
	}
	return func() error {
		s.restored = true
		return s.restoreErr
	}, nil
}

func (s *stubWindowSystem) TargetMonitor(placement.Rectangle) (targetMonitor, error) {
	return s.target, s.targetErr
}

func (s *stubWindowSystem) Measure(uint64) (measurement, error) {
	call := s.measureCalls
	s.measureCalls++
	if err := s.measureErrors[call]; err != nil {
		return measurement{}, err
	}
	if s.measureErr != nil {
		return measurement{}, s.measureErr
	}
	if len(s.measurements) == 0 {
		return measurement{}, errors.New("no measurement configured")
	}
	measured := s.measurements[0]
	if len(s.measurements) > 1 {
		s.measurements = s.measurements[1:]
	}
	return measured, nil
}

func (s *stubWindowSystem) Move(handle uint64, bounds placement.Rectangle, resize bool) error {
	call := len(s.moves)
	s.moves = append(s.moves, moveCall{handle: handle, bounds: bounds, resize: resize})
	if err := s.moveErrors[call]; err != nil {
		return err
	}
	return s.moveErr
}
