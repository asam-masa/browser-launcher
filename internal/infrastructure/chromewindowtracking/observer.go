package chromewindowtracking

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	application "github.com/asam-masa/browser-launcher/internal/application/chromewindowtracking"
)

const defaultPollInterval = 500 * time.Millisecond

var ErrObservationStartFailed = errors.New("chrome window observation start failed")

type pollingTimer interface {
	Ticks() <-chan time.Time
	Stop()
}

type ticker struct {
	*time.Ticker
}

func newTicker(interval time.Duration) pollingTimer {
	return ticker{Ticker: time.NewTicker(interval)}
}

func (t ticker) Ticks() <-chan time.Time {
	return t.C
}

func (p Provider) Start() (application.Observation, error) {
	baseline, err := p.Snapshot()
	if err != nil {
		return nil, fmt.Errorf("%w: capture baseline: %w", ErrObservationStartFailed, err)
	}
	if p.startHook == nil || p.newPoller == nil || p.pollInterval <= 0 {
		return nil, fmt.Errorf("%w: provider is not configured", ErrObservationStartFailed)
	}

	hook, err := p.startHook()
	if err != nil {
		return nil, fmt.Errorf("%w: start WinEvent hook: %w", ErrObservationStartFailed, err)
	}

	return &observation{
		provider: p,
		baseline: append(application.Snapshot(nil), baseline...),
		hook:     hook,
		poller:   p.newPoller(p.pollInterval),
	}, nil
}

type observation struct {
	provider Provider
	baseline application.Snapshot
	hook     changeHook
	poller   pollingTimer
	close    sync.Once
	closeErr error
}

func (o *observation) Baseline() application.Snapshot {
	return append(application.Snapshot(nil), o.baseline...)
}

func (o *observation) Next(ctx context.Context) application.Event {
	select {
	case <-ctx.Done():
		return contextEvent(ctx)
	case err := <-o.hook.Failures():
		if err == nil {
			err = errors.New("WinEvent hook failed without an error")
		}
		return application.Event{Type: application.EventFailed, Err: err}
	case <-o.hook.Notifications():
		return o.snapshotEvent()
	case <-o.poller.Ticks():
		return o.snapshotEvent()
	}
}

func (o *observation) Close() error {
	o.close.Do(func() {
		o.poller.Stop()
		o.closeErr = o.hook.Close()
	})
	return o.closeErr
}

func (o *observation) snapshotEvent() application.Event {
	snapshot, err := o.provider.Snapshot()
	if err != nil {
		return application.Event{Type: application.EventFailed, Err: err}
	}
	return application.Event{Type: application.EventSnapshot, Snapshot: snapshot}
}

func contextEvent(ctx context.Context) application.Event {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return application.Event{Type: application.EventTimedOut}
	}
	return application.Event{Type: application.EventCancelled}
}

var _ application.Observation = (*observation)(nil)
