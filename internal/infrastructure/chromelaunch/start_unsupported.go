//go:build !windows

package chromelaunch

import application "github.com/asam-masa/browser-launcher/internal/application/chromelaunch"

func (Provider) Start(application.Command) (application.Result, error) {
	return application.Result{}, application.ErrUnsupportedPlatform
}
