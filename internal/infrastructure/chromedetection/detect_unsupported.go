//go:build !windows

package chromedetection

import application "github.com/asam-masa/browser-launcher/internal/application/chromedetection"

func (Provider) Detect() (application.Installation, error) {
	return application.Installation{}, application.ErrUnsupportedPlatform
}
