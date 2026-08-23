package chromewindowtracking

import (
	"context"
	"errors"
	"testing"
	"time"
)

var (
	baselineWindow     = Window{Handle: 1, ProcessID: 10, ProcessStartTime: 100}
	profilePicker      = Window{Handle: 2, ProcessID: 20, ProcessStartTime: 200}
	browserWindow      = Window{Handle: 3, ProcessID: 30, ProcessStartTime: 300}
	otherBrowserWindow = Window{Handle: 4, ProcessID: 40, ProcessStartTime: 400}
)

func TestSessionWaitCompletesOnlyAfterUniqueReplacement(t *testing.T) {
	t.Parallel()

	observation := &fakeObservation{
		baseline: Snapshot{baselineWindow},
		events: []Event{
			{Type: EventSnapshot, Snapshot: Snapshot{baselineWindow, profilePicker}},
			{Type: EventSnapshot, Snapshot: Snapshot{baselineWindow, profilePicker, browserWindow}},
			{Type: EventSnapshot, Snapshot: Snapshot{baselineWindow, browserWindow}},
		},
	}
	session := beginTestSession(t, observation)

	result := session.Wait(context.Background())

	if result.Outcome != OutcomeCompleted {
		t.Fatalf("Outcome = %q, want %q", result.Outcome, OutcomeCompleted)
	}
	if result.Candidate != browserWindow {
		t.Fatalf("Candidate = %+v, want %+v", result.Candidate, browserWindow)
	}
	if !observation.closed {
		t.Fatal("observation was not closed")
	}
}

func TestSessionWaitCompletesWhenSnapshotsContainReplacementGap(t *testing.T) {
	t.Parallel()

	observation := &fakeObservation{events: []Event{
		{Type: EventSnapshot, Snapshot: Snapshot{profilePicker}},
		{Type: EventSnapshot, Snapshot: Snapshot{}},
		{Type: EventSnapshot, Snapshot: Snapshot{browserWindow}},
	}}
	result := beginTestSession(t, observation).Wait(context.Background())

	if result.Outcome != OutcomeCompleted {
		t.Fatalf("Outcome = %q, want %q", result.Outcome, OutcomeCompleted)
	}
	if result.Candidate != browserWindow {
		t.Fatalf("Candidate = %+v, want %+v", result.Candidate, browserWindow)
	}
}

func TestSessionWaitClassifiesTerminalEvents(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		events      []Event
		wantOutcome Outcome
		wantErr     error
	}{
		{
			name:        "cancelled",
			events:      []Event{{Type: EventCancelled}},
			wantOutcome: OutcomeCancelled,
		},
		{
			name: "direct candidate times out without replacement",
			events: []Event{
				{Type: EventSnapshot, Snapshot: Snapshot{profilePicker}},
				{Type: EventTimedOut},
			},
			wantOutcome: OutcomeTimedOut,
		},
		{
			name: "multiple candidates remain",
			events: []Event{
				{Type: EventSnapshot, Snapshot: Snapshot{profilePicker, browserWindow}},
				{Type: EventTimedOut},
			},
			wantOutcome: OutcomeAmbiguous,
			wantErr:     ErrAmbiguousCandidates,
		},
		{
			name: "candidate disappears",
			events: []Event{
				{Type: EventSnapshot, Snapshot: Snapshot{profilePicker}},
				{Type: EventSnapshot, Snapshot: Snapshot{}},
				{Type: EventTimedOut},
			},
			wantOutcome: OutcomeCandidateLost,
			wantErr:     ErrCandidateLost,
		},
		{
			name:        "observation fails",
			events:      []Event{{Type: EventFailed, Err: errors.New("snapshot failed")}},
			wantOutcome: OutcomeFailed,
			wantErr:     ErrObservationFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			observation := &fakeObservation{events: tt.events}
			result := beginTestSession(t, observation).Wait(context.Background())
			if result.Outcome != tt.wantOutcome {
				t.Fatalf("Outcome = %q, want %q", result.Outcome, tt.wantOutcome)
			}
			if tt.wantErr != nil && !errors.Is(result.Err, tt.wantErr) {
				t.Fatalf("Err = %v, want %v", result.Err, tt.wantErr)
			}
			if !observation.closed {
				t.Fatal("observation was not closed")
			}
		})
	}
}

func TestSessionWaitClassifiesContextEnd(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		context     func() context.Context
		wantOutcome Outcome
	}{
		{
			name: "cancelled",
			context: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			wantOutcome: OutcomeCancelled,
		},
		{
			name: "deadline exceeded",
			context: func() context.Context {
				ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
				cancel()
				return ctx
			},
			wantOutcome: OutcomeTimedOut,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			observation := &fakeObservation{}
			result := beginTestSession(t, observation).Wait(tt.context())
			if result.Outcome != tt.wantOutcome {
				t.Fatalf("Outcome = %q, want %q", result.Outcome, tt.wantOutcome)
			}
			if !observation.closed {
				t.Fatal("observation was not closed")
			}
		})
	}
}

func TestSessionWaitDoesNotSelectOneOfConcurrentCandidates(t *testing.T) {
	t.Parallel()

	observation := &fakeObservation{events: []Event{
		{Type: EventSnapshot, Snapshot: Snapshot{profilePicker, browserWindow, otherBrowserWindow}},
		{Type: EventSnapshot, Snapshot: Snapshot{browserWindow, otherBrowserWindow}},
		{Type: EventTimedOut},
	}}
	result := beginTestSession(t, observation).Wait(context.Background())

	if result.Outcome != OutcomeAmbiguous {
		t.Fatalf("Outcome = %q, want %q", result.Outcome, OutcomeAmbiguous)
	}
}

func TestSessionWaitDoesNotTreatReducedConcurrentCandidatesAsReplacement(t *testing.T) {
	t.Parallel()

	observation := &fakeObservation{events: []Event{
		{Type: EventSnapshot, Snapshot: Snapshot{browserWindow, otherBrowserWindow}},
		{Type: EventSnapshot, Snapshot: Snapshot{browserWindow}},
		{Type: EventTimedOut},
	}}
	result := beginTestSession(t, observation).Wait(context.Background())

	if result.Outcome != OutcomeAmbiguous {
		t.Fatalf("Outcome = %q, want %q", result.Outcome, OutcomeAmbiguous)
	}
}

func TestSessionWaitReportsCandidateLostAfterConcurrentCandidatesDisappear(t *testing.T) {
	t.Parallel()

	observation := &fakeObservation{events: []Event{
		{Type: EventSnapshot, Snapshot: Snapshot{profilePicker, browserWindow}},
		{Type: EventSnapshot, Snapshot: Snapshot{}},
		{Type: EventTimedOut},
	}}
	result := beginTestSession(t, observation).Wait(context.Background())

	if result.Outcome != OutcomeCandidateLost {
		t.Fatalf("Outcome = %q, want %q", result.Outcome, OutcomeCandidateLost)
	}
	if !errors.Is(result.Err, ErrCandidateLost) {
		t.Fatalf("Err = %v, want %v", result.Err, ErrCandidateLost)
	}
}

func TestSessionWaitReportsCloseFailure(t *testing.T) {
	t.Parallel()

	closeErr := errors.New("unhook failed")
	observation := &fakeObservation{
		events:   []Event{{Type: EventCancelled}},
		closeErr: closeErr,
	}
	result := beginTestSession(t, observation).Wait(context.Background())

	if result.Outcome != OutcomeFailed {
		t.Fatalf("Outcome = %q, want %q", result.Outcome, OutcomeFailed)
	}
	if !errors.Is(result.Err, closeErr) || !errors.Is(result.Err, ErrObservationFailed) {
		t.Fatalf("Err = %v, want close and observation errors", result.Err)
	}
}

func TestSessionClosePreservesFirstFailure(t *testing.T) {
	t.Parallel()

	closeErr := errors.New("unhook failed")
	observation := &fakeObservation{closeErr: closeErr}
	session := beginTestSession(t, observation)

	if err := session.Close(); !errors.Is(err, closeErr) {
		t.Fatalf("first Close() error = %v, want %v", err, closeErr)
	}
	if err := session.Close(); !errors.Is(err, closeErr) {
		t.Fatalf("second Close() error = %v, want %v", err, closeErr)
	}
	result := session.Wait(context.Background())
	if !errors.Is(result.Err, ErrSessionClosed) || !errors.Is(result.Err, closeErr) {
		t.Fatalf("Wait() error = %v, want session and close errors", result.Err)
	}
	if observation.closeCalls != 1 {
		t.Fatalf("Close() calls = %d, want 1", observation.closeCalls)
	}
}

func TestServiceBeginReportsObserverFailure(t *testing.T) {
	t.Parallel()

	startErr := errors.New("hook registration failed")
	_, err := NewService(fakeObserver{err: startErr}).Begin()
	if !errors.Is(err, startErr) || !errors.Is(err, ErrObservationFailed) {
		t.Fatalf("Begin() error = %v, want start and observation errors", err)
	}
}

func TestSessionCannotBeWaitedTwiceOrAfterClose(t *testing.T) {
	t.Parallel()

	t.Run("twice", func(t *testing.T) {
		t.Parallel()

		session := beginTestSession(t, &fakeObservation{events: []Event{{Type: EventCancelled}}})
		_ = session.Wait(context.Background())
		result := session.Wait(context.Background())
		if !errors.Is(result.Err, ErrSessionAlreadyUsed) {
			t.Fatalf("Err = %v, want %v", result.Err, ErrSessionAlreadyUsed)
		}
	})

	t.Run("after explicit close", func(t *testing.T) {
		t.Parallel()

		session := beginTestSession(t, &fakeObservation{})
		if err := session.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
		result := session.Wait(context.Background())
		if !errors.Is(result.Err, ErrSessionClosed) {
			t.Fatalf("Err = %v, want %v", result.Err, ErrSessionClosed)
		}
	})
}

func beginTestSession(t *testing.T, observation *fakeObservation) *Session {
	t.Helper()

	session, err := NewService(fakeObserver{observation: observation}).Begin()
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	return session
}

type fakeObserver struct {
	observation Observation
	err         error
}

func (f fakeObserver) Start() (Observation, error) {
	return f.observation, f.err
}

type fakeObservation struct {
	baseline   Snapshot
	events     []Event
	index      int
	closed     bool
	closeCalls int
	closeErr   error
}

func (f *fakeObservation) Baseline() Snapshot {
	return f.baseline
}

func (f *fakeObservation) Next(ctx context.Context) Event {
	if f.index >= len(f.events) {
		<-ctx.Done()
		return Event{}
	}
	event := f.events[f.index]
	f.index++
	return event
}

func (f *fakeObservation) Close() error {
	f.closed = true
	f.closeCalls++
	return f.closeErr
}
