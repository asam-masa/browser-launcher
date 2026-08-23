//go:build windows

package chromedetection

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	application "github.com/asam-masa/browser-launcher/internal/application/chromedetection"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const appPathsChromeKey = `Software\Microsoft\Windows\CurrentVersion\App Paths\chrome.exe`

type installationScope uint8

const (
	userScope installationScope = iota
	systemScope
)

type windowsSystem interface {
	KnownFolderPath(scope installationScope) (string, error)
	ReadAppPath(scope installationScope) (string, bool, error)
	Stat(path string) (fs.FileInfo, error)
}

type nativeWindowsSystem struct{}

func (Provider) Detect() (application.Installation, error) {
	return detect(nativeWindowsSystem{})
}

func detect(system windowsSystem) (application.Installation, error) {
	allowedPaths := make(map[installationScope]string, 2)
	for _, scope := range []installationScope{userScope, systemScope} {
		folder, err := system.KnownFolderPath(scope)
		if err != nil {
			return application.Installation{}, fmt.Errorf("get %s installation folder: %w", scope, err)
		}
		allowedPaths[scope] = filepath.Join(folder, "Google", "Chrome", "Application", "chrome.exe")
	}

	for _, scope := range []installationScope{userScope, systemScope} {
		path, found, err := system.ReadAppPath(scope)
		if err != nil {
			return application.Installation{}, fmt.Errorf("read %s Chrome App Paths entry: %w", scope, err)
		}
		valid, err := isAllowedExecutable(system, path, allowedPaths)
		if err != nil {
			return application.Installation{}, fmt.Errorf("validate %s Chrome App Paths entry: %w", scope, err)
		}
		if found && valid {
			return application.Installation{ExecutablePath: filepath.Clean(path)}, nil
		}
	}

	for _, scope := range []installationScope{userScope, systemScope} {
		path := allowedPaths[scope]
		valid, err := isAllowedExecutable(system, path, allowedPaths)
		if err != nil {
			return application.Installation{}, fmt.Errorf("validate %s Chrome standard path: %w", scope, err)
		}
		if valid {
			return application.Installation{ExecutablePath: filepath.Clean(path)}, nil
		}
	}

	return application.Installation{}, application.ErrNotFound
}

func isAllowedExecutable(system windowsSystem, path string, allowedPaths map[installationScope]string) (bool, error) {
	if path == "" {
		return false, nil
	}
	cleanPath := filepath.Clean(path)
	allowed := false
	for _, allowedPath := range allowedPaths {
		if strings.EqualFold(cleanPath, filepath.Clean(allowedPath)) {
			allowed = true
			break
		}
	}
	if !allowed {
		return false, nil
	}

	info, err := system.Stat(cleanPath)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return info.Mode().IsRegular(), nil
}

func (nativeWindowsSystem) KnownFolderPath(scope installationScope) (string, error) {
	folderID := windows.FOLDERID_LocalAppData
	if scope == systemScope {
		folderID = windows.FOLDERID_ProgramFiles
	}
	return windows.KnownFolderPath(folderID, 0)
}

func (nativeWindowsSystem) ReadAppPath(scope installationScope) (string, bool, error) {
	root := registry.CURRENT_USER
	if scope == systemScope {
		root = registry.LOCAL_MACHINE
	}

	key, err := registry.OpenKey(root, appPathsChromeKey, registry.QUERY_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	defer key.Close()

	path, _, err := key.GetStringValue("")
	if errors.Is(err, registry.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return path, true, nil
}

func (nativeWindowsSystem) Stat(path string) (fs.FileInfo, error) {
	return os.Stat(path)
}

func (s installationScope) String() string {
	if s == systemScope {
		return "system"
	}
	return "user"
}
