package chromewindowplacement

import (
	"context"
	"errors"
	"math"
	"testing"

	tracking "github.com/asam-masa/browser-launcher/internal/application/chromewindowtracking"
	domain "github.com/asam-masa/browser-launcher/internal/domain/launchcondition"
)

var validRequest = Request{
	Window: tracking.Window{Handle: 1, ProcessID: 10, ProcessStartTime: 100},
	LogicalRequested: domain.New(
		mustDimension(1000),
		mustDimension(700),
		mustCoordinate(100),
		mustCoordinate(100),
	),
	InitialTarget: Rectangle{X: 100, Y: 100, Width: 1000, Height: 700},
}

func TestServicePlaceRejectsInvalidRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*Request)
	}{
		{name: "window handle", mutate: func(r *Request) { r.Window.Handle = 0 }},
		{name: "process ID", mutate: func(r *Request) { r.Window.ProcessID = 0 }},
		{name: "process start time", mutate: func(r *Request) { r.Window.ProcessStartTime = 0 }},
		{name: "logical bounds", mutate: func(r *Request) { r.LogicalRequested = domain.Condition{} }},
		{name: "width", mutate: func(r *Request) { r.InitialTarget.Width = 0 }},
		{name: "coordinate overflow", mutate: func(r *Request) { r.InitialTarget.X = math.MaxInt }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			request := validRequest
			test.mutate(&request)
			placer := &stubPlacer{}
			result, err := NewService(placer).Place(context.Background(), request)
			if !errors.Is(err, ErrInvalidRequest) {
				t.Fatalf("Place() error = %v, want ErrInvalidRequest", err)
			}
			if result.Stage != StageValidation || result.Requested != request.InitialTarget {
				t.Fatalf("Place() result = %#v, want validation result", result)
			}
			if placer.calls != 0 {
				t.Fatalf("placer calls = %d, want 0", placer.calls)
			}
		})
	}
}

func TestFinalBoundsResolverUsesTargetDPIAndWorkArea(t *testing.T) {
	t.Parallel()

	resolver := finalBoundsResolver(validRequest.LogicalRequested)
	bounds, validationErrors, err := resolver(FinalPlacementContext{
		MonitorLeft: -3840,
		MonitorTop:  -2160,
		WorkLeft:    -3840,
		WorkTop:     -2160,
		WorkWidth:   3840,
		WorkHeight:  2080,
		DPI:         144,
	})
	if err != nil {
		t.Fatalf("resolve final bounds: %v", err)
	}
	if len(validationErrors) != 0 {
		t.Fatalf("validation errors = %+v, want none", validationErrors)
	}
	want := Rectangle{X: -3690, Y: -2010, Width: 1500, Height: 1050}
	if bounds != want {
		t.Fatalf("bounds = %+v, want %+v", bounds, want)
	}
}

func TestFinalBoundsResolverRejectsOutsideWorkArea(t *testing.T) {
	t.Parallel()

	resolver := finalBoundsResolver(validRequest.LogicalRequested)
	_, validationErrors, err := resolver(FinalPlacementContext{
		WorkWidth:  1000,
		WorkHeight: 700,
		DPI:        96,
	})
	if !errors.Is(err, ErrFinalValidation) {
		t.Fatalf("resolve final bounds error = %v, want ErrFinalValidation", err)
	}
	if len(validationErrors) != 2 || validationErrors[0].Field != "width" || validationErrors[1].Field != "height" {
		t.Fatalf("validation errors = %+v, want width and height", validationErrors)
	}
}

func TestServicePlacePreservesPartialFailure(t *testing.T) {
	t.Parallel()

	actual := Rectangle{X: 100, Y: 100, Width: 1200, Height: 800}
	placerErr := errors.New("stability timeout")
	placer := &stubPlacer{
		result: Result{
			Stage:            StageWaitForStability,
			Actual:           actual,
			ActualKnown:      true,
			PartiallyChanged: true,
		},
		err: placerErr,
	}

	result, err := NewService(placer).Place(context.Background(), validRequest)
	if !errors.Is(err, ErrPlacementFailed) || !errors.Is(err, placerErr) {
		t.Fatalf("Place() error = %v, want placement and placer errors", err)
	}
	if result.Stage != StageWaitForStability || !result.PartiallyChanged || !result.ActualKnown || result.Actual != actual {
		t.Fatalf("Place() result = %#v, want preserved partial failure", result)
	}
	if result.Requested != validRequest.InitialTarget {
		t.Fatalf("Place() requested = %#v, want %#v", result.Requested, validRequest.InitialTarget)
	}
}

func TestServicePlaceRejectsInvalidPlacerResult(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		placer stubPlacer
	}{
		{name: "failure without stage", placer: stubPlacer{err: errors.New("failed")}},
		{name: "failure with unknown stage", placer: stubPlacer{result: Result{Stage: "unknown"}, err: errors.New("failed")}},
		{name: "move failure without actual bounds", placer: stubPlacer{result: Result{Stage: StageMoveToTarget}, err: errors.New("failed")}},
		{name: "stability failure without actual bounds", placer: stubPlacer{result: Result{Stage: StageWaitForStability}, err: errors.New("failed")}},
		{name: "placement failure without actual bounds", placer: stubPlacer{result: Result{Stage: StageFinalPlacement}, err: errors.New("failed")}},
		{name: "measurement failure without actual bounds", placer: stubPlacer{result: Result{Stage: StageFinalMeasurement}, err: errors.New("failed")}},
		{name: "success without actual bounds", placer: stubPlacer{result: Result{Stage: StageFinalMeasurement}}},
		{
			name: "success before final measurement",
			placer: stubPlacer{result: Result{
				Stage:       StageFinalPlacement,
				Actual:      validRequest.InitialTarget,
				ActualKnown: true,
			}},
		},
		{
			name: "success with invalid actual bounds",
			placer: stubPlacer{result: Result{
				Stage:       StageFinalMeasurement,
				Actual:      Rectangle{X: 100, Y: 100, Width: 0, Height: 700},
				ActualKnown: true,
			}},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := NewService(&test.placer).Place(context.Background(), validRequest)
			if !errors.Is(err, ErrPlacementFailed) || !errors.Is(err, ErrInvalidResult) {
				t.Fatalf("Place() error = %v, want placement and invalid result errors", err)
			}
		})
	}
}

func TestServicePlaceAllowsInitialMeasurementFailureWithoutActualBounds(t *testing.T) {
	t.Parallel()

	placerErr := errors.New("measurement failed")
	placer := &stubPlacer{
		result: Result{Stage: StageInitialMeasurement},
		err:    placerErr,
	}

	result, err := NewService(placer).Place(context.Background(), validRequest)
	if !errors.Is(err, ErrPlacementFailed) || !errors.Is(err, placerErr) {
		t.Fatalf("Place() error = %v, want placement and measurement errors", err)
	}
	if errors.Is(err, ErrInvalidResult) {
		t.Fatalf("Place() error = %v, did not want ErrInvalidResult", err)
	}
	if result.ActualKnown {
		t.Fatalf("Place() result = %#v, want unknown actual bounds", result)
	}
}

func TestServicePlaceReportsBoundsMismatch(t *testing.T) {
	t.Parallel()

	placer := &stubPlacer{result: Result{
		Stage:       StageFinalMeasurement,
		Actual:      Rectangle{X: 100, Y: 100, Width: 999, Height: 700},
		ActualKnown: true,
	}}

	result, err := NewService(placer).Place(context.Background(), validRequest)
	if !errors.Is(err, ErrPlacementFailed) || !errors.Is(err, ErrBoundsMismatch) {
		t.Fatalf("Place() error = %v, want placement and bounds mismatch errors", err)
	}
	if result.Stage != StageFinalMeasurement || result.Matched {
		t.Fatalf("Place() result = %#v, want unmatched final measurement", result)
	}
}

func TestServicePlaceCompletesOnlyForMatchingBounds(t *testing.T) {
	t.Parallel()

	finalBounds := Rectangle{X: 150, Y: 150, Width: 1500, Height: 1050}
	placer := &stubPlacer{result: Result{
		Stage:            StageFinalMeasurement,
		Actual:           finalBounds,
		ActualKnown:      true,
		PartiallyChanged: true,
	}, resolveFinal: true, context: FinalPlacementContext{
		WorkWidth:  3840,
		WorkHeight: 2080,
		DPI:        144,
	}}

	result, err := NewService(placer).Place(context.Background(), validRequest)
	if err != nil {
		t.Fatalf("Place() error = %v", err)
	}
	if result.Stage != StageCompleted || !result.Matched || result.PartiallyChanged {
		t.Fatalf("Place() result = %#v, want completed matching result", result)
	}
	if result.Requested != finalBounds {
		t.Fatalf("Place() requested = %+v, want resolved bounds %+v", result.Requested, finalBounds)
	}
}

func TestServicePlaceReportsFinalValidationErrors(t *testing.T) {
	t.Parallel()

	placer := &stubPlacer{
		result: Result{
			Stage:            StageFinalPlacement,
			Actual:           Rectangle{X: 0, Y: 0, Width: 1000, Height: 700},
			ActualKnown:      true,
			PartiallyChanged: true,
		},
		resolveFinal: true,
		context: FinalPlacementContext{
			WorkWidth:  1000,
			WorkHeight: 700,
			DPI:        96,
		},
	}

	result, err := NewService(placer).Place(context.Background(), validRequest)
	if !errors.Is(err, ErrPlacementFailed) || !errors.Is(err, ErrFinalValidation) {
		t.Fatalf("Place() error = %v, want placement and final validation errors", err)
	}
	if len(result.ValidationErrors) != 2 ||
		result.ValidationErrors[0].Field != "width" || result.ValidationErrors[1].Field != "height" {
		t.Fatalf("Place() validation errors = %+v, want width and height", result.ValidationErrors)
	}
}

type stubPlacer struct {
	result       Result
	err          error
	calls        int
	resolveFinal bool
	context      FinalPlacementContext
}

func (p *stubPlacer) Place(_ context.Context, _ Request, resolver FinalBoundsResolver) (Result, error) {
	p.calls++
	if p.resolveFinal {
		bounds, validationErrors, err := resolver(p.context)
		p.result.ValidationErrors = validationErrors
		if err != nil {
			return p.result, err
		}
		p.result.Requested = bounds
	}
	return p.result, p.err
}

func mustDimension(value int) domain.Dimension {
	dimension, err := domain.NewDimension(value)
	if err != nil {
		panic(err)
	}
	return dimension
}

func mustCoordinate(value int) domain.Coordinate {
	coordinate, err := domain.NewCoordinate(value)
	if err != nil {
		panic(err)
	}
	return coordinate
}
