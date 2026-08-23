//go:build !windows

package chromelaunch

import (
	"errors"
	"testing"

	application "github.com/asam-masa/browser-launcher/internal/application/chromelaunch"
)

func TestProviderStartReturnsUnsupportedPlatform(t *testing.T) {
	t.Parallel()

	_, err := NewProvider().Start(application.Command{})
	if !errors.Is(err, application.ErrUnsupportedPlatform) {
		t.Fatalf("Start() error = %v, want %v", err, application.ErrUnsupportedPlatform)
	}
}
