package config

import "testing"

func TestFromEnvLoadsDesktopAccessKey(t *testing.T) {
	t.Setenv("DESKTOP_ACCESS_KEY", "desktop-restricted-key")

	cfg := FromEnv()

	if cfg.DesktopAccessKey != "desktop-restricted-key" {
		t.Fatalf("unexpected desktop access key: %q", cfg.DesktopAccessKey)
	}
}
