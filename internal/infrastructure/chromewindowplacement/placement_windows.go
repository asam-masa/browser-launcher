//go:build windows

package chromewindowplacement

import (
	"errors"
	"fmt"
	"runtime"
	"unsafe"

	placement "github.com/asam-masa/browser-launcher/internal/application/chromewindowplacement"
	"golang.org/x/sys/windows"
)

const (
	dpiAwarenessContextPerMonitorV2 = ^uintptr(3)
	monitorDefaultToNull            = 0
	setWindowPosNoSize              = 0x0001
	setWindowPosNoZOrder            = 0x0004
	setWindowPosNoActivate          = 0x0010
)

var (
	user32                           = windows.NewLazySystemDLL("user32.dll")
	procGetDpiForWindow              = user32.NewProc("GetDpiForWindow")
	procGetMonitorInfoW              = user32.NewProc("GetMonitorInfoW")
	procGetWindowRect                = user32.NewProc("GetWindowRect")
	procMonitorFromPoint             = user32.NewProc("MonitorFromPoint")
	procMonitorFromWindow            = user32.NewProc("MonitorFromWindow")
	procSetThreadDpiAwarenessContext = user32.NewProc("SetThreadDpiAwarenessContext")
	procSetWindowPos                 = user32.NewProc("SetWindowPos")
)

type nativeWindowSystem struct{}

type nativeRect struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

type nativeMonitorInfo struct {
	Size    uint32
	Monitor nativeRect
	Work    nativeRect
	Flags   uint32
}

func newWindowSystem() windowSystem {
	return nativeWindowSystem{}
}

func (nativeWindowSystem) BeginDPIAware() (func() error, error) {
	runtime.LockOSThread()
	previous, _, callErr := procSetThreadDpiAwarenessContext.Call(dpiAwarenessContextPerMonitorV2)
	if previous == 0 {
		runtime.UnlockOSThread()
		return nil, windowsCallError("SetThreadDpiAwarenessContext", callErr)
	}
	return func() error {
		defer runtime.UnlockOSThread()
		restored, _, restoreErr := procSetThreadDpiAwarenessContext.Call(previous)
		if restored == 0 {
			return windowsCallError("restore SetThreadDpiAwarenessContext", restoreErr)
		}
		return nil
	}, nil
}

func (nativeWindowSystem) TargetMonitor(bounds placement.Rectangle) (targetMonitor, error) {
	if err := validateWindowsRectangle(bounds); err != nil {
		return targetMonitor{}, err
	}
	point := packPoint(int32(bounds.X), int32(bounds.Y))
	monitor, _, callErr := procMonitorFromPoint.Call(point, monitorDefaultToNull)
	if monitor == 0 {
		return targetMonitor{}, windowsCallError("MonitorFromPoint", callErr)
	}
	info := nativeMonitorInfo{Size: uint32(unsafe.Sizeof(nativeMonitorInfo{}))}
	succeeded, _, callErr := procGetMonitorInfoW.Call(monitor, uintptr(unsafe.Pointer(&info)))
	if succeeded == 0 {
		return targetMonitor{}, windowsCallError("GetMonitorInfoW", callErr)
	}
	return targetMonitor{handle: monitor, workX: int(info.Work.Left), workY: int(info.Work.Top)}, nil
}

func (nativeWindowSystem) Measure(handle uint64) (measurement, error) {
	hwnd := windows.HWND(handle)
	var bounds nativeRect
	succeeded, _, callErr := procGetWindowRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&bounds)))
	if succeeded == 0 {
		return measurement{}, windowsCallError("GetWindowRect", callErr)
	}
	dpi, _, callErr := procGetDpiForWindow.Call(uintptr(hwnd))
	if dpi == 0 {
		return measurement{}, windowsCallError("GetDpiForWindow", callErr)
	}
	monitor, _, callErr := procMonitorFromWindow.Call(uintptr(hwnd), monitorDefaultToNull)
	if monitor == 0 {
		return measurement{}, windowsCallError("MonitorFromWindow", callErr)
	}
	info := nativeMonitorInfo{Size: uint32(unsafe.Sizeof(nativeMonitorInfo{}))}
	succeeded, _, callErr = procGetMonitorInfoW.Call(monitor, uintptr(unsafe.Pointer(&info)))
	if succeeded == 0 {
		return measurement{}, windowsCallError("GetMonitorInfoW", callErr)
	}
	return measurement{
		bounds: placement.Rectangle{
			X:      int(bounds.Left),
			Y:      int(bounds.Top),
			Width:  int(bounds.Right - bounds.Left),
			Height: int(bounds.Bottom - bounds.Top),
		},
		dpi:         uint32(dpi),
		monitor:     monitor,
		monitorLeft: int(info.Monitor.Left),
		monitorTop:  int(info.Monitor.Top),
		workLeft:    int(info.Work.Left),
		workTop:     int(info.Work.Top),
		workWidth:   int(info.Work.Right - info.Work.Left),
		workHeight:  int(info.Work.Bottom - info.Work.Top),
	}, nil
}

func (nativeWindowSystem) Move(handle uint64, bounds placement.Rectangle, resize bool) error {
	if err := validateWindowsRectangle(bounds); err != nil {
		return err
	}
	flags := uintptr(setWindowPosNoZOrder | setWindowPosNoActivate)
	if !resize {
		flags |= setWindowPosNoSize
	}
	succeeded, _, callErr := procSetWindowPos.Call(
		uintptr(windows.HWND(handle)),
		0,
		uintptr(bounds.X),
		uintptr(bounds.Y),
		uintptr(bounds.Width),
		uintptr(bounds.Height),
		flags,
	)
	if succeeded == 0 {
		return windowsCallError("SetWindowPos", callErr)
	}
	return nil
}

func packPoint(x, y int32) uintptr {
	return uintptr(uint64(uint32(x)) | uint64(uint32(y))<<32)
}

func windowsCallError(operation string, callErr error) error {
	if callErr == nil || errors.Is(callErr, windows.ERROR_SUCCESS) {
		return fmt.Errorf("%s failed", operation)
	}
	return fmt.Errorf("%s failed: %w", operation, callErr)
}
