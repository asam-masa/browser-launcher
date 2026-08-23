//go:build !windows

package chromewindowtracking

import (
	"errors"
	"testing"
)

func TestStartChangeHookReturnsUnsupportedPlatform(t *testing.T) {
	t.Parallel()

	_, err := startChangeHook()
	if !errors.Is(err, ErrUnsupportedPlatform) {
		t.Fatalf("startChangeHook() error = %v, want ErrUnsupportedPlatform", err)
	}
}
