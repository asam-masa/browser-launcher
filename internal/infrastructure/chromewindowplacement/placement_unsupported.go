//go:build !windows

package chromewindowplacement

import (
	placement "github.com/asam-masa/browser-launcher/internal/application/chromewindowplacement"
)

type unsupportedWindowSystem struct{}

func newWindowSystem() windowSystem {
	return unsupportedWindowSystem{}
}

func (unsupportedWindowSystem) BeginDPIAware() (func() error, error) {
	return nil, ErrUnsupportedPlatform
}

func (unsupportedWindowSystem) TargetMonitor(placement.Rectangle) (targetMonitor, error) {
	return targetMonitor{}, ErrUnsupportedPlatform
}

func (unsupportedWindowSystem) Measure(uint64) (measurement, error) {
	return measurement{}, ErrUnsupportedPlatform
}

func (unsupportedWindowSystem) Move(uint64, placement.Rectangle, bool) error {
	return ErrUnsupportedPlatform
}
