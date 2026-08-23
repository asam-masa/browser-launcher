//go:build !windows

package chromewindowtracking

import (
	"errors"
	"testing"
)

func TestProviderSnapshotReturnsUnsupportedPlatform(t *testing.T) {
	t.Parallel()

	_, err := NewProvider("/chrome").Snapshot()
	if !errors.Is(err, ErrUnsupportedPlatform) {
		t.Fatalf("Snapshot() error = %v, want ErrUnsupportedPlatform", err)
	}
	if !errors.Is(err, ErrSnapshotFailed) {
		t.Fatalf("Snapshot() error = %v, want ErrSnapshotFailed", err)
	}
}
