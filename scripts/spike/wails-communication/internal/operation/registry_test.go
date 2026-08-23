package operation

import (
	"context"
	"errors"
	"testing"
)

func TestRegistryCreateAndGet(t *testing.T) {
	registry := NewRegistry()
	_, cancel := context.WithCancel(context.Background())

	created, err := registry.Create("operation-1", cancel)
	if err != nil {
		t.Fatalf("Create returned an error: %v", err)
	}
	if created.State != StateStarting {
		t.Fatalf("Create state = %q, want %q", created.State, StateStarting)
	}

	got, found := registry.Get("operation-1")
	if !found {
		t.Fatal("Get did not find the created operation")
	}
	if got != created {
		t.Fatalf("Get = %#v, want %#v", got, created)
	}
}

func TestRegistryRejectsDuplicateID(t *testing.T) {
	registry := NewRegistry()
	_, cancel := context.WithCancel(context.Background())

	if _, err := registry.Create("operation-1", cancel); err != nil {
		t.Fatalf("first Create returned an error: %v", err)
	}
	if _, err := registry.Create("operation-1", cancel); !errors.Is(err, ErrOperationExists) {
		t.Fatalf("second Create error = %v, want %v", err, ErrOperationExists)
	}
}

func TestRegistryCancelDistinguishesOperationStates(t *testing.T) {
	registry := NewRegistry()
	ctx, cancel := context.WithCancel(context.Background())

	if _, err := registry.Create("operation-1", cancel); err != nil {
		t.Fatalf("Create returned an error: %v", err)
	}

	status, snapshot := registry.Cancel("operation-1")
	if status != CancelAccepted {
		t.Fatalf("first Cancel status = %q, want %q", status, CancelAccepted)
	}
	if snapshot.State != StateCancelling {
		t.Fatalf("first Cancel state = %q, want %q", snapshot.State, StateCancelling)
	}
	if ctx.Err() != context.Canceled {
		t.Fatalf("context error = %v, want %v", ctx.Err(), context.Canceled)
	}

	status, _ = registry.Cancel("operation-1")
	if status != CancelAlreadyCancelling {
		t.Fatalf("second Cancel status = %q, want %q", status, CancelAlreadyCancelling)
	}

	if _, transitioned := registry.Transition(
		"operation-1",
		StateCancelled,
		"",
		"operation cancelled",
	); !transitioned {
		t.Fatal("Transition to cancelled returned false")
	}

	status, _ = registry.Cancel("operation-1")
	if status != CancelAlreadyFinished {
		t.Fatalf("finished Cancel status = %q, want %q", status, CancelAlreadyFinished)
	}

	status, _ = registry.Cancel("missing")
	if status != CancelNotFound {
		t.Fatalf("missing Cancel status = %q, want %q", status, CancelNotFound)
	}
}

func TestRegistryRejectsTransitionAfterTerminalState(t *testing.T) {
	registry := NewRegistry()
	_, cancel := context.WithCancel(context.Background())

	if _, err := registry.Create("operation-1", cancel); err != nil {
		t.Fatalf("Create returned an error: %v", err)
	}
	if _, transitioned := registry.Transition(
		"operation-1",
		StateCompleted,
		"",
		"operation completed",
	); !transitioned {
		t.Fatal("Transition to completed returned false")
	}
	if _, transitioned := registry.Transition(
		"operation-1",
		StateFailed,
		"unexpected_failure",
		"operation failed",
	); transitioned {
		t.Fatal("Transition after terminal state returned true")
	}
}
