package chromelaunchoperation

import "errors"

var (
	ErrOperationInProgress  = errors.New("chrome launch operation is already in progress")
	ErrOperationNotFound    = errors.New("chrome launch operation was not found")
	ErrInvalidTransition    = errors.New("chrome launch operation state transition is invalid")
	ErrNilContext           = errors.New("chrome launch operation context is nil")
	ErrInvalidTerminalState = errors.New("chrome launch operation terminal state is invalid")
)

type ID string

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

type Snapshot struct {
	ID        ID
	State     State
	ErrorCode ErrorCode
}

type CancelStatus string

const (
	CancelAccepted          CancelStatus = "accepted"
	CancelAlreadyCancelling CancelStatus = "already_cancelling"
	CancelAlreadyFinished   CancelStatus = "already_finished"
	CancelNotFound          CancelStatus = "not_found"
)

func isTerminal(state State) bool {
	switch state {
	case StateCompleted, StateCancelled, StateTimedOut, StateFailed:
		return true
	default:
		return false
	}
}

func canTransition(from State, to State) bool {
	switch from {
	case StateStarting:
		return to == StateRunning || to == StateFailed
	case StateRunning:
		return to == StateCompleted || to == StateTimedOut || to == StateFailed
	case StateCancelling:
		return to == StateCancelled
	default:
		return false
	}
}
