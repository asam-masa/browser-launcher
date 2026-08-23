//go:build windows

package chromewindowtracking

import (
	"errors"
	"fmt"
	"syscall"

	"golang.org/x/sys/windows"
)

const processImagePathBufferSize = 32768

type nativeWindowSystem struct{}

func newWindowSystem() windowSystem {
	return nativeWindowSystem{}
}

func (nativeWindowSystem) Enumerate() ([]uint64, error) {
	handles := make([]uint64, 0)
	callback := syscall.NewCallback(func(handle windows.HWND, _ uintptr) uintptr {
		handles = append(handles, uint64(handle))
		return 1
	})
	if err := windows.EnumWindows(callback, nil); err != nil {
		return nil, err
	}
	return handles, nil
}

func (nativeWindowSystem) Inspect(handle uint64) (windowMetadata, error) {
	hwnd := windows.HWND(handle)
	if !windows.IsWindow(hwnd) {
		return windowMetadata{}, errWindowUnavailable
	}

	className, err := windowClassName(hwnd)
	if err != nil {
		return windowMetadata{}, unavailableWindowError(err)
	}
	if className != chromeWindowClass || !windows.IsWindowVisible(hwnd) {
		return windowMetadata{handle: handle, className: className}, nil
	}

	processID, executablePath, processStartTime, err := processIdentity(hwnd)
	if err != nil {
		return windowMetadata{}, unavailableWindowError(err)
	}
	return windowMetadata{
		handle:           handle,
		processID:        processID,
		processStartTime: processStartTime,
		executablePath:   executablePath,
		className:        className,
		visible:          true,
	}, nil
}

func (nativeWindowSystem) Revalidate(metadata windowMetadata) (bool, error) {
	hwnd := windows.HWND(metadata.handle)
	if !windows.IsWindow(hwnd) || !windows.IsWindowVisible(hwnd) {
		return false, nil
	}
	className, err := windowClassName(hwnd)
	if err != nil {
		return false, unavailableWindowError(err)
	}
	if className != metadata.className {
		return false, nil
	}
	processID, executablePath, processStartTime, err := processIdentity(hwnd)
	if err != nil {
		return false, unavailableWindowError(err)
	}
	return processID == metadata.processID &&
		processStartTime == metadata.processStartTime &&
		sameExecutablePath(executablePath, metadata.executablePath), nil
}

func windowClassName(hwnd windows.HWND) (string, error) {
	buffer := make([]uint16, 256)
	copied, err := windows.GetClassName(hwnd, &buffer[0], int32(len(buffer)))
	if err != nil {
		return "", fmt.Errorf("get window class: %w", err)
	}
	return windows.UTF16ToString(buffer[:copied]), nil
}

func processIdentity(hwnd windows.HWND) (uint32, string, uint64, error) {
	var processID uint32
	if _, err := windows.GetWindowThreadProcessId(hwnd, &processID); err != nil {
		return 0, "", 0, fmt.Errorf("get window process: %w", err)
	}
	if processID == 0 {
		return 0, "", 0, errWindowUnavailable
	}

	process, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, processID)
	if err != nil {
		return 0, "", 0, fmt.Errorf("open window process: %w", err)
	}
	defer windows.CloseHandle(process)

	buffer := make([]uint16, processImagePathBufferSize)
	size := uint32(len(buffer))
	if err := windows.QueryFullProcessImageName(process, 0, &buffer[0], &size); err != nil {
		return 0, "", 0, fmt.Errorf("query window process image: %w", err)
	}

	var creation windows.Filetime
	var exit windows.Filetime
	var kernel windows.Filetime
	var user windows.Filetime
	if err := windows.GetProcessTimes(process, &creation, &exit, &kernel, &user); err != nil {
		return 0, "", 0, fmt.Errorf("get window process times: %w", err)
	}
	startTime := uint64(creation.HighDateTime)<<32 | uint64(creation.LowDateTime)
	if startTime == 0 {
		return 0, "", 0, errWindowUnavailable
	}

	return processID, windows.UTF16ToString(buffer[:size]), startTime, nil
}

func unavailableWindowError(err error) error {
	if errors.Is(err, windows.ERROR_INVALID_WINDOW_HANDLE) ||
		errors.Is(err, windows.ERROR_INVALID_PARAMETER) ||
		errors.Is(err, windows.ERROR_NOT_FOUND) {
		return errors.Join(errWindowUnavailable, err)
	}
	return err
}
