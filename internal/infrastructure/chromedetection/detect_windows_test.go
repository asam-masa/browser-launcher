//go:build windows

package chromedetection

import (
	"errors"
	"io/fs"
	"os"
	"testing"
	"time"

	application "github.com/asam-masa/browser-launcher/internal/application/chromedetection"
)

const (
	userFolder   = `C:\Users\tester\AppData\Local`
	systemFolder = `C:\Program Files`
	userChrome   = userFolder + `\Google\Chrome\Application\chrome.exe`
	systemChrome = systemFolder + `\Google\Chrome\Application\chrome.exe`
)

type fakeWindowsSystem struct {
	knownFolders map[installationScope]string
	appPaths     map[installationScope]string
	files        map[string]fs.FileInfo
	appPathError error
	statError    error
}

func TestProviderDetectIntegration(t *testing.T) {
	if os.Getenv("BROWSER_LAUNCHER_RUN_WINDOWS_INTEGRATION") != "1" {
		t.Skip("set BROWSER_LAUNCHER_RUN_WINDOWS_INTEGRATION=1 to test the installed Chrome")
	}

	installation, err := NewProvider().Detect()
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if installation.ExecutablePath == "" {
		t.Fatal("ExecutablePath is empty")
	}
}

func TestDetectPrefersUserAppPath(t *testing.T) {
	t.Parallel()

	system := validFakeSystem()
	system.appPaths[userScope] = userChrome
	system.appPaths[systemScope] = systemChrome

	installation, err := detect(system)
	if err != nil {
		t.Fatalf("detect() error = %v", err)
	}
	if installation.ExecutablePath != userChrome {
		t.Fatalf("ExecutablePath = %q, want %q", installation.ExecutablePath, userChrome)
	}
}

func TestDetectUsesSystemAppPathWhenUserAppPathIsMissing(t *testing.T) {
	t.Parallel()

	system := validFakeSystem()
	system.appPaths[systemScope] = systemChrome

	installation, err := detect(system)
	if err != nil {
		t.Fatalf("detect() error = %v", err)
	}
	if installation.ExecutablePath != systemChrome {
		t.Fatalf("ExecutablePath = %q, want %q", installation.ExecutablePath, systemChrome)
	}
}

func TestDetectFallsBackToSystemStandardPath(t *testing.T) {
	t.Parallel()

	system := validFakeSystem()
	delete(system.files, userChrome)

	installation, err := detect(system)
	if err != nil {
		t.Fatalf("detect() error = %v", err)
	}
	if installation.ExecutablePath != systemChrome {
		t.Fatalf("ExecutablePath = %q, want %q", installation.ExecutablePath, systemChrome)
	}
}

func TestDetectIgnoresAppPathOutsideAllowedLocations(t *testing.T) {
	t.Parallel()

	system := validFakeSystem()
	system.appPaths[userScope] = `C:\Tools\Chrome\chrome.exe`

	installation, err := detect(system)
	if err != nil {
		t.Fatalf("detect() error = %v", err)
	}
	if installation.ExecutablePath != userChrome {
		t.Fatalf("ExecutablePath = %q, want fallback %q", installation.ExecutablePath, userChrome)
	}
}

func TestDetectReturnsNotFoundWhenAllowedFilesDoNotExist(t *testing.T) {
	t.Parallel()

	system := validFakeSystem()
	system.files = nil

	_, err := detect(system)
	if !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("detect() error = %v, want %v", err, application.ErrNotFound)
	}
}

func TestDetectReturnsRegistryFailure(t *testing.T) {
	t.Parallel()

	system := validFakeSystem()
	system.appPathError = errors.New("access denied")

	_, err := detect(system)
	if err == nil || errors.Is(err, application.ErrNotFound) {
		t.Fatalf("detect() error = %v, want infrastructure failure", err)
	}
}

func TestDetectReturnsFileInspectionFailure(t *testing.T) {
	t.Parallel()

	system := validFakeSystem()
	system.statError = errors.New("access denied")

	_, err := detect(system)
	if err == nil || errors.Is(err, application.ErrNotFound) {
		t.Fatalf("detect() error = %v, want infrastructure failure", err)
	}
}

func validFakeSystem() fakeWindowsSystem {
	return fakeWindowsSystem{
		knownFolders: map[installationScope]string{
			userScope:   userFolder,
			systemScope: systemFolder,
		},
		appPaths: map[installationScope]string{},
		files: map[string]fs.FileInfo{
			userChrome:   fakeFileInfo{name: "chrome.exe"},
			systemChrome: fakeFileInfo{name: "chrome.exe"},
		},
	}
}

func (s fakeWindowsSystem) KnownFolderPath(scope installationScope) (string, error) {
	return s.knownFolders[scope], nil
}

func (s fakeWindowsSystem) ReadAppPath(scope installationScope) (string, bool, error) {
	if s.appPathError != nil {
		return "", false, s.appPathError
	}
	path, found := s.appPaths[scope]
	return path, found, nil
}

func (s fakeWindowsSystem) Stat(path string) (fs.FileInfo, error) {
	if s.statError != nil {
		return nil, s.statError
	}
	info, found := s.files[path]
	if !found {
		return nil, fs.ErrNotExist
	}
	return info, nil
}

type fakeFileInfo struct {
	name string
}

func (f fakeFileInfo) Name() string     { return f.name }
func (fakeFileInfo) Size() int64        { return 0 }
func (fakeFileInfo) Mode() fs.FileMode  { return 0 }
func (fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (fakeFileInfo) IsDir() bool        { return false }
func (fakeFileInfo) Sys() any           { return nil }

var _ fs.FileInfo = fakeFileInfo{}
