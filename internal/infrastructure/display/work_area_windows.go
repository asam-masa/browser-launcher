//go:build windows

package display

import (
	"errors"
	"fmt"
	"runtime"
	"unsafe"

	application "github.com/asam-masa/browser-launcher/internal/application/launchcondition"
	"golang.org/x/sys/windows"
)

const (
	monitorDefaultToPrimary         = 1
	dpiAwarenessContextPerMonitorV2 = ^uintptr(3)
	monitorDPITypeEffective         = 0
	userDefaultScreenDPI            = 96
)

var (
	user32                    = windows.NewLazySystemDLL("user32.dll")
	shcore                    = windows.NewLazySystemDLL("shcore.dll")
	procMonitorFromPoint      = user32.NewProc("MonitorFromPoint")
	procGetMonitorInfoW       = user32.NewProc("GetMonitorInfoW")
	procGetDpiForMonitor      = shcore.NewProc("GetDpiForMonitor")
	procGetThreadDpiAwareness = user32.NewProc("GetThreadDpiAwarenessContext")
	procSetThreadDpiAwareness = user32.NewProc("SetThreadDpiAwarenessContext")
)

type rect struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

type monitorInfo struct {
	Size    uint32
	Monitor rect
	Work    rect
	Flags   uint32
}

func (Provider) GetPrimaryWorkArea() (result application.PrimaryWorkArea, resultErr error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	previousContext, _, callErr := procSetThreadDpiAwareness.Call(dpiAwarenessContextPerMonitorV2)
	if previousContext == 0 {
		return application.PrimaryWorkArea{}, windowsCallError("SetThreadDpiAwarenessContext", callErr)
	}
	defer func() {
		restoredContext, _, restoreErr := procSetThreadDpiAwareness.Call(previousContext)
		if restoredContext == 0 {
			result = application.PrimaryWorkArea{}
			resultErr = errors.Join(
				resultErr,
				windowsCallError("restore SetThreadDpiAwarenessContext", restoreErr),
			)
		}
	}()

	monitor, _, callErr := procMonitorFromPoint.Call(0, monitorDefaultToPrimary)
	if monitor == 0 {
		return application.PrimaryWorkArea{}, windowsCallError("MonitorFromPoint", callErr)
	}

	info := monitorInfo{Size: uint32(unsafe.Sizeof(monitorInfo{}))}
	succeeded, _, callErr := procGetMonitorInfoW.Call(monitor, uintptr(unsafe.Pointer(&info)))
	if succeeded == 0 {
		return application.PrimaryWorkArea{}, windowsCallError("GetMonitorInfoW", callErr)
	}

	var dpiX uint32
	var dpiY uint32
	hresult, _, _ := procGetDpiForMonitor.Call(
		monitor,
		monitorDPITypeEffective,
		uintptr(unsafe.Pointer(&dpiX)),
		uintptr(unsafe.Pointer(&dpiY)),
	)
	if hresult != 0 {
		return application.PrimaryWorkArea{}, fmt.Errorf("GetDpiForMonitor failed: HRESULT 0x%08X", uint32(hresult))
	}
	if dpiX < userDefaultScreenDPI || dpiX != dpiY {
		return application.PrimaryWorkArea{}, fmt.Errorf(
			"GetDpiForMonitor returned unsupported DPI: x=%d, y=%d",
			dpiX,
			dpiY,
		)
	}

	return application.PrimaryWorkArea{
		MonitorLeft: int(info.Monitor.Left),
		MonitorTop:  int(info.Monitor.Top),
		WorkLeft:    int(info.Work.Left),
		WorkTop:     int(info.Work.Top),
		WorkWidth:   int(info.Work.Right - info.Work.Left),
		WorkHeight:  int(info.Work.Bottom - info.Work.Top),
		DPI:         int(dpiX),
	}, nil
}

func windowsCallError(operation string, callErr error) error {
	if callErr == nil || errors.Is(callErr, windows.ERROR_SUCCESS) {
		return fmt.Errorf("%s failed", operation)
	}
	return fmt.Errorf("%s failed: %w", operation, callErr)
}
