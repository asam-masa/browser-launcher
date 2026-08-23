package appinfo

import "testing"

func TestServiceGet(t *testing.T) {
	service := NewService("Browser Launcher", "dev")

	got := service.Get()

	if got.Name != "Browser Launcher" {
		t.Fatalf("Name = %q, want %q", got.Name, "Browser Launcher")
	}
	if got.Version != "dev" {
		t.Fatalf("Version = %q, want %q", got.Version, "dev")
	}
}
