//go:build windows

package chromewindowtracking

import (
	"context"
	"os"
	"testing"
	"time"

	application "github.com/asam-masa/browser-launcher/internal/application/chromewindowtracking"
	"golang.org/x/sys/windows"
)

func TestWindowsChangeHookCanCloseAndRestart(t *testing.T) {
	first, err := startChangeHook()
	if err != nil {
		t.Fatalf("first startChangeHook() error = %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("first Close() error = %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("second first.Close() error = %v", err)
	}

	second, err := startChangeHook()
	if err != nil {
		t.Fatalf("second startChangeHook() error = %v", err)
	}
	if err := second.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
}

func TestWindowsChangeHookReceivesNotification(t *testing.T) {
	hook, err := startChangeHook()
	if err != nil {
		t.Fatalf("startChangeHook() error = %v", err)
	}
	t.Cleanup(func() {
		if err := hook.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})

	procNotifyWinEvent := hookUser32.NewProc("NotifyWinEvent")
	window := windows.GetForegroundWindow()
	if window == 0 {
		window = windows.GetDesktopWindow()
	}
	procNotifyWinEvent.Call(eventObjectShow, uintptr(window), 0, 0)

	select {
	case <-hook.Notifications():
	case <-time.After(2 * time.Second):
		t.Fatal("WinEvent notification was not received")
	}
}

func TestWindowsObservationReleasesHookAfterContextEnds(t *testing.T) {
	executablePath := os.Getenv("BROWSER_LAUNCHER_CHROME_EXECUTABLE")
	if executablePath == "" {
		t.Skip("BROWSER_LAUNCHER_CHROME_EXECUTABLE is not set")
	}

	for _, test := range []struct {
		name        string
		context     func() (context.Context, context.CancelFunc)
		wantOutcome application.Outcome
	}{
		{
			name: "cancelled",
			context: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx, func() {}
			},
			wantOutcome: application.OutcomeCancelled,
		},
		{
			name: "timed out",
			context: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), time.Second)
			},
			wantOutcome: application.OutcomeTimedOut,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := application.NewService(NewProvider(executablePath))
			session, err := service.Begin()
			if err != nil {
				t.Fatalf("Begin() error = %v", err)
			}
			ctx, cancel := test.context()
			defer cancel()

			if got := session.Wait(ctx); got.Outcome != test.wantOutcome {
				t.Fatalf("Wait() outcome = %q, want %q; error = %v", got.Outcome, test.wantOutcome, got.Err)
			}
		})
	}

	service := application.NewService(NewProvider(executablePath))
	session, err := service.Begin()
	if err != nil {
		t.Fatalf("Begin() after context completion error = %v", err)
	}
	if err := session.Close(); err != nil {
		t.Fatalf("Close() after restart error = %v", err)
	}
}
