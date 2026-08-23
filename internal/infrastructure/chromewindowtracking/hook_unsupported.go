//go:build !windows

package chromewindowtracking

func startChangeHook() (changeHook, error) {
	return nil, ErrUnsupportedPlatform
}
