//go:build !windows

package chromewindowtracking

type unsupportedWindowSystem struct{}

func newWindowSystem() windowSystem {
	return unsupportedWindowSystem{}
}

func (unsupportedWindowSystem) Enumerate() ([]uint64, error) {
	return nil, ErrUnsupportedPlatform
}

func (unsupportedWindowSystem) Inspect(uint64) (windowMetadata, error) {
	return windowMetadata{}, ErrUnsupportedPlatform
}

func (unsupportedWindowSystem) Revalidate(windowMetadata) (bool, error) {
	return false, ErrUnsupportedPlatform
}
