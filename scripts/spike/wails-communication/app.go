package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"browser-launcher/scripts/spike/wails-communication/internal/operation"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const operationStateEvent = "operation:state-changed"

type OperationRequestDTO struct {
	Mode                 string `json:"mode"`
	DurationMilliseconds int    `json:"durationMilliseconds"`
}

type StartOperationResultDTO struct {
	OperationID string `json:"operationId"`
	State       string `json:"state"`
}

type OperationStateDTO struct {
	OperationID string `json:"operationId"`
	State       string `json:"state"`
	ErrorCode   string `json:"errorCode,omitempty"`
	Message     string `json:"message,omitempty"`
}

type GetOperationStateResultDTO struct {
	Found bool               `json:"found"`
	State *OperationStateDTO `json:"state,omitempty"`
}

type CancelOperationResultDTO struct {
	Status string             `json:"status"`
	State  *OperationStateDTO `json:"state,omitempty"`
}

type App struct {
	ctx       context.Context
	registry  *operation.Registry
	sequence  atomic.Uint64
	wg        sync.WaitGroup
	emitState func(OperationStateDTO)
}

func NewApp() *App {
	return &App{
		registry: operation.NewRegistry(),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) shutdown(_ context.Context) {
	a.registry.CancelAll()
	a.wg.Wait()
}

func (a *App) StartOperation(
	request OperationRequestDTO,
) (StartOperationResultDTO, error) {
	if a.ctx == nil {
		return StartOperationResultDTO{}, errors.New("application is not ready")
	}
	if err := validateRequest(request); err != nil {
		return StartOperationResultDTO{}, err
	}

	id := "operation-" + strconv.FormatUint(a.sequence.Add(1), 10)
	ctx, cancel := context.WithCancel(a.ctx)
	snapshot, err := a.registry.Create(id, cancel)
	if err != nil {
		cancel()
		return StartOperationResultDTO{}, fmt.Errorf("register operation: %w", err)
	}

	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		a.runOperation(ctx, id, request)
	}()

	return StartOperationResultDTO{
		OperationID: snapshot.ID,
		State:       string(snapshot.State),
	}, nil
}

func (a *App) CancelOperation(operationID string) CancelOperationResultDTO {
	status, snapshot := a.registry.Cancel(operationID)
	result := CancelOperationResultDTO{
		Status: string(status),
	}
	if snapshot.ID != "" {
		state := toStateDTO(snapshot)
		result.State = &state
	}
	return result
}

func (a *App) GetOperationState(
	operationID string,
) GetOperationStateResultDTO {
	snapshot, found := a.registry.Get(operationID)
	if !found {
		return GetOperationStateResultDTO{Found: false}
	}
	state := toStateDTO(snapshot)
	return GetOperationStateResultDTO{
		Found: true,
		State: &state,
	}
}

func (a *App) runOperation(
	ctx context.Context,
	id string,
	request OperationRequestDTO,
) {
	a.transition(id, operation.StateStarting, "", "operation started")
	a.transition(id, operation.StateRunning, "", "operation is running")

	timer := time.NewTimer(time.Duration(request.DurationMilliseconds) * time.Millisecond)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		a.finishCancelled(id)
	case <-timer.C:
		a.finishTimedOperation(ctx, id, request.Mode)
	}
}

func (a *App) finishTimedOperation(
	ctx context.Context,
	id string,
	mode string,
) {
	var transitioned bool
	switch mode {
	case "complete":
		transitioned = a.transition(
			id,
			operation.StateCompleted,
			"",
			"operation completed",
		)
	case "timeout":
		transitioned = a.transition(
			id,
			operation.StateTimedOut,
			"operation_timeout",
			"operation timed out",
		)
	case "fail":
		transitioned = a.transition(
			id,
			operation.StateFailed,
			"unexpected_failure",
			"operation failed without exposing internal details",
		)
	}
	if !transitioned && ctx.Err() != nil {
		a.finishCancelled(id)
	}
}

func (a *App) finishCancelled(id string) {
	snapshot, found := a.registry.Get(id)
	if found && snapshot.State == operation.StateCancelling {
		a.emit(toStateDTO(snapshot))
	}
	a.transition(id, operation.StateCancelled, "", "operation cancelled")
}

func (a *App) transition(
	id string,
	state operation.State,
	errorCode string,
	message string,
) bool {
	snapshot, transitioned := a.registry.Transition(id, state, errorCode, message)
	if !transitioned {
		return false
	}
	a.emit(toStateDTO(snapshot))
	return true
}

func (a *App) emit(state OperationStateDTO) {
	if a.emitState != nil {
		a.emitState(state)
		return
	}
	runtime.EventsEmit(a.ctx, operationStateEvent, state)
}

func validateRequest(request OperationRequestDTO) error {
	switch request.Mode {
	case "complete", "timeout", "fail":
	default:
		return errors.New("mode must be complete, timeout, or fail")
	}
	if request.DurationMilliseconds < 100 || request.DurationMilliseconds > 10_000 {
		return errors.New("durationMilliseconds must be between 100 and 10000")
	}
	return nil
}

func toStateDTO(snapshot operation.Snapshot) OperationStateDTO {
	return OperationStateDTO{
		OperationID: snapshot.ID,
		State:       string(snapshot.State),
		ErrorCode:   snapshot.ErrorCode,
		Message:     snapshot.Message,
	}
}
