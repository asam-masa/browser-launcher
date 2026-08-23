//go:build !windows

package display

import (
	"errors"
	"testing"
)

func TestProviderGetPrimaryWorkAreaReturnsUnsupportedPlatform(t *testing.T) {
	t.Parallel()

	_, err := NewProvider().GetPrimaryWorkArea()
	if !errors.Is(err, ErrUnsupportedPlatform) {
		t.Fatalf("GetPrimaryWorkArea() error = %v, want %v", err, ErrUnsupportedPlatform)
	}
}
