package wailsadapter

import (
	"testing"

	"github.com/asam-masa/browser-launcher/internal/application/appinfo"
)

func TestAppControllerGetApplicationInfo(t *testing.T) {
	controller := NewAppController(
		appinfo.NewService("Browser Launcher", "dev"),
	)

	got := controller.GetApplicationInfo()

	if got.Name != "Browser Launcher" {
		t.Fatalf("Name = %q, want %q", got.Name, "Browser Launcher")
	}
	if got.Version != "dev" {
		t.Fatalf("Version = %q, want %q", got.Version, "dev")
	}
}
