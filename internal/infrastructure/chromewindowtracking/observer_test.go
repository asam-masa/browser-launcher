package chromewindowtracking

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	application "github.com/asam-masa/browser-launcher/internal/application/chromewindowtracking"
)

func TestProviderStartCapturesBaselineBeforeStartingHook(t *testing.T) {
	t.Parallel()

	system := &stubWindowSystem{
		handles:      []uint64{1},
		metadata:     map[uint64]windowMetadata{1: chromeMetadata(1, 10, 100)},
		revalidation: map[uint64]bool{1: true},
	}
	hook := newStubChangeHook()
	provider := observerProvider(system, hook)
	provider.startHook = func() (changeHook, error) {
		if system.enumerateCalls != 1 {
			t.Fatalf("Enumerate() calls before startHook = %d, want 1", system.enumerateCalls)
		}
		return hook, nil
	}

	got, err := provider.Start()
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() { _ = got.Close() })

	want := application.Snapshot{{Handle: 1, ProcessID: 10, ProcessStartTime: 100}}
	if !reflect.DeepEqual(got.Baseline(), want) {
		t.Fatalf("Baseline() = %#v, want %#v", got.Baseline(), want)
	}
}

func TestObservationNextSnapshotsOnNotificationAndPolling(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name    string
		trigger func(*stubChangeHook, *stubPollingTimer)
	}{
		{name: "notification", trigger: func(h *stubChangeHook, _ *stubPollingTimer) { h.notifications <- struct{}{} }},
		{name: "polling", trigger: func(_ *stubChangeHook, p *stubPollingTimer) { p.ticks <- time.Time{} }},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			system := &stubWindowSystem{
				handles:      []uint64{1},
				metadata:     map[uint64]windowMetadata{1: chromeMetadata(1, 10, 100)},
				revalidation: map[uint64]bool{1: true},
			}
			hook := newStubChangeHook()
			poller := newStubPollingTimer()
			provider := observerProvider(system, hook)
			provider.newPoller = func(time.Duration) pollingTimer { return poller }

			started, err := provider.Start()
			if err != nil {
				t.Fatalf("Start() error = %v", err)
			}
			t.Cleanup(func() { _ = started.Close() })
			system.handles = []uint64{2}
			system.metadata = map[uint64]windowMetadata{2: chromeMetadata(2, 20, 200)}
			system.revalidation = map[uint64]bool{2: true}
			test.trigger(hook, poller)

			got := started.Next(context.Background())
			want := application.Snapshot{{Handle: 2, ProcessID: 20, ProcessStartTime: 200}}
			if got.Type != application.EventSnapshot || !reflect.DeepEqual(got.Snapshot, want) {
				t.Fatalf("Next() = %#v, want snapshot %#v", got, want)
			}
		})
	}
}

func TestObservationNextReportsContextAndHookFailure(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name      string
		prepare   func(*stubChangeHook) context.Context
		wantType  application.EventType
		wantError error
	}{
		{
			name: "cancelled",
			prepare: func(*stubChangeHook) context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			wantType: application.EventCancelled,
		},
		{
			name: "timed out",
			prepare: func(*stubChangeHook) context.Context {
				ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
				cancel()
				return ctx
			},
			wantType: application.EventTimedOut,
		},
		{
			name: "hook failure",
			prepare: func(h *stubChangeHook) context.Context {
				h.failures <- errTestHook
				return context.Background()
			},
			wantType:  application.EventFailed,
			wantError: errTestHook,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			hook := newStubChangeHook()
			observation := &observation{hook: hook, poller: newStubPollingTimer()}
			got := observation.Next(test.prepare(hook))
			if got.Type != test.wantType || !errors.Is(got.Err, test.wantError) {
				t.Fatalf("Next() = %#v, want type %q and error %v", got, test.wantType, test.wantError)
			}
		})
	}
}

func TestObservationCloseStopsResourcesOnce(t *testing.T) {
	t.Parallel()

	hook := newStubChangeHook()
	hook.closeErr = errTestHook
	poller := newStubPollingTimer()
	observation := &observation{hook: hook, poller: poller}

	if err := observation.Close(); !errors.Is(err, errTestHook) {
		t.Fatalf("Close() error = %v, want %v", err, errTestHook)
	}
	if err := observation.Close(); !errors.Is(err, errTestHook) {
		t.Fatalf("second Close() error = %v, want %v", err, errTestHook)
	}
	if hook.closeCalls != 1 || poller.stopCalls != 1 {
		t.Fatalf("resource close calls = hook %d, poller %d; want 1 each", hook.closeCalls, poller.stopCalls)
	}
}

func TestProviderStartReturnsHookFailure(t *testing.T) {
	t.Parallel()

	system := &stubWindowSystem{metadata: map[uint64]windowMetadata{}}
	provider := observerProvider(system, newStubChangeHook())
	provider.startHook = func() (changeHook, error) { return nil, errTestHook }

	_, err := provider.Start()
	if !errors.Is(err, ErrObservationStartFailed) || !errors.Is(err, errTestHook) {
		t.Fatalf("Start() error = %v, want ErrObservationStartFailed and %v", err, errTestHook)
	}
}

var errTestHook = errors.New("test hook failure")

func observerProvider(system *stubWindowSystem, hook *stubChangeHook) Provider {
	return Provider{
		executablePath: "/chrome",
		system:         system,
		startHook:      func() (changeHook, error) { return hook, nil },
		newPoller:      func(time.Duration) pollingTimer { return newStubPollingTimer() },
		pollInterval:   defaultPollInterval,
	}
}

type stubChangeHook struct {
	notifications chan struct{}
	failures      chan error
	closeErr      error
	closeCalls    int
}

func newStubChangeHook() *stubChangeHook {
	return &stubChangeHook{notifications: make(chan struct{}, 1), failures: make(chan error, 1)}
}

func (h *stubChangeHook) Notifications() <-chan struct{} { return h.notifications }
func (h *stubChangeHook) Failures() <-chan error         { return h.failures }
func (h *stubChangeHook) Close() error {
	h.closeCalls++
	return h.closeErr
}

type stubPollingTimer struct {
	ticks     chan time.Time
	stopCalls int
}

func newStubPollingTimer() *stubPollingTimer {
	return &stubPollingTimer{ticks: make(chan time.Time, 1)}
}

func (p *stubPollingTimer) Ticks() <-chan time.Time { return p.ticks }
func (p *stubPollingTimer) Stop()                   { p.stopCalls++ }
