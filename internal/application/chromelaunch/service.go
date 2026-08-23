package chromelaunch

import (
	"errors"
	"fmt"
	"slices"

	detection "github.com/asam-masa/browser-launcher/internal/application/chromedetection"
)

var (
	ErrChromeNotFound      = errors.New("chrome installation not found")
	ErrUnsupportedPlatform = errors.New("chrome launch is only supported on Windows")
	ErrLaunchFailed        = errors.New("chrome launch failed")
)

const newWindowArgument = "--new-window"

type Command struct {
	executablePath string
	arguments      []string
}

func (c Command) ExecutablePath() string {
	return c.executablePath
}

func (c Command) Arguments() []string {
	return slices.Clone(c.arguments)
}

type Result struct {
	ProcessID int
}

type ChromeDetector interface {
	Detect() (detection.Installation, error)
}

type ProcessLauncher interface {
	Start(command Command) (Result, error)
}

type Service struct {
	detector ChromeDetector
	launcher ProcessLauncher
}

func NewService(detector ChromeDetector, launcher ProcessLauncher) Service {
	return Service{
		detector: detector,
		launcher: launcher,
	}
}

func (s Service) Launch() (Result, error) {
	installation, err := s.Detect()
	if err != nil {
		return Result{}, err
	}
	return s.LaunchDetected(installation)
}

func (s Service) Detect() (detection.Installation, error) {
	installation, err := s.detector.Detect()
	if err != nil {
		return detection.Installation{}, classifyDetectionError(err)
	}
	return installation, nil
}

func (s Service) LaunchDetected(installation detection.Installation) (Result, error) {
	command := Command{
		executablePath: installation.ExecutablePath,
		arguments:      []string{newWindowArgument},
	}
	result, err := s.launcher.Start(command)
	if err != nil {
		return result, errors.Join(ErrLaunchFailed, fmt.Errorf("start Chrome process: %w", err))
	}
	if result.ProcessID < 1 {
		return Result{}, fmt.Errorf("%w: launcher returned invalid process ID", ErrLaunchFailed)
	}

	return result, nil
}

func classifyDetectionError(err error) error {
	switch {
	case errors.Is(err, detection.ErrNotFound):
		return errors.Join(ErrChromeNotFound, fmt.Errorf("detect Chrome: %w", err))
	case errors.Is(err, detection.ErrUnsupportedPlatform):
		return errors.Join(ErrUnsupportedPlatform, fmt.Errorf("detect Chrome: %w", err))
	default:
		return fmt.Errorf("detect Chrome: %w", err)
	}
}
