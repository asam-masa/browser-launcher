package operation

import (
	"context"
	"errors"
	"sync"
)

type State string

const (
	StateStarting   State = "starting"
	StateRunning    State = "running"
	StateCancelling State = "cancelling"
	StateCompleted  State = "completed"
	StateCancelled  State = "cancelled"
	StateTimedOut   State = "timed_out"
	StateFailed     State = "failed"
)

var ErrOperationExists = errors.New("operation already exists")

type Snapshot struct {
	ID        string
	State     State
	ErrorCode string
	Message   string
}

type CancelStatus string

const (
	CancelAccepted          CancelStatus = "accepted"
	CancelAlreadyCancelling CancelStatus = "already_cancelling"
	CancelAlreadyFinished   CancelStatus = "already_finished"
	CancelNotFound          CancelStatus = "not_found"
)

type record struct {
	snapshot Snapshot
	cancel   context.CancelFunc
}

type Registry struct {
	mu         sync.RWMutex
	operations map[string]*record
}

func NewRegistry() *Registry {
	return &Registry{
		operations: make(map[string]*record),
	}
}

func (r *Registry) Create(id string, cancel context.CancelFunc) (Snapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.operations[id]; exists {
		return Snapshot{}, ErrOperationExists
	}

	snapshot := Snapshot{
		ID:    id,
		State: StateStarting,
	}
	r.operations[id] = &record{
		snapshot: snapshot,
		cancel:   cancel,
	}
	return snapshot, nil
}

func (r *Registry) Transition(
	id string,
	state State,
	errorCode string,
	message string,
) (Snapshot, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	item, exists := r.operations[id]
	if !exists || isTerminal(item.snapshot.State) {
		return Snapshot{}, false
	}
	if item.snapshot.State == StateCancelling && state != StateCancelled {
		return Snapshot{}, false
	}

	item.snapshot.State = state
	item.snapshot.ErrorCode = errorCode
	item.snapshot.Message = message
	return item.snapshot, true
}

func (r *Registry) Get(id string) (Snapshot, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	item, exists := r.operations[id]
	if !exists {
		return Snapshot{}, false
	}
	return item.snapshot, true
}

func (r *Registry) Cancel(id string) (CancelStatus, Snapshot) {
	r.mu.Lock()
	defer r.mu.Unlock()

	item, exists := r.operations[id]
	if !exists {
		return CancelNotFound, Snapshot{}
	}
	if isTerminal(item.snapshot.State) {
		return CancelAlreadyFinished, item.snapshot
	}
	if item.snapshot.State == StateCancelling {
		return CancelAlreadyCancelling, item.snapshot
	}

	item.snapshot.State = StateCancelling
	item.cancel()
	return CancelAccepted, item.snapshot
}

func (r *Registry) CancelAll() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, item := range r.operations {
		if !isTerminal(item.snapshot.State) {
			item.cancel()
		}
	}
}

func isTerminal(state State) bool {
	switch state {
	case StateCompleted, StateCancelled, StateTimedOut, StateFailed:
		return true
	default:
		return false
	}
}
