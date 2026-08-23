//go:build windows

package display

import (
	"runtime"
	"testing"
)

func TestProviderGetPrimaryWorkArea(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	beforeContext, _, _ := procGetThreadDpiAwareness.Call()
	if beforeContext == 0 {
		t.Fatal("GetThreadDpiAwarenessContext() returned zero before provider call")
	}

	workArea, err := NewProvider().GetPrimaryWorkArea()
	if err != nil {
		t.Fatalf("GetPrimaryWorkArea() error = %v", err)
	}
	if workArea.WorkWidth < 1 || workArea.WorkHeight < 1 {
		t.Fatalf("work area = %+v, want positive dimensions", workArea)
	}
	if workArea.DPI < userDefaultScreenDPI {
		t.Fatalf("DPI = %d, want at least %d", workArea.DPI, userDefaultScreenDPI)
	}
	if workArea.WorkLeft < workArea.MonitorLeft || workArea.WorkTop < workArea.MonitorTop {
		t.Fatalf("work area = %+v, want origin inside monitor", workArea)
	}
	afterContext, _, _ := procGetThreadDpiAwareness.Call()
	if afterContext == 0 {
		t.Fatal("GetThreadDpiAwarenessContext() returned zero after provider call")
	}
	if afterContext != beforeContext {
		t.Fatalf("DPI awareness context = %d after provider call, want %d", afterContext, beforeContext)
	}

	t.Logf("primary work area: %+v", workArea)
}
