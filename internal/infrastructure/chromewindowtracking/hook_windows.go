//go:build windows

package chromewindowtracking

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	eventObjectCreate    = 0x8000
	eventObjectDestroy   = 0x8001
	eventObjectShow      = 0x8002
	winEventOutOfContext = 0
	windowMessageQuit    = 0x0012
	peekMessageNoRemove  = 0
)

var (
	hookUser32             = windows.NewLazySystemDLL("user32.dll")
	procSetWinEventHook    = hookUser32.NewProc("SetWinEventHook")
	procUnhookWinEvent     = hookUser32.NewProc("UnhookWinEvent")
	procPeekMessageW       = hookUser32.NewProc("PeekMessageW")
	procGetMessageW        = hookUser32.NewProc("GetMessageW")
	procTranslateMessage   = hookUser32.NewProc("TranslateMessage")
	procDispatchMessageW   = hookUser32.NewProc("DispatchMessageW")
	procPostThreadMessageW = hookUser32.NewProc("PostThreadMessageW")
	winEventHooks          sync.Map
	winEventCallback       = syscall.NewCallback(dispatchWinEvent)
)

type windowsMessage struct {
	Window  uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Point   struct{ X, Y int32 }
	Private uint32
}

type windowsChangeHook struct {
	notifications chan struct{}
	failures      chan error
	done          chan struct{}
	result        chan error
	threadID      uint32
	close         sync.Once
	closeErr      error
}

func startChangeHook() (changeHook, error) {
	hook := &windowsChangeHook{
		notifications: make(chan struct{}, 1),
		failures:      make(chan error, 1),
		done:          make(chan struct{}),
		result:        make(chan error, 1),
	}
	ready := make(chan error, 1)
	go hook.run(ready)
	if err := <-ready; err != nil {
		return nil, err
	}
	return hook, nil
}

func (h *windowsChangeHook) Notifications() <-chan struct{} { return h.notifications }
func (h *windowsChangeHook) Failures() <-chan error         { return h.failures }

func (h *windowsChangeHook) Close() error {
	h.close.Do(func() {
		select {
		case <-h.done:
		default:
			if err := postThreadQuit(h.threadID); err != nil {
				h.closeErr = err
				return
			}
			<-h.done
		}
		h.closeErr = errors.Join(h.closeErr, <-h.result)
	})
	return h.closeErr
}

func (h *windowsChangeHook) run(ready chan<- error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(h.done)

	h.threadID = windows.GetCurrentThreadId()
	var message windowsMessage
	procPeekMessageW.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0, peekMessageNoRemove)

	hookHandle, _, callErr := procSetWinEventHook.Call(
		eventObjectCreate,
		eventObjectShow,
		0,
		winEventCallback,
		0,
		0,
		winEventOutOfContext,
	)
	if hookHandle == 0 {
		ready <- windowsHookCallError("SetWinEventHook", callErr)
		return
	}
	winEventHooks.Store(hookHandle, h)
	ready <- nil

	loopErr := runWindowsMessageLoop(&message)
	if loopErr != nil {
		select {
		case h.failures <- loopErr:
		default:
		}
	}
	unhookErr := unhookWinEvent(hookHandle)
	winEventHooks.Delete(hookHandle)
	h.result <- errors.Join(loopErr, unhookErr)
}

func runWindowsMessageLoop(message *windowsMessage) error {
	for {
		result, _, callErr := procGetMessageW.Call(uintptr(unsafe.Pointer(message)), 0, 0, 0)
		switch int32(result) {
		case -1:
			return windowsHookCallError("GetMessageW", callErr)
		case 0:
			return nil
		default:
			procTranslateMessage.Call(uintptr(unsafe.Pointer(message)))
			procDispatchMessageW.Call(uintptr(unsafe.Pointer(message)))
		}
	}
}

func postThreadQuit(threadID uint32) error {
	succeeded, _, callErr := procPostThreadMessageW.Call(uintptr(threadID), windowMessageQuit, 0, 0)
	if succeeded == 0 {
		return windowsHookCallError("PostThreadMessageW", callErr)
	}
	return nil
}

func unhookWinEvent(hookHandle uintptr) error {
	succeeded, _, callErr := procUnhookWinEvent.Call(hookHandle)
	if succeeded == 0 {
		return windowsHookCallError("UnhookWinEvent", callErr)
	}
	return nil
}

func dispatchWinEvent(hookHandle uintptr, event uint32, _ uintptr, _ int32, _ int32, _ uint32, _ uint32) uintptr {
	if event < eventObjectCreate || event > eventObjectShow {
		return 0
	}
	value, ok := winEventHooks.Load(hookHandle)
	if !ok {
		return 0
	}
	hook := value.(*windowsChangeHook)
	select {
	case hook.notifications <- struct{}{}:
	default:
	}
	return 0
}

func windowsHookCallError(operation string, callErr error) error {
	if callErr == nil || errors.Is(callErr, windows.ERROR_SUCCESS) {
		return fmt.Errorf("%s failed", operation)
	}
	return fmt.Errorf("%s failed: %w", operation, callErr)
}
