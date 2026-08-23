package main

import (
	"context"
	"testing"

	"browser-launcher/scripts/spike/wails-communication/internal/operation"
)

func TestFinishCancelledEmitsCancellingBeforeCancelled(t *testing.T) {
	app, emitted := newTestApp(t)
	_, cancel := context.WithCancel(context.Background())

	if _, err := app.registry.Create("operation-1", cancel); err != nil {
		t.Fatalf("Create returned an error: %v", err)
	}
	if status, _ := app.registry.Cancel("operation-1"); status != operation.CancelAccepted {
		t.Fatalf("Cancel status = %q, want %q", status, operation.CancelAccepted)
	}

	app.finishCancelled("operation-1")

	assertStates(t, emitted, operation.StateCancelling, operation.StateCancelled)
}

func TestFinishTimedOperationConvergesToCancelled(t *testing.T) {
	app, emitted := newTestApp(t)
	ctx, cancel := context.WithCancel(context.Background())

	if _, err := app.registry.Create("operation-1", cancel); err != nil {
		t.Fatalf("Create returned an error: %v", err)
	}
	if status, _ := app.registry.Cancel("operation-1"); status != operation.CancelAccepted {
		t.Fatalf("Cancel status = %q, want %q", status, operation.CancelAccepted)
	}

	app.finishTimedOperation(ctx, "operation-1", "complete")

	assertStates(t, emitted, operation.StateCancelling, operation.StateCancelled)
	snapshot, found := app.registry.Get("operation-1")
	if !found {
		t.Fatal("Get did not find operation-1")
	}
	if snapshot.State != operation.StateCancelled {
		t.Fatalf("state = %q, want %q", snapshot.State, operation.StateCancelled)
	}
}

func newTestApp(t *testing.T) (*App, *[]OperationStateDTO) {
	t.Helper()

	app := NewApp()
	app.ctx = context.Background()
	emitted := make([]OperationStateDTO, 0, 2)
	app.emitState = func(state OperationStateDTO) {
		emitted = append(emitted, state)
	}
	return app, &emitted
}

func assertStates(
	t *testing.T,
	emitted *[]OperationStateDTO,
	want ...operation.State,
) {
	t.Helper()

	if len(*emitted) != len(want) {
		t.Fatalf("emitted state count = %d, want %d", len(*emitted), len(want))
	}
	for index, state := range want {
		if got := (*emitted)[index].State; got != string(state) {
			t.Fatalf("emitted state %d = %q, want %q", index, got, state)
		}
	}
}
