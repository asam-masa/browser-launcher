package chromewindowtracking

import (
	"context"
	"errors"
	"fmt"
)

type Service struct {
	observer Observer
}

func NewService(observer Observer) Service {
	return Service{observer: observer}
}

func (s Service) Begin() (*Session, error) {
	observation, err := s.observer.Start()
	if err != nil {
		return nil, errors.Join(ErrObservationFailed, fmt.Errorf("start observation: %w", err))
	}
	return &Session{
		observation: observation,
		tracker:     newCandidateTracker(observation.Baseline()),
	}, nil
}

type Session struct {
	observation Observation
	tracker     candidateTracker
	closed      bool
	closeErr    error
	used        bool
}

func (s *Session) Wait(ctx context.Context) Result {
	if s.used {
		return failedResult(ErrSessionAlreadyUsed)
	}
	if s.closed {
		return failedResult(errors.Join(ErrSessionClosed, s.closeErr))
	}
	s.used = true

	result := s.wait(ctx)
	if err := s.close(); err != nil {
		return failedResult(errors.Join(
			result.Err,
			ErrObservationFailed,
			fmt.Errorf("close observation: %w", err),
		))
	}
	return result
}

func (s *Session) Close() error {
	return s.close()
}

func (s *Session) wait(ctx context.Context) Result {
	for {
		if result, done := contextResult(ctx); done {
			return result
		}
		event := s.observation.Next(ctx)
		if result, done := contextResult(ctx); done {
			return result
		}
		switch event.Type {
		case EventSnapshot:
			if candidate, ok := s.tracker.observe(event.Snapshot); ok {
				return Result{Outcome: OutcomeCompleted, Candidate: candidate}
			}
		case EventCancelled:
			return Result{Outcome: OutcomeCancelled}
		case EventTimedOut:
			return s.tracker.timeoutResult()
		case EventFailed:
			if event.Err == nil {
				return failedResult(errors.Join(ErrInvalidEvent, errors.New("failed event has no error")))
			}
			return failedResult(errors.Join(ErrObservationFailed, event.Err))
		default:
			return failedResult(fmt.Errorf("%w: type %q", ErrInvalidEvent, event.Type))
		}
	}
}

func contextResult(ctx context.Context) (Result, bool) {
	switch {
	case errors.Is(ctx.Err(), context.Canceled):
		return Result{Outcome: OutcomeCancelled}, true
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return Result{Outcome: OutcomeTimedOut}, true
	default:
		return Result{}, false
	}
}

func (s *Session) close() error {
	if s.closed {
		return s.closeErr
	}
	s.closed = true
	s.closeErr = s.observation.Close()
	return s.closeErr
}

func failedResult(err error) Result {
	return Result{Outcome: OutcomeFailed, Err: err}
}

type candidateTracker struct {
	baseline                     map[windowKey]struct{}
	observed                     map[windowKey]struct{}
	firstSeenGeneration          map[windowKey]uint64
	current                      map[windowKey]Window
	generation                   uint64
	lastDisappearedGeneration    uint64
	replacementObserved          bool
	concurrentCandidatesObserved bool
}

func newCandidateTracker(baseline Snapshot) candidateTracker {
	return candidateTracker{
		baseline:            snapshotKeys(baseline),
		observed:            make(map[windowKey]struct{}),
		firstSeenGeneration: make(map[windowKey]uint64),
		current:             make(map[windowKey]Window),
	}
}

func (t *candidateTracker) observe(snapshot Snapshot) (Window, bool) {
	t.generation++
	next := make(map[windowKey]Window)
	for _, window := range snapshot {
		key := keyOf(window)
		if _, existed := t.baseline[key]; existed {
			continue
		}
		next[key] = window
		if _, observed := t.observed[key]; !observed {
			t.observed[key] = struct{}{}
			t.firstSeenGeneration[key] = t.generation
		}
	}
	if len(next) > 1 {
		t.concurrentCandidatesObserved = true
	}

	for key := range t.current {
		if _, remains := next[key]; remains {
			continue
		}
		if disappearedGeneration := t.firstSeenGeneration[key]; disappearedGeneration > t.lastDisappearedGeneration {
			t.lastDisappearedGeneration = disappearedGeneration
		}
	}
	t.current = next
	for key := range t.current {
		if t.firstSeenGeneration[key] > t.lastDisappearedGeneration && t.lastDisappearedGeneration > 0 {
			t.replacementObserved = true
			break
		}
	}

	if !t.replacementObserved || len(t.current) != 1 {
		return Window{}, false
	}
	for _, candidate := range t.current {
		return candidate, true
	}
	return Window{}, false
}

func (t candidateTracker) timeoutResult() Result {
	switch {
	case len(t.current) == 0 && len(t.observed) > 0:
		return Result{Outcome: OutcomeCandidateLost, Err: ErrCandidateLost}
	case t.concurrentCandidatesObserved:
		return Result{Outcome: OutcomeAmbiguous, Err: ErrAmbiguousCandidates}
	default:
		return Result{Outcome: OutcomeTimedOut}
	}
}

func snapshotKeys(snapshot Snapshot) map[windowKey]struct{} {
	keys := make(map[windowKey]struct{}, len(snapshot))
	for _, window := range snapshot {
		keys[keyOf(window)] = struct{}{}
	}
	return keys
}
