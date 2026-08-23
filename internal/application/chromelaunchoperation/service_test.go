package chromelaunchoperation

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	launchcondition "github.com/asam-masa/browser-launcher/internal/application/launchcondition"
)

func TestServiceStartRunsWorkflowToCompletion(t *testing.T) {
	t.Parallel()

	runner := &stubWorkflowRunner{result: WorkflowResult{Outcome: OutcomeCompleted}}
	service := testOperationService(runner)
	started, err := service.Start(context.Background(), launchcondition.Input{Width: "1000"})

	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if started.State != StateStarting || started.ID == "" {
		t.Fatalf("Start() = %+v, want starting operation", started)
	}
	finished := waitForTerminal(t, service, started.ID)
	if finished.State != StateCompleted || finished.ErrorCode != ErrorCodeNone {
		t.Fatalf("terminal snapshot = %+v, want completed", finished)
	}
	if runner.input.Width != "1000" {
		t.Fatalf("workflow input = %+v, want forwarded input", runner.input)
	}
}

func TestServiceMapsWorkflowFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		result    WorkflowResult
		wantState State
		wantCode  ErrorCode
	}{
		{name: "timed out", result: WorkflowResult{Outcome: OutcomeTimedOut}, wantState: StateTimedOut},
		{name: "failed", result: WorkflowResult{Outcome: OutcomeFailed, ErrorCode: ErrorCodeLaunchFailed}, wantState: StateFailed, wantCode: ErrorCodeLaunchFailed},
		{name: "invalid", result: WorkflowResult{Outcome: OutcomeInvalid, ErrorCode: ErrorCodeInvalidRequest}, wantState: StateFailed, wantCode: ErrorCodeInvalidRequest},
		{name: "missing error code", result: WorkflowResult{Outcome: OutcomeFailed}, wantState: StateFailed, wantCode: ErrorCodeUnexpected},
		{name: "unknown outcome", result: WorkflowResult{Outcome: "unknown"}, wantState: StateFailed, wantCode: ErrorCodeUnexpected},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			service := testOperationService(&stubWorkflowRunner{result: test.result})
			started, err := service.Start(context.Background(), launchcondition.Input{})
			if err != nil {
				t.Fatalf("Start() error = %v", err)
			}
			finished := waitForTerminal(t, service, started.ID)
			if finished.State != test.wantState || finished.ErrorCode != test.wantCode {
				t.Fatalf("terminal snapshot = %+v, want state %q and code %q", finished, test.wantState, test.wantCode)
			}
		})
	}
}

func TestServiceCancelStopsWorkflowAndWinsCompletionRace(t *testing.T) {
	t.Parallel()

	runner := &blockingWorkflowRunner{
		started: make(chan struct{}),
		release: make(chan struct{}),
		result:  WorkflowResult{Outcome: OutcomeTimedOut, ErrorCode: ErrorCodeUnexpected},
	}
	service := testOperationService(runner)
	started, err := service.Start(context.Background(), launchcondition.Input{})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	<-runner.started

	status, cancelling := service.Cancel(started.ID)
	close(runner.release)

	if status != CancelAccepted || cancelling.State != StateCancelling {
		t.Fatalf("Cancel() = (%q, %+v), want accepted and cancelling", status, cancelling)
	}
	finished := waitForTerminal(t, service, started.ID)
	if finished.State != StateCancelled || finished.ErrorCode != ErrorCodeNone {
		t.Fatalf("terminal snapshot = %+v, want cancelled", finished)
	}
}

func TestServiceTimeoutCancelsWorkflow(t *testing.T) {
	t.Parallel()

	runner := &contextWorkflowRunner{started: make(chan struct{})}
	service := testOperationService(runner)
	service.timeout = 10 * time.Millisecond
	started, err := service.Start(context.Background(), launchcondition.Input{})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	<-runner.started

	finished := waitForTerminal(t, service, started.ID)

	if finished.State != StateTimedOut {
		t.Fatalf("terminal snapshot = %+v, want timed out", finished)
	}
}

func TestServiceTimeoutFinishesNonCooperativeWorkflowAndAllowsRestart(t *testing.T) {
	t.Parallel()

	runner := &nonCooperativeWorkflowRunner{
		started: make(chan struct{}, 1),
		release: make(chan struct{}),
	}
	service := testOperationService(runner)
	service.timeout = 10 * time.Millisecond
	first, err := service.Start(context.Background(), launchcondition.Input{})
	if err != nil {
		t.Fatalf("first Start() error = %v", err)
	}
	<-runner.started

	finished := waitForTerminal(t, service, first.ID)
	if finished.State != StateTimedOut {
		t.Fatalf("terminal snapshot = %+v, want timed out", finished)
	}

	second, err := service.Start(context.Background(), launchcondition.Input{})
	if err != nil {
		t.Fatalf("second Start() error = %v", err)
	}
	if second.ID == first.ID {
		t.Fatalf("second ID = %q, want a new ID", second.ID)
	}

	close(runner.release)
}

func TestServiceRejectsConcurrentStart(t *testing.T) {
	t.Parallel()

	runner := &blockingWorkflowRunner{
		started: make(chan struct{}),
		release: make(chan struct{}),
		result:  WorkflowResult{Outcome: OutcomeCompleted},
	}
	service := testOperationService(runner)
	first, err := service.Start(context.Background(), launchcondition.Input{})
	if err != nil {
		t.Fatalf("first Start() error = %v", err)
	}
	<-runner.started
	if _, err := service.Start(context.Background(), launchcondition.Input{}); !errors.Is(err, ErrOperationInProgress) {
		t.Fatalf("concurrent Start() error = %v, want ErrOperationInProgress", err)
	}
	close(runner.release)
	_ = waitForTerminal(t, service, first.ID)
}

func TestServiceAllowsRestartAfterFinish(t *testing.T) {
	t.Parallel()

	service := testOperationService(&stubWorkflowRunner{result: WorkflowResult{Outcome: OutcomeCompleted}})
	first, err := service.Start(context.Background(), launchcondition.Input{})
	if err != nil {
		t.Fatalf("first Start() error = %v", err)
	}
	_ = waitForTerminal(t, service, first.ID)

	second, err := service.Start(context.Background(), launchcondition.Input{})
	if err != nil {
		t.Fatalf("second Start() error = %v", err)
	}
	if second.ID == first.ID {
		t.Fatalf("second ID = %q, want a new ID", second.ID)
	}
}

func TestServiceRecoversWorkflowPanic(t *testing.T) {
	t.Parallel()

	service := testOperationService(panicWorkflowRunner{})
	started, err := service.Start(context.Background(), launchcondition.Input{})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	finished := waitForTerminal(t, service, started.ID)
	if finished.State != StateFailed || finished.ErrorCode != ErrorCodeUnexpected {
		t.Fatalf("terminal snapshot = %+v, want unexpected failure", finished)
	}
}

func TestServiceNotifiesStateChanges(t *testing.T) {
	t.Parallel()

	notifier := &recordingStateNotifier{states: make(chan Snapshot, 3)}
	service := NewService(
		&stubWorkflowRunner{result: WorkflowResult{Outcome: OutcomeCompleted}},
		NewRegistry(),
		notifier,
	)
	started, err := service.Start(context.Background(), launchcondition.Input{})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	_ = waitForTerminal(t, service, started.ID)

	want := []State{StateStarting, StateRunning, StateCompleted}
	for _, wantState := range want {
		select {
		case snapshot := <-notifier.states:
			if snapshot.ID != started.ID || snapshot.State != wantState {
				t.Fatalf("notification = %+v, want ID %q and state %q", snapshot, started.ID, wantState)
			}
		case <-time.After(time.Second):
			t.Fatalf("notification for state %q was not received", wantState)
		}
	}
}

func TestServiceNotifiesAcceptedCancellation(t *testing.T) {
	t.Parallel()

	runner := &blockingWorkflowRunner{
		started: make(chan struct{}),
		release: make(chan struct{}),
		result:  WorkflowResult{Outcome: OutcomeCompleted},
	}
	notifier := &recordingStateNotifier{states: make(chan Snapshot, 4)}
	service := NewService(runner, NewRegistry(), notifier)
	started, err := service.Start(context.Background(), launchcondition.Input{})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	<-runner.started
	status, _ := service.Cancel(started.ID)
	close(runner.release)
	if status != CancelAccepted {
		t.Fatalf("Cancel() status = %q, want %q", status, CancelAccepted)
	}
	_ = waitForTerminal(t, service, started.ID)

	wantPrefix := []State{StateStarting, StateRunning}
	for _, wantState := range wantPrefix {
		select {
		case snapshot := <-notifier.states:
			if snapshot.State != wantState {
				t.Fatalf("notification state = %q, want %q", snapshot.State, wantState)
			}
		case <-time.After(time.Second):
			t.Fatalf("notification for state %q was not received", wantState)
		}
	}
	select {
	case snapshot := <-notifier.states:
		if snapshot.State == StateCancelling {
			select {
			case snapshot = <-notifier.states:
			case <-time.After(time.Second):
				t.Fatal("cancelled notification was not received after cancelling")
			}
		}
		if snapshot.State != StateCancelled {
			t.Fatalf("final notification state = %q, want cancelled", snapshot.State)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled notification was not received")
	}
}

func TestServiceRejectsInvalidConfiguration(t *testing.T) {
	t.Parallel()

	if _, err := (Service{}).Start(context.Background(), launchcondition.Input{}); !errors.Is(err, ErrServiceNotConfigured) {
		t.Fatalf("Start() error = %v, want ErrServiceNotConfigured", err)
	}
	service := testOperationService(&stubWorkflowRunner{})
	if _, err := service.Start(nil, launchcondition.Input{}); !errors.Is(err, ErrNilContext) {
		t.Fatalf("Start(nil) error = %v, want ErrNilContext", err)
	}
}

func TestServiceShutdownWaitsForWorkflowCleanup(t *testing.T) {
	t.Parallel()

	runner := &contextWorkflowRunner{started: make(chan struct{})}
	service := testOperationService(runner)
	started, err := service.Start(context.Background(), launchcondition.Input{})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	<-runner.started
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := service.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	finished := waitForTerminal(t, service, started.ID)
	if finished.State != StateCancelled {
		t.Fatalf("terminal snapshot = %+v, want cancelled", finished)
	}
}

func TestServiceShutdownStopsAtDeadlineForNonCooperativeWorkflow(t *testing.T) {
	t.Parallel()

	runner := &nonCooperativeWorkflowRunner{
		started: make(chan struct{}, 1),
		release: make(chan struct{}),
	}
	service := testOperationService(runner)
	_, err := service.Start(context.Background(), launchcondition.Input{})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	<-runner.started

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := service.Shutdown(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Shutdown() error = %v, want context deadline exceeded", err)
	}
	close(runner.release)
}

func TestServiceShutdownWaitsForStartRegisteredBeforePublication(t *testing.T) {
	t.Parallel()

	notifier := &blockingStateNotifier{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	service := NewService(
		&stubWorkflowRunner{result: WorkflowResult{Outcome: OutcomeCompleted}},
		NewRegistry(),
		notifier,
	)
	startDone := make(chan error, 1)
	go func() {
		_, err := service.Start(context.Background(), launchcondition.Input{})
		startDone <- err
	}()
	<-notifier.started

	shutdownDone := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		shutdownDone <- service.Shutdown(ctx)
	}()
	select {
	case err := <-shutdownDone:
		t.Fatalf("Shutdown() returned before registered Start continued: %v", err)
	case <-time.After(10 * time.Millisecond):
	}

	close(notifier.release)
	if err := <-startDone; err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := <-shutdownDone; err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
}

func TestServiceRejectsStartAfterShutdownBegins(t *testing.T) {
	t.Parallel()

	service := testOperationService(&stubWorkflowRunner{result: WorkflowResult{Outcome: OutcomeCompleted}})
	if err := service.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	if _, err := service.Start(context.Background(), launchcondition.Input{}); !errors.Is(err, ErrServiceShuttingDown) {
		t.Fatalf("Start() error = %v, want ErrServiceShuttingDown", err)
	}
}

func testOperationService(runner WorkflowRunner) Service {
	service := NewService(runner, NewRegistry())
	service.timeout = time.Second
	return service
}

func waitForTerminal(t *testing.T, service Service, id ID) Snapshot {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		snapshot, ok := service.Get(id)
		if ok && isTerminal(snapshot.State) {
			return snapshot
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("operation %q did not reach a terminal state", id)
	return Snapshot{}
}

type stubWorkflowRunner struct {
	result WorkflowResult
	input  launchcondition.Input
}

func (r *stubWorkflowRunner) Run(_ context.Context, input launchcondition.Input) WorkflowResult {
	r.input = input
	return r.result
}

type blockingWorkflowRunner struct {
	started chan struct{}
	release chan struct{}
	result  WorkflowResult
}

func (r *blockingWorkflowRunner) Run(context.Context, launchcondition.Input) WorkflowResult {
	close(r.started)
	<-r.release
	return r.result
}

type contextWorkflowRunner struct {
	started chan struct{}
}

type nonCooperativeWorkflowRunner struct {
	started chan struct{}
	release chan struct{}
}

func (r *nonCooperativeWorkflowRunner) Run(context.Context, launchcondition.Input) WorkflowResult {
	select {
	case r.started <- struct{}{}:
	default:
	}
	<-r.release
	return WorkflowResult{Outcome: OutcomeCompleted}
}

func (r *contextWorkflowRunner) Run(ctx context.Context, _ launchcondition.Input) WorkflowResult {
	close(r.started)
	<-ctx.Done()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return WorkflowResult{Outcome: OutcomeTimedOut}
	}
	return WorkflowResult{Outcome: OutcomeCancelled}
}

type panicWorkflowRunner struct{}

func (panicWorkflowRunner) Run(context.Context, launchcondition.Input) WorkflowResult {
	panic("unexpected workflow panic")
}

type recordingStateNotifier struct {
	states chan Snapshot
}

func (n *recordingStateNotifier) Notify(snapshot Snapshot) {
	n.states <- snapshot
}

type blockingStateNotifier struct {
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (n *blockingStateNotifier) Notify(Snapshot) {
	n.once.Do(func() {
		close(n.started)
		<-n.release
	})
}
