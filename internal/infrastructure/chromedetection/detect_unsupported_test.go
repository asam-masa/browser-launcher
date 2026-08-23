//go:build !windows

package chromedetection

import (
	"errors"
	"testing"

	application "github.com/asam-masa/browser-launcher/internal/application/chromedetection"
)

func TestProviderDetectReturnsUnsupportedPlatform(t *testing.T) {
	t.Parallel()

	_, err := NewProvider().Detect()
	if !errors.Is(err, application.ErrUnsupportedPlatform) {
		t.Fatalf("Detect() error = %v, want %v", err, application.ErrUnsupportedPlatform)
	}
}
