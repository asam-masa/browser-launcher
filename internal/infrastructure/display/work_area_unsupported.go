//go:build !windows

package display

import (
	"errors"

	application "github.com/asam-masa/browser-launcher/internal/application/launchcondition"
)

var ErrUnsupportedPlatform = errors.New("primary work area is only supported on Windows")

func (Provider) GetPrimaryWorkArea() (application.PrimaryWorkArea, error) {
	return application.PrimaryWorkArea{}, ErrUnsupportedPlatform
}
