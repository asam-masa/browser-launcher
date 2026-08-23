//go:build windows

package chromewindowtracking

import (
	"os"
	"testing"
)

func TestSameExecutablePathUsesWindowsPathRules(t *testing.T) {
	t.Parallel()

	if !sameExecutablePath(`C:\Program Files\Google\Chrome\Application\chrome.exe`, `c:\program files\google\chrome\application\chrome.exe`) {
		t.Fatal("sameExecutablePath() = false, want true")
	}
	if sameExecutablePath(`C:\Chrome\chrome.exe`, `C:\Chrome Beta\chrome.exe`) {
		t.Fatal("sameExecutablePath() = true, want false")
	}
}

func TestProviderSnapshotOnWindows(t *testing.T) {
	executablePath := os.Getenv("BROWSER_LAUNCHER_CHROME_EXECUTABLE")
	if executablePath == "" {
		t.Skip("BROWSER_LAUNCHER_CHROME_EXECUTABLE is not set")
	}

	snapshot, err := NewProvider(executablePath).Snapshot()
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	for _, window := range snapshot {
		if window.Handle == 0 || window.ProcessID == 0 || window.ProcessStartTime == 0 {
			t.Fatalf("Snapshot() returned an invalid identity: %#v", window)
		}
	}
	t.Logf("Snapshot() returned %d Chrome window(s)", len(snapshot))
}
