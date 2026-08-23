//go:build !windows

package chromewindowplacement

import (
	"context"
	"errors"
	"testing"

	placement "github.com/asam-masa/browser-launcher/internal/application/chromewindowplacement"
)

func TestProviderPlaceReturnsUnsupportedPlatform(t *testing.T) {
	t.Parallel()

	provider := NewProvider(&stubIdentityValidator{valid: true})
	requested := placement.Rectangle{X: 1, Y: 2, Width: 3, Height: 4}
	_, err := provider.Place(context.Background(), placement.Request{
		Window:        testWindow(),
		InitialTarget: requested,
	}, fixedBoundsResolver(requested))
	if !errors.Is(err, ErrUnsupportedPlatform) {
		t.Fatalf("Place() error = %v, want ErrUnsupportedPlatform", err)
	}
}
