package chromelaunchoperation

import (
	"context"
	"errors"
	"testing"
)

func TestRegistryStartCreatesStartingOperation(t *testing.T) {
	registry := NewRegistry()

	snapshot, operationContext, err := registry.Start(context.Background())

	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if snapshot.ID != ID("operation-1") {
		t.Errorf("Start() ID = %q, want %q", snapshot.ID, ID("operation-1"))
	}
	if snapshot.State != StateStarting {
		t.Errorf("Start() State = %q, want %q", snapshot.State, StateStarting)
	}
	if err := operationContext.Err(); err != nil {
		t.Errorf("operation context error = %v, want nil", err)
	}
	if current, ok := registry.Current(); !ok || current != snapshot {
		t.Errorf("Current() = (%+v, %t), want (%+v, true)", current, ok, snapshot)
	}
}

func TestRegistryStartRejectsNilContext(t *testing.T) {
	registry := NewRegistry()

	_, _, err := registry.Start(nil)

	if !errors.Is(err, ErrNilContext) {
		t.Fatalf("Start() error = %v, want ErrNilContext", err)
	}
	if _, ok := registry.Current(); ok {
		t.Fatal("Current() found an operation after rejected start")
	}
}

func TestRegistryRejectsConcurrentOperation(t *testing.T) {
	registry := NewRegistry()
	first, _, err := registry.Start(context.Background())
	if err != nil {
		t.Fatalf("first Start() error = %v", err)
	}

	_, _, err = registry.Start(context.Background())

	if !errors.Is(err, ErrOperationInProgress) {
		t.Fatalf("second Start() error = %v, want ErrOperationInProgress", err)
	}
	if current, ok := registry.Current(); !ok || current != first {
		t.Errorf("Current() = (%+v, %t), want (%+v, true)", current, ok, first)
	}
}

func TestRegistryTransitionsOperationToCompletion(t *testing.T) {
	registry := NewRegistry()
	started, operationContext, err := registry.Start(context.Background())
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	running, err := registry.Transition(started.ID, StateRunning)
	if err != nil {
		t.Fatalf("Transition(running) error = %v", err)
	}
	if running.State != StateRunning {
		t.Errorf("Transition(running) State = %q, want %q", running.State, StateRunning)
	}

	completed, err := registry.Transition(started.ID, StateCompleted)
	if err != nil {
		t.Fatalf("Transition(completed) error = %v", err)
	}
	if completed.State != StateCompleted {
		t.Errorf("Transition(completed) State = %q, want %q", completed.State, StateCompleted)
	}
	if !errors.Is(operationContext.Err(), context.Canceled) {
		t.Errorf("operation context error = %v, want context.Canceled", operationContext.Err())
	}
	if stored, ok := registry.Get(started.ID); !ok || stored != completed {
		t.Errorf("Get() = (%+v, %t), want (%+v, true)", stored, ok, completed)
	}
}

func TestRegistryReplacesCompletedOperationOnNextStart(t *testing.T) {
	registry := NewRegistry()
	first, _, err := registry.Start(context.Background())
	if err != nil {
		t.Fatalf("first Start() error = %v", err)
	}
	if _, err := registry.Transition(first.ID, StateFailed); err != nil {
		t.Fatalf("Transition(failed) error = %v", err)
	}

	second, _, err := registry.Start(context.Background())

	if err != nil {
		t.Fatalf("second Start() error = %v", err)
	}
	if second.ID != ID("operation-2") {
		t.Errorf("second Start() ID = %q, want %q", second.ID, ID("operation-2"))
	}
	if _, ok := registry.Get(first.ID); ok {
		t.Error("Get(first ID) found the replaced operation")
	}
	if stored, ok := registry.Get(second.ID); !ok || stored != second {
		t.Errorf("Get(second ID) = (%+v, %t), want (%+v, true)", stored, ok, second)
	}
}

func TestRegistryCancelConvergesToCancelled(t *testing.T) {
	registry := NewRegistry()
	started, operationContext, err := registry.Start(context.Background())
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if _, err := registry.Transition(started.ID, StateRunning); err != nil {
		t.Fatalf("Transition(running) error = %v", err)
	}

	status, cancelling := registry.Cancel(started.ID)

	if status != CancelAccepted {
		t.Errorf("Cancel() status = %q, want %q", status, CancelAccepted)
	}
	if cancelling.State != StateCancelling {
		t.Errorf("Cancel() State = %q, want %q", cancelling.State, StateCancelling)
	}
	if !errors.Is(operationContext.Err(), context.Canceled) {
		t.Errorf("operation context error = %v, want context.Canceled", operationContext.Err())
	}
	if _, err := registry.Transition(started.ID, StateTimedOut); !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("Transition(timed_out) error = %v, want ErrInvalidTransition", err)
	}
	cancelled, err := registry.Transition(started.ID, StateCancelled)
	if err != nil {
		t.Fatalf("Transition(cancelled) error = %v", err)
	}
	if cancelled.State != StateCancelled {
		t.Errorf("Transition(cancelled) State = %q, want %q", cancelled.State, StateCancelled)
	}

	next, _, err := registry.Start(context.Background())
	if err != nil {
		t.Fatalf("Start() after cancellation error = %v", err)
	}
	if next.ID != ID("operation-2") {
		t.Errorf("Start() after cancellation ID = %q, want %q", next.ID, ID("operation-2"))
	}
}

func TestRegistryCancelReportsExistingState(t *testing.T) {
	registry := NewRegistry()
	if status, _ := registry.Cancel(ID("missing")); status != CancelNotFound {
		t.Errorf("Cancel(missing) status = %q, want %q", status, CancelNotFound)
	}

	started, _, err := registry.Start(context.Background())
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if status, _ := registry.Cancel(started.ID); status != CancelAccepted {
		t.Fatalf("first Cancel() status = %q, want %q", status, CancelAccepted)
	}
	if status, _ := registry.Cancel(started.ID); status != CancelAlreadyCancelling {
		t.Errorf("second Cancel() status = %q, want %q", status, CancelAlreadyCancelling)
	}
	if _, err := registry.Transition(started.ID, StateCancelled); err != nil {
		t.Fatalf("Transition(cancelled) error = %v", err)
	}
	if status, _ := registry.Cancel(started.ID); status != CancelAlreadyFinished {
		t.Errorf("Cancel(finished) status = %q, want %q", status, CancelAlreadyFinished)
	}
}

func TestRegistryRejectsInvalidTransitions(t *testing.T) {
	tests := []struct {
		name string
		from State
		to   State
	}{
		{name: "starting to completed", from: StateStarting, to: StateCompleted},
		{name: "running to cancelled", from: StateRunning, to: StateCancelled},
		{name: "completed to running", from: StateCompleted, to: StateRunning},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			registry := NewRegistry()
			started, _, err := registry.Start(context.Background())
			if err != nil {
				t.Fatalf("Start() error = %v", err)
			}
			switch test.from {
			case StateRunning:
				if _, err := registry.Transition(started.ID, StateRunning); err != nil {
					t.Fatalf("prepare running state: %v", err)
				}
			case StateCompleted:
				if _, err := registry.Transition(started.ID, StateRunning); err != nil {
					t.Fatalf("prepare running state: %v", err)
				}
				if _, err := registry.Transition(started.ID, StateCompleted); err != nil {
					t.Fatalf("prepare completed state: %v", err)
				}
			}

			_, err = registry.Transition(started.ID, test.to)

			if !errors.Is(err, ErrInvalidTransition) {
				t.Errorf("Transition(%q to %q) error = %v, want ErrInvalidTransition", test.from, test.to, err)
			}
		})
	}
}

func TestRegistryTransitionRejectsUnknownOperation(t *testing.T) {
	registry := NewRegistry()

	_, err := registry.Transition(ID("missing"), StateRunning)

	if !errors.Is(err, ErrOperationNotFound) {
		t.Fatalf("Transition() error = %v, want ErrOperationNotFound", err)
	}
}

func TestRegistryFinishStoresTerminalStateAndErrorCode(t *testing.T) {
	registry := NewRegistry()
	started, _, err := registry.Start(context.Background())
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if _, err := registry.Transition(started.ID, StateRunning); err != nil {
		t.Fatalf("Transition(running) error = %v", err)
	}

	finished, err := registry.Finish(started.ID, StateFailed, ErrorCodeLaunchFailed)

	if err != nil {
		t.Fatalf("Finish() error = %v", err)
	}
	want := Snapshot{ID: started.ID, State: StateFailed, ErrorCode: ErrorCodeLaunchFailed}
	if finished != want {
		t.Fatalf("Finish() = %+v, want %+v", finished, want)
	}
}

func TestRegistryFinishPrioritizesAcceptedCancellation(t *testing.T) {
	registry := NewRegistry()
	started, _, err := registry.Start(context.Background())
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if _, err := registry.Transition(started.ID, StateRunning); err != nil {
		t.Fatalf("Transition(running) error = %v", err)
	}
	if status, _ := registry.Cancel(started.ID); status != CancelAccepted {
		t.Fatalf("Cancel() status = %q, want %q", status, CancelAccepted)
	}

	finished, err := registry.Finish(started.ID, StateTimedOut, ErrorCodeUnexpected)

	if err != nil {
		t.Fatalf("Finish() error = %v", err)
	}
	want := Snapshot{ID: started.ID, State: StateCancelled}
	if finished != want {
		t.Fatalf("Finish() = %+v, want %+v", finished, want)
	}
}

func TestRegistryFinishRejectsInvalidStateAndUnknownOperation(t *testing.T) {
	registry := NewRegistry()
	started, _, err := registry.Start(context.Background())
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if _, err := registry.Finish(started.ID, StateRunning, ErrorCodeNone); !errors.Is(err, ErrInvalidTerminalState) {
		t.Errorf("Finish(non-terminal) error = %v, want ErrInvalidTerminalState", err)
	}
	if _, err := registry.Finish(ID("missing"), StateFailed, ErrorCodeUnexpected); !errors.Is(err, ErrOperationNotFound) {
		t.Errorf("Finish(missing) error = %v, want ErrOperationNotFound", err)
	}
}

func TestRegistryFinishRejectsInvalidTransition(t *testing.T) {
	registry := NewRegistry()
	started, _, err := registry.Start(context.Background())
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if _, err := registry.Finish(started.ID, StateCompleted, ErrorCodeNone); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("Finish(starting to completed) error = %v, want ErrInvalidTransition", err)
	}
	snapshot, ok := registry.Get(started.ID)
	if !ok || snapshot.State != StateStarting {
		t.Fatalf("Get() = (%+v, %t), want unchanged starting operation", snapshot, ok)
	}
}
