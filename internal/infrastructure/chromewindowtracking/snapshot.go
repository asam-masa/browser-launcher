package chromewindowtracking

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	application "github.com/asam-masa/browser-launcher/internal/application/chromewindowtracking"
)

const chromeWindowClass = "Chrome_WidgetWin_1"

var errWindowUnavailable = errors.New("window is no longer available")

var (
	errProviderNotInitialized = errors.New("provider is not initialized")
	errExecutablePathRequired = errors.New("Chrome executable path is required")
)

type windowSystem interface {
	Enumerate() ([]uint64, error)
	Inspect(handle uint64) (windowMetadata, error)
	Revalidate(windowMetadata) (bool, error)
}

type windowMetadata struct {
	handle           uint64
	processID        uint32
	processStartTime uint64
	executablePath   string
	className        string
	visible          bool
}

func (p Provider) snapshot() (application.Snapshot, error) {
	if p.system == nil {
		return nil, fmt.Errorf("%w: validate provider: %w", ErrSnapshotFailed, errProviderNotInitialized)
	}
	if p.executablePath == "" {
		return nil, fmt.Errorf("%w: validate provider: %w", ErrSnapshotFailed, errExecutablePathRequired)
	}

	handles, err := p.system.Enumerate()
	if err != nil {
		return nil, fmt.Errorf("%w: enumerate windows: %w", ErrSnapshotFailed, err)
	}

	snapshot := make(application.Snapshot, 0, len(handles))
	for _, handle := range handles {
		metadata, err := p.system.Inspect(handle)
		if errors.Is(err, errWindowUnavailable) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("%w: inspect window: %w", ErrSnapshotFailed, err)
		}
		if !p.matches(metadata) {
			continue
		}

		valid, err := p.system.Revalidate(metadata)
		if errors.Is(err, errWindowUnavailable) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("%w: revalidate window: %w", ErrSnapshotFailed, err)
		}
		if !valid {
			continue
		}

		snapshot = append(snapshot, application.Window{
			Handle:           metadata.handle,
			ProcessID:        metadata.processID,
			ProcessStartTime: metadata.processStartTime,
		})
	}

	sort.Slice(snapshot, func(i, j int) bool {
		if snapshot[i].Handle != snapshot[j].Handle {
			return snapshot[i].Handle < snapshot[j].Handle
		}
		if snapshot[i].ProcessID != snapshot[j].ProcessID {
			return snapshot[i].ProcessID < snapshot[j].ProcessID
		}
		return snapshot[i].ProcessStartTime < snapshot[j].ProcessStartTime
	})
	return snapshot, nil
}

func (p Provider) validate(window application.Window) (bool, error) {
	if p.system == nil {
		return false, fmt.Errorf("%w: validate provider: %w", ErrValidationFailed, errProviderNotInitialized)
	}
	if p.executablePath == "" {
		return false, fmt.Errorf("%w: validate provider: %w", ErrValidationFailed, errExecutablePathRequired)
	}
	if window.Handle == 0 || window.ProcessID == 0 || window.ProcessStartTime == 0 {
		return false, nil
	}

	metadata, err := p.system.Inspect(window.Handle)
	if errors.Is(err, errWindowUnavailable) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("%w: inspect window: %w", ErrValidationFailed, err)
	}
	if !p.matches(metadata) ||
		metadata.processID != window.ProcessID ||
		metadata.processStartTime != window.ProcessStartTime {
		return false, nil
	}

	valid, err := p.system.Revalidate(metadata)
	if errors.Is(err, errWindowUnavailable) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("%w: revalidate window: %w", ErrValidationFailed, err)
	}
	return valid, nil
}

func (p Provider) matches(metadata windowMetadata) bool {
	return metadata.visible &&
		metadata.className == chromeWindowClass &&
		metadata.handle != 0 &&
		metadata.processID != 0 &&
		metadata.processStartTime != 0 &&
		sameExecutablePath(metadata.executablePath, p.executablePath)
}

func normalizeExecutablePath(path string) string {
	if path == "" {
		return ""
	}
	return filepath.Clean(path)
}

func sameExecutablePath(left, right string) bool {
	if left == "" || right == "" {
		return false
	}
	return strings.EqualFold(normalizeExecutablePath(left), normalizeExecutablePath(right))
}
