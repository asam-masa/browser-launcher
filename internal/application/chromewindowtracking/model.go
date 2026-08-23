package chromewindowtracking

import (
	"context"
	"errors"
)

var (
	ErrObservationFailed   = errors.New("chrome window observation failed")
	ErrSessionClosed       = errors.New("chrome window tracking session is closed")
	ErrSessionAlreadyUsed  = errors.New("chrome window tracking session was already used")
	ErrInvalidEvent        = errors.New("chrome window tracking event is invalid")
	ErrAmbiguousCandidates = errors.New("multiple chrome window candidates remain")
	ErrCandidateLost       = errors.New("chrome window candidate disappeared")
)

type Window struct {
	Handle           uint64
	ProcessID        uint32
	ProcessStartTime uint64
}

// Snapshot contains Chrome top-level windows whose handles, processes, class,
// and visibility were revalidated by the observation boundary.
type Snapshot []Window

type Outcome string

const (
	OutcomeCompleted     Outcome = "completed"
	OutcomeCancelled     Outcome = "cancelled"
	OutcomeTimedOut      Outcome = "timed_out"
	OutcomeFailed        Outcome = "failed"
	OutcomeAmbiguous     Outcome = "ambiguous"
	OutcomeCandidateLost Outcome = "candidate_lost"
)

type Result struct {
	Outcome   Outcome
	Candidate Window
	Err       error
}

type EventType string

const (
	EventSnapshot  EventType = "snapshot"
	EventCancelled EventType = "cancelled"
	EventTimedOut  EventType = "timed_out"
	EventFailed    EventType = "failed"
)

type Event struct {
	Type     EventType
	Snapshot Snapshot
	Err      error
}

type Observer interface {
	// Start captures the pre-launch baseline, prepares change notifications,
	// and returns only after the observation is ready.
	Start() (Observation, error)
}

type Observation interface {
	Baseline() Snapshot
	// Next waits for the next observation event and must return when ctx ends.
	Next(ctx context.Context) Event
	// Close releases observation resources and must be safe to call repeatedly.
	Close() error
}

type windowKey struct {
	handle           uint64
	processID        uint32
	processStartTime uint64
}

func keyOf(window Window) windowKey {
	return windowKey{
		handle:           window.Handle,
		processID:        window.ProcessID,
		processStartTime: window.ProcessStartTime,
	}
}
