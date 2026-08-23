package chromelaunchoperation

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

type record struct {
	snapshot Snapshot
	cancel   context.CancelFunc
}

// Registry retains the latest operation until the next operation starts.
// It permits only one non-terminal operation at a time.
type Registry struct {
	mu       sync.RWMutex
	sequence uint64
	current  *record
}

func NewRegistry() *Registry {
	return &Registry{}
}

func (r *Registry) Start(parent context.Context) (Snapshot, context.Context, error) {
	if parent == nil {
		return Snapshot{}, nil, ErrNilContext
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.current != nil && !isTerminal(r.current.snapshot.State) {
		return Snapshot{}, nil, ErrOperationInProgress
	}

	r.sequence++
	operationContext, cancel := context.WithCancel(parent)
	snapshot := Snapshot{
		ID:    ID(fmt.Sprintf("operation-%d", r.sequence)),
		State: StateStarting,
	}
	r.current = &record{
		snapshot: snapshot,
		cancel:   cancel,
	}

	return snapshot, operationContext, nil
}

func (r *Registry) Transition(id ID, next State) (Snapshot, error) {
	r.mu.Lock()
	if r.current == nil || r.current.snapshot.ID != id {
		r.mu.Unlock()
		return Snapshot{}, ErrOperationNotFound
	}

	current := r.current.snapshot.State
	if !canTransition(current, next) {
		r.mu.Unlock()
		return Snapshot{}, errors.Join(
			ErrInvalidTransition,
			fmt.Errorf("cannot transition from %q to %q", current, next),
		)
	}

	r.current.snapshot.State = next
	snapshot := r.current.snapshot
	cancel := r.current.cancel
	r.mu.Unlock()

	if isTerminal(next) {
		cancel()
	}

	return snapshot, nil
}

// Finish atomically resolves an operation to a terminal state. An accepted
// cancellation always converges to StateCancelled.
func (r *Registry) Finish(id ID, terminal State, errorCode ErrorCode) (Snapshot, error) {
	if !isTerminal(terminal) {
		return Snapshot{}, ErrInvalidTerminalState
	}

	r.mu.Lock()
	if r.current == nil || r.current.snapshot.ID != id {
		r.mu.Unlock()
		return Snapshot{}, ErrOperationNotFound
	}
	if isTerminal(r.current.snapshot.State) {
		r.mu.Unlock()
		return Snapshot{}, ErrInvalidTransition
	}
	if r.current.snapshot.State == StateCancelling {
		terminal = StateCancelled
		errorCode = ErrorCodeNone
	}
	if !canTransition(r.current.snapshot.State, terminal) {
		current := r.current.snapshot.State
		r.mu.Unlock()
		return Snapshot{}, errors.Join(
			ErrInvalidTransition,
			fmt.Errorf("cannot transition from %q to %q", current, terminal),
		)
	}
	r.current.snapshot.State = terminal
	r.current.snapshot.ErrorCode = errorCode
	snapshot := r.current.snapshot
	cancel := r.current.cancel
	r.mu.Unlock()

	cancel()
	return snapshot, nil
}

func (r *Registry) Cancel(id ID) (CancelStatus, Snapshot) {
	r.mu.Lock()
	if r.current == nil || r.current.snapshot.ID != id {
		r.mu.Unlock()
		return CancelNotFound, Snapshot{}
	}

	switch r.current.snapshot.State {
	case StateCancelling:
		snapshot := r.current.snapshot
		r.mu.Unlock()
		return CancelAlreadyCancelling, snapshot
	case StateCompleted, StateCancelled, StateTimedOut, StateFailed:
		snapshot := r.current.snapshot
		r.mu.Unlock()
		return CancelAlreadyFinished, snapshot
	default:
		r.current.snapshot.State = StateCancelling
		snapshot := r.current.snapshot
		cancel := r.current.cancel
		r.mu.Unlock()
		cancel()
		return CancelAccepted, snapshot
	}
}

func (r *Registry) Get(id ID) (Snapshot, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.current == nil || r.current.snapshot.ID != id {
		return Snapshot{}, false
	}
	return r.current.snapshot, true
}

func (r *Registry) Current() (Snapshot, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.current == nil {
		return Snapshot{}, false
	}
	return r.current.snapshot, true
}
