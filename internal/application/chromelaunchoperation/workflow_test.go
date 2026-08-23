package chromelaunchoperation

import (
	"context"
	"errors"
	"testing"
	"time"

	detection "github.com/asam-masa/browser-launcher/internal/application/chromedetection"
	chromelaunch "github.com/asam-masa/browser-launcher/internal/application/chromelaunch"
	placement "github.com/asam-masa/browser-launcher/internal/application/chromewindowplacement"
	tracking "github.com/asam-masa/browser-launcher/internal/application/chromewindowtracking"
	launchcondition "github.com/asam-masa/browser-launcher/internal/application/launchcondition"
)

var (
	testProfileWindow = tracking.Window{Handle: 1, ProcessID: 10, ProcessStartTime: 100}
	testBrowserWindow = tracking.Window{Handle: 2, ProcessID: 20, ProcessStartTime: 200}
)

func TestWorkflowRunsLaunchStagesInOrder(t *testing.T) {
	t.Parallel()

	fixture := newWorkflowFixture()
	result := fixture.workflow().Run(context.Background(), validWorkflowInput())

	if result.Outcome != OutcomeCompleted || result.ErrorCode != ErrorCodeNone || result.Err != nil {
		t.Fatalf("Run() result = %#v, want completed", result)
	}
	wantCalls := []string{"validate", "detect", "prepare_window_services", "begin_tracking", "launch", "wait", "wait", "wait", "place"}
	assertCalls(t, fixture.calls, wantCalls)
	if fixture.placer.request.Window != testBrowserWindow {
		t.Fatalf("placement window = %+v, want %+v", fixture.placer.request.Window, testBrowserWindow)
	}
	wantBounds := placement.Rectangle{X: 100, Y: 100, Width: 1000, Height: 700}
	if fixture.placer.request.InitialTarget != wantBounds {
		t.Fatalf("initial target = %+v, want %+v", fixture.placer.request.InitialTarget, wantBounds)
	}
}

func TestWorkflowStopsAfterValidationError(t *testing.T) {
	t.Parallel()

	fixture := newWorkflowFixture()
	result := fixture.workflow().Run(context.Background(), launchcondition.Input{})

	if result.Outcome != OutcomeInvalid || result.ErrorCode != ErrorCodeInvalidRequest {
		t.Fatalf("Run() result = %#v, want invalid request", result)
	}
	if len(result.ValidationErrors) != 4 {
		t.Fatalf("validation errors = %+v, want 4 errors", result.ValidationErrors)
	}
	assertCalls(t, fixture.calls, nil)
}

func TestWorkflowClassifiesWorkAreaFailure(t *testing.T) {
	t.Parallel()

	fixture := newWorkflowFixture()
	providerErr := errors.New("display unavailable")
	fixture.workArea.err = providerErr
	result := fixture.workflow().Run(context.Background(), validWorkflowInput())

	if result.Outcome != OutcomeFailed || result.ErrorCode != ErrorCodeWorkArea || !errors.Is(result.Err, providerErr) {
		t.Fatalf("Run() result = %#v, want work area failure", result)
	}
	assertCalls(t, fixture.calls, []string{"validate"})
}

func TestWorkflowStopsAfterTrackingStartFailure(t *testing.T) {
	t.Parallel()

	fixture := newWorkflowFixture()
	observerErr := errors.New("hook failed")
	fixture.observer.err = observerErr
	result := fixture.workflow().Run(context.Background(), validWorkflowInput())

	if result.Outcome != OutcomeFailed || result.ErrorCode != ErrorCodeObservationFailed ||
		!errors.Is(result.Err, observerErr) {
		t.Fatalf("Run() result = %#v, want observation failure", result)
	}
	assertCalls(t, fixture.calls, []string{"validate", "detect", "prepare_window_services", "begin_tracking"})
}

func TestWorkflowClassifiesLaunchFailureAndManagesTracking(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		detectErr error
		launchErr error
		wantCode  ErrorCode
	}{
		{name: "Chrome not found", detectErr: detection.ErrNotFound, wantCode: ErrorCodeChromeNotFound},
		{name: "unsupported platform", detectErr: detection.ErrUnsupportedPlatform, wantCode: ErrorCodeUnsupported},
		{name: "process start", launchErr: errors.New("start failed"), wantCode: ErrorCodeLaunchFailed},
		{name: "unexpected detection", detectErr: errors.New("registry failed"), wantCode: ErrorCodeUnexpected},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			fixture := newWorkflowFixture()
			fixture.detector.err = test.detectErr
			fixture.process.err = test.launchErr

			result := fixture.workflow().Run(context.Background(), validWorkflowInput())

			if result.Outcome != OutcomeFailed || result.ErrorCode != test.wantCode || result.Err == nil {
				t.Fatalf("Run() result = %#v, want launch failure %q", result, test.wantCode)
			}
			if test.detectErr != nil && fixture.observation.closed {
				t.Fatal("tracking observation was closed even though tracking did not start")
			}
			if test.detectErr == nil && !fixture.observation.closed {
				t.Fatal("tracking observation was not closed after launch failure")
			}
			for _, call := range *fixture.calls {
				if call == "wait" || call == "place" {
					t.Fatalf("calls = %v, did not want wait or place", fixture.calls)
				}
			}
		})
	}
}

func TestWorkflowClassifiesTrackingOutcomes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		events      []tracking.Event
		wantOutcome Outcome
		wantCode    ErrorCode
	}{
		{name: "cancelled", events: []tracking.Event{{Type: tracking.EventCancelled}}, wantOutcome: OutcomeCancelled},
		{name: "timed out", events: []tracking.Event{{Type: tracking.EventTimedOut}}, wantOutcome: OutcomeTimedOut},
		{
			name: "ambiguous windows",
			events: []tracking.Event{
				{Type: tracking.EventSnapshot, Snapshot: tracking.Snapshot{testProfileWindow, testBrowserWindow}},
				{Type: tracking.EventTimedOut},
			},
			wantOutcome: OutcomeFailed,
			wantCode:    ErrorCodeAmbiguousWindows,
		},
		{
			name: "candidate lost",
			events: []tracking.Event{
				{Type: tracking.EventSnapshot, Snapshot: tracking.Snapshot{testProfileWindow}},
				{Type: tracking.EventSnapshot, Snapshot: tracking.Snapshot{}},
				{Type: tracking.EventTimedOut},
			},
			wantOutcome: OutcomeFailed,
			wantCode:    ErrorCodeCandidateLost,
		},
		{
			name:        "observation failed",
			events:      []tracking.Event{{Type: tracking.EventFailed, Err: errors.New("snapshot failed")}},
			wantOutcome: OutcomeFailed,
			wantCode:    ErrorCodeObservationFailed,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			fixture := newWorkflowFixture()
			fixture.observation.events = test.events

			result := fixture.workflow().Run(context.Background(), validWorkflowInput())

			if result.Outcome != test.wantOutcome || result.ErrorCode != test.wantCode {
				t.Fatalf("Run() result = %#v, want outcome %q and code %q", result, test.wantOutcome, test.wantCode)
			}
			for _, call := range *fixture.calls {
				if call == "place" {
					t.Fatalf("calls = %v, did not want placement", fixture.calls)
				}
			}
		})
	}
}

func TestWorkflowPreservesPlacementFailureResult(t *testing.T) {
	t.Parallel()

	fixture := newWorkflowFixture()
	placementErr := errors.New("stability failed")
	fixture.placer.result = placement.Result{
		Stage:            placement.StageWaitForStability,
		Actual:           placement.Rectangle{X: 100, Y: 100, Width: 900, Height: 600},
		ActualKnown:      true,
		PartiallyChanged: true,
	}
	fixture.placer.err = placementErr

	result := fixture.workflow().Run(context.Background(), validWorkflowInput())

	if result.Outcome != OutcomeFailed || result.ErrorCode != ErrorCodePlacementFailed ||
		!errors.Is(result.Err, placementErr) {
		t.Fatalf("Run() result = %#v, want placement failure", result)
	}
	if !result.Placement.PartiallyChanged || !result.Placement.ActualKnown ||
		result.Placement.Stage != placement.StageWaitForStability {
		t.Fatalf("placement result = %#v, want preserved partial result", result.Placement)
	}
}

func TestWorkflowClassifiesContextEndDuringPlacement(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		context     func() (context.Context, context.CancelFunc)
		endContext  func(context.Context, context.CancelFunc)
		wantOutcome Outcome
	}{
		{
			name: "cancelled",
			context: func() (context.Context, context.CancelFunc) {
				return context.WithCancel(context.Background())
			},
			endContext:  func(_ context.Context, cancel context.CancelFunc) { cancel() },
			wantOutcome: OutcomeCancelled,
		},
		{
			name: "timed out",
			context: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 50*time.Millisecond)
			},
			endContext: func(ctx context.Context, _ context.CancelFunc) {
				<-ctx.Done()
			},
			wantOutcome: OutcomeTimedOut,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := test.context()
			defer cancel()
			fixture := newWorkflowFixture()
			fixture.placer.beforeResult = func() { test.endContext(ctx, cancel) }
			result := fixture.workflow().Run(ctx, validWorkflowInput())

			if result.Outcome != test.wantOutcome {
				t.Fatalf("Run() outcome = %q, want %q", result.Outcome, test.wantOutcome)
			}
			if !result.Placement.PartiallyChanged {
				t.Fatalf("placement result = %#v, want partial result", result.Placement)
			}
		})
	}
}

func TestWorkflowStopsBeforeSideEffectsWhenContextEnded(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		context     func() context.Context
		wantOutcome Outcome
	}{
		{
			name: "cancelled",
			context: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			wantOutcome: OutcomeCancelled,
		},
		{
			name: "timed out",
			context: func() context.Context {
				ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
				cancel()
				return ctx
			},
			wantOutcome: OutcomeTimedOut,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			fixture := newWorkflowFixture()
			result := fixture.workflow().Run(test.context(), validWorkflowInput())
			if result.Outcome != test.wantOutcome {
				t.Fatalf("Run() outcome = %q, want %q", result.Outcome, test.wantOutcome)
			}
			assertCalls(t, fixture.calls, nil)
		})
	}
}

func TestWorkflowDoesNotPlaceWhenContextEndsAfterTracking(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	fixture := newWorkflowFixture()
	fixture.observation.afterEvent = func(index int) {
		if index == len(fixture.observation.events) {
			cancel()
		}
	}

	result := fixture.workflow().Run(ctx, validWorkflowInput())

	if result.Outcome != OutcomeCancelled {
		t.Fatalf("Run() outcome = %q, want %q", result.Outcome, OutcomeCancelled)
	}
	for _, call := range *fixture.calls {
		if call == "place" {
			t.Fatalf("calls = %v, did not want placement", *fixture.calls)
		}
	}
}

func TestWorkflowRejectsNilContext(t *testing.T) {
	t.Parallel()

	fixture := newWorkflowFixture()
	result := fixture.workflow().Run(nil, validWorkflowInput())
	if result.Outcome != OutcomeFailed || result.ErrorCode != ErrorCodeInvalidRequest ||
		!errors.Is(result.Err, ErrNilContext) {
		t.Fatalf("Run() result = %#v, want invalid context", result)
	}
	assertCalls(t, fixture.calls, nil)
}

type workflowFixture struct {
	calls       *[]string
	workArea    *workflowWorkAreaProvider
	detector    *workflowDetector
	process     *workflowProcessLauncher
	observer    *workflowObserver
	observation *workflowObservation
	placer      *workflowPlacer
}

func newWorkflowFixture() workflowFixture {
	calls := &[]string{}
	observation := &workflowObservation{
		calls: calls,
		events: []tracking.Event{
			{Type: tracking.EventSnapshot, Snapshot: tracking.Snapshot{testProfileWindow}},
			{Type: tracking.EventSnapshot, Snapshot: tracking.Snapshot{}},
			{Type: tracking.EventSnapshot, Snapshot: tracking.Snapshot{testBrowserWindow}},
		},
	}
	return workflowFixture{
		calls: calls,
		workArea: &workflowWorkAreaProvider{
			calls: calls,
			workArea: launchcondition.PrimaryWorkArea{
				WorkWidth: 1920, WorkHeight: 1080, DPI: 96,
			},
		},
		detector: &workflowDetector{
			calls:        calls,
			installation: detection.Installation{ExecutablePath: `C:\Program Files\Google\Chrome\Application\chrome.exe`},
		},
		process:     &workflowProcessLauncher{calls: calls},
		observer:    &workflowObserver{calls: calls, observation: observation},
		observation: observation,
		placer:      &workflowPlacer{calls: calls},
	}
}

func (f workflowFixture) workflow() Workflow {
	return NewWorkflow(
		launchcondition.NewService(f.workArea),
		chromelaunch.NewService(f.detector, f.process),
		func(executablePath string) WindowServices {
			*f.calls = append(*f.calls, "prepare_window_services")
			if executablePath != f.detector.installation.ExecutablePath {
				panic("window services received an unexpected executable path")
			}
			return WindowServices{
				Tracker: tracking.NewService(f.observer),
				Placer:  placement.NewService(f.placer),
			}
		},
	)
}

func validWorkflowInput() launchcondition.Input {
	return launchcondition.Input{Width: "1000", Height: "700", X: "100", Y: "100"}
}

func assertCalls(t *testing.T, calls *[]string, want []string) {
	t.Helper()
	if len(*calls) != len(want) {
		t.Fatalf("calls = %v, want %v", *calls, want)
	}
	for index := range want {
		if (*calls)[index] != want[index] {
			t.Fatalf("calls = %v, want %v", *calls, want)
		}
	}
}

type workflowWorkAreaProvider struct {
	calls    *[]string
	workArea launchcondition.PrimaryWorkArea
	err      error
}

func (p *workflowWorkAreaProvider) GetPrimaryWorkArea() (launchcondition.PrimaryWorkArea, error) {
	*p.calls = append(*p.calls, "validate")
	return p.workArea, p.err
}

type workflowDetector struct {
	calls        *[]string
	installation detection.Installation
	err          error
}

func (d *workflowDetector) Detect() (detection.Installation, error) {
	*d.calls = append(*d.calls, "detect")
	return d.installation, d.err
}

type workflowProcessLauncher struct {
	calls *[]string
	err   error
}

func (l *workflowProcessLauncher) Start(chromelaunch.Command) (chromelaunch.Result, error) {
	*l.calls = append(*l.calls, "launch")
	return chromelaunch.Result{ProcessID: 1234}, l.err
}

type workflowObserver struct {
	calls       *[]string
	observation tracking.Observation
	err         error
}

func (o *workflowObserver) Start() (tracking.Observation, error) {
	*o.calls = append(*o.calls, "begin_tracking")
	return o.observation, o.err
}

type workflowObservation struct {
	calls      *[]string
	events     []tracking.Event
	index      int
	closed     bool
	afterEvent func(int)
}

func (o *workflowObservation) Baseline() tracking.Snapshot { return nil }

func (o *workflowObservation) Next(context.Context) tracking.Event {
	*o.calls = append(*o.calls, "wait")
	event := o.events[o.index]
	o.index++
	if o.afterEvent != nil {
		o.afterEvent(o.index)
	}
	return event
}

func (o *workflowObservation) Close() error {
	o.closed = true
	return nil
}

type workflowPlacer struct {
	calls        *[]string
	request      placement.Request
	result       placement.Result
	err          error
	beforeResult func()
}

func (p *workflowPlacer) Place(ctx context.Context, request placement.Request, _ placement.FinalBoundsResolver) (placement.Result, error) {
	*p.calls = append(*p.calls, "place")
	p.request = request
	if p.beforeResult != nil {
		p.result.PartiallyChanged = true
		p.beforeResult()
	}
	if err := ctx.Err(); err != nil {
		return p.result, err
	}
	if p.err != nil {
		return p.result, p.err
	}
	return placement.Result{
		Stage:       placement.StageFinalMeasurement,
		Requested:   request.InitialTarget,
		Actual:      request.InitialTarget,
		ActualKnown: true,
	}, nil
}
