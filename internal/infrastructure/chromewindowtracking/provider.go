package chromewindowtracking

import (
	"errors"
	"time"

	application "github.com/asam-masa/browser-launcher/internal/application/chromewindowtracking"
)

var (
	ErrSnapshotFailed      = errors.New("chrome window snapshot failed")
	ErrValidationFailed    = errors.New("chrome window validation failed")
	ErrUnsupportedPlatform = errors.New("chrome window tracking is unsupported on this platform")
)

type Provider struct {
	executablePath string
	system         windowSystem
	startHook      func() (changeHook, error)
	newPoller      func(time.Duration) pollingTimer
	pollInterval   time.Duration
}

func NewProvider(executablePath string) Provider {
	return Provider{
		executablePath: normalizeExecutablePath(executablePath),
		system:         newWindowSystem(),
		startHook:      startChangeHook,
		newPoller:      newTicker,
		pollInterval:   defaultPollInterval,
	}
}

func (p Provider) Snapshot() (application.Snapshot, error) {
	return p.snapshot()
}

func (p Provider) Validate(window application.Window) (bool, error) {
	return p.validate(window)
}

var _ application.Observer = Provider{}
