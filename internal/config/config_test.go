package config

import "testing"

func TestLoadRequiresSessionSecretWhenFallbackDisabled(t *testing.T) {
	t.Setenv("ALLOW_DEV_AUTH_FALLBACK", "false")
	t.Setenv("SESSION_SECRET", "")
	_, err := Load()
	if err == nil {
		t.Fatalf("expected SESSION_SECRET validation error")
	}
}

func TestLoadAllowsPrototypeModeWithoutSessionSecret(t *testing.T) {
	t.Setenv("PROTOTYPE_MODE", "true")
	t.Setenv("ALLOW_DEV_AUTH_FALLBACK", "true")
	t.Setenv("SESSION_SECRET", "")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected load error: %v", err)
	}
	if !cfg.AllowDevAuthFallback || !cfg.PrototypeMode {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}
