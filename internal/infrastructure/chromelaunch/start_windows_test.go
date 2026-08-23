//go:build windows

package chromelaunch

import (
	"errors"
	"os"
	"slices"
	"testing"

	detection "github.com/asam-masa/browser-launcher/internal/application/chromedetection"
	application "github.com/asam-masa/browser-launcher/internal/application/chromelaunch"
	infraDetection "github.com/asam-masa/browser-launcher/internal/infrastructure/chromedetection"
)

type fakeProcessStarter struct {
	process        process
	err            error
	executablePath string
	arguments      []string
}

type fakeProcess struct {
	processID  int
	releaseErr error
	released   bool
}

func TestStartExecutesCommandAndReleasesProcess(t *testing.T) {
	t.Parallel()

	startedProcess := &fakeProcess{processID: 1234}
	starter := &fakeProcessStarter{process: startedProcess}
	command := launchCommand(t)

	result, err := start(starter, command)
	if err != nil {
		t.Fatalf("start() error = %v", err)
	}
	if result.ProcessID != 1234 {
		t.Fatalf("ProcessID = %d, want 1234", result.ProcessID)
	}
	if starter.executablePath != command.ExecutablePath() {
		t.Fatalf("executablePath = %q, want %q", starter.executablePath, command.ExecutablePath())
	}
	if !slices.Equal(starter.arguments, command.Arguments()) {
		t.Fatalf("arguments = %q, want %q", starter.arguments, command.Arguments())
	}
	if !startedProcess.released {
		t.Fatal("Release() was not called")
	}
}

func TestStartReturnsProcessStartFailure(t *testing.T) {
	t.Parallel()

	starter := &fakeProcessStarter{err: errors.New("access denied")}

	result, err := start(starter, launchCommand(t))
	if err == nil {
		t.Fatal("start() error = nil, want error")
	}
	if result.ProcessID != 0 {
		t.Fatalf("ProcessID = %d, want 0", result.ProcessID)
	}
}

func TestStartReturnsPartialResultOnReleaseFailure(t *testing.T) {
	t.Parallel()

	startedProcess := &fakeProcess{
		processID:  1234,
		releaseErr: errors.New("release failed"),
	}

	result, err := start(&fakeProcessStarter{process: startedProcess}, launchCommand(t))
	if err == nil {
		t.Fatal("start() error = nil, want error")
	}
	if result.ProcessID != 1234 {
		t.Fatalf("ProcessID = %d, want partial result 1234", result.ProcessID)
	}
}

func TestProcessAttributesCloseStandardStreams(t *testing.T) {
	t.Parallel()

	attributes := processAttributes()
	if len(attributes.Files) != 3 {
		t.Fatalf("len(Files) = %d, want 3", len(attributes.Files))
	}
	for index, file := range attributes.Files {
		if file != nil {
			t.Fatalf("Files[%d] = %v, want nil", index, file)
		}
	}
}

func TestProviderStartIntegration(t *testing.T) {
	if os.Getenv("BROWSER_LAUNCHER_RUN_WINDOWS_INTEGRATION") != "1" {
		t.Skip("set BROWSER_LAUNCHER_RUN_WINDOWS_INTEGRATION=1 to launch the installed Chrome")
	}

	detector := infraDetection.NewProvider()
	service := application.NewService(detector, NewProvider())
	result, err := service.Launch()
	if err != nil {
		t.Fatalf("Launch() error = %v", err)
	}
	if result.ProcessID < 1 {
		t.Fatalf("ProcessID = %d, want positive value", result.ProcessID)
	}
}

func launchCommand(t *testing.T) application.Command {
	t.Helper()

	launcher := &captureLauncher{result: application.Result{ProcessID: 1}}
	service := application.NewService(
		fixedDetector{},
		launcher,
	)
	if _, err := service.Launch(); err != nil {
		t.Fatalf("Launch() error = %v", err)
	}
	return launcher.command
}

type fixedDetector struct{}

func (fixedDetector) Detect() (detection.Installation, error) {
	return detection.Installation{ExecutablePath: `C:\Program Files\Google\Chrome\Application\chrome.exe`}, nil
}

type captureLauncher struct {
	result  application.Result
	command application.Command
}

func (l *captureLauncher) Start(command application.Command) (application.Result, error) {
	l.command = command
	return l.result, nil
}

func (s *fakeProcessStarter) Start(executablePath string, arguments ...string) (process, error) {
	s.executablePath = executablePath
	s.arguments = slices.Clone(arguments)
	return s.process, s.err
}

func (p *fakeProcess) PID() int {
	return p.processID
}

func (p *fakeProcess) Release() error {
	p.released = true
	return p.releaseErr
}
