package chromelaunch

import (
	"errors"
	"slices"
	"testing"

	detection "github.com/asam-masa/browser-launcher/internal/application/chromedetection"
)

const testChromePath = `C:\Program Files\Google\Chrome\Application\chrome.exe`

type fakeDetector struct {
	installation detection.Installation
	err          error
}

type fakeLauncher struct {
	result  Result
	err     error
	command Command
	called  bool
}

func TestServiceLaunchUsesDetectedChromeAndFixedArgument(t *testing.T) {
	t.Parallel()

	launcher := &fakeLauncher{result: Result{ProcessID: 1234}}
	service := NewService(fakeDetector{
		installation: detection.Installation{ExecutablePath: testChromePath},
	}, launcher)

	result, err := service.Launch()
	if err != nil {
		t.Fatalf("Launch() error = %v", err)
	}
	if result.ProcessID != 1234 {
		t.Fatalf("ProcessID = %d, want 1234", result.ProcessID)
	}
	if launcher.command.ExecutablePath() != testChromePath {
		t.Fatalf("ExecutablePath() = %q, want %q", launcher.command.ExecutablePath(), testChromePath)
	}
	if arguments := launcher.command.Arguments(); !slices.Equal(arguments, []string{newWindowArgument}) {
		t.Fatalf("Arguments() = %q, want [%q]", arguments, newWindowArgument)
	}
}

func TestServiceLaunchDoesNotExposeMutableArguments(t *testing.T) {
	t.Parallel()

	launcher := &fakeLauncher{result: Result{ProcessID: 1234}}
	service := NewService(fakeDetector{
		installation: detection.Installation{ExecutablePath: testChromePath},
	}, launcher)

	_, err := service.Launch()
	if err != nil {
		t.Fatalf("Launch() error = %v", err)
	}
	arguments := launcher.command.Arguments()
	arguments[0] = "--user-data-dir=unexpected"
	if got := launcher.command.Arguments(); !slices.Equal(got, []string{newWindowArgument}) {
		t.Fatalf("Arguments() after mutation = %q, want [%q]", got, newWindowArgument)
	}
}

func TestServiceLaunchClassifiesDetectionErrorsWithoutStartingProcess(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		detectErr error
		wantErr   error
	}{
		{name: "not found", detectErr: detection.ErrNotFound, wantErr: ErrChromeNotFound},
		{name: "unsupported", detectErr: detection.ErrUnsupportedPlatform, wantErr: ErrUnsupportedPlatform},
		{name: "infrastructure", detectErr: errors.New("registry unavailable")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			launcher := &fakeLauncher{result: Result{ProcessID: 1234}}
			service := NewService(fakeDetector{err: tt.detectErr}, launcher)

			_, err := service.Launch()
			if err == nil {
				t.Fatal("Launch() error = nil, want error")
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("Launch() error = %v, want %v", err, tt.wantErr)
			}
			if launcher.called {
				t.Fatal("Start() called after detection error")
			}
		})
	}
}

func TestServiceLaunchClassifiesProcessFailure(t *testing.T) {
	t.Parallel()

	launcher := &fakeLauncher{
		result: Result{ProcessID: 1234},
		err:    errors.New("access denied"),
	}
	service := NewService(fakeDetector{
		installation: detection.Installation{ExecutablePath: testChromePath},
	}, launcher)

	result, err := service.Launch()
	if !errors.Is(err, ErrLaunchFailed) {
		t.Fatalf("Launch() error = %v, want %v", err, ErrLaunchFailed)
	}
	if result.ProcessID != 1234 {
		t.Fatalf("ProcessID = %d, want partial result 1234", result.ProcessID)
	}
}

func TestServiceLaunchRejectsInvalidProcessID(t *testing.T) {
	t.Parallel()

	service := NewService(fakeDetector{
		installation: detection.Installation{ExecutablePath: testChromePath},
	}, &fakeLauncher{})

	_, err := service.Launch()
	if !errors.Is(err, ErrLaunchFailed) {
		t.Fatalf("Launch() error = %v, want %v", err, ErrLaunchFailed)
	}
}

func (f fakeDetector) Detect() (detection.Installation, error) {
	return f.installation, f.err
}

func (f *fakeLauncher) Start(command Command) (Result, error) {
	f.called = true
	f.command = command
	return f.result, f.err
}
