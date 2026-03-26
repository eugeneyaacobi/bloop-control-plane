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
	if cfg.SessionCookieSecure {
		t.Fatalf("expected insecure cookies by default in prototype mode")
	}
}

func TestLoadDefaultsSecureCookiesOutsidePrototypeMode(t *testing.T) {
	t.Setenv("PROTOTYPE_MODE", "false")
	t.Setenv("ALLOW_DEV_AUTH_FALLBACK", "true")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected load error: %v", err)
	}
	if !cfg.SessionCookieSecure {
		t.Fatalf("expected secure cookies outside prototype mode")
	}
}
