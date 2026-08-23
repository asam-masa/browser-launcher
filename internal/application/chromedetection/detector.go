package chromedetection

import "errors"

var (
	ErrNotFound            = errors.New("chrome installation not found")
	ErrUnsupportedPlatform = errors.New("chrome detection is only supported on Windows")
)

type Installation struct {
	ExecutablePath string
}
