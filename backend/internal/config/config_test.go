package config

import "testing"

func TestProductionValidation(t *testing.T) {
	t.Setenv("OPDS_ENABLED", "false")
	t.Setenv("APP_ENV", "production")
	t.Setenv("SESSION_SECRET", "short")
	if _, err := Load(); err == nil {
		t.Fatal("expected short secret to fail")
	}
	t.Setenv("SESSION_SECRET", "this-is-a-long-enough-session-secret-value")
	t.Setenv("ADMIN_USERNAME", "admin")
	t.Setenv("ADMIN_PASSWORD", "change-me-immediately")
	if _, err := Load(); err == nil {
		t.Fatal("expected weak password to fail")
	}
}

func TestLegacyOPDSEnvironmentMapsAccessMode(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("SESSION_SECRET", "this-is-a-long-enough-session-secret-value")
	t.Setenv("OPDS_ENABLED", "true")
	t.Setenv("OPDS_ALLOW_INSECURE_HTTP", "true")
	cfg, err := Load()
	if err != nil || cfg.OPDSAccessMode != "http_and_https" {
		t.Fatalf("mode = %q, error %v", cfg.OPDSAccessMode, err)
	}
}

func TestOPDSAllowInsecureHTTPDefaultsFalse(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("OPDS_ENABLED", "false")
	if cfg, err := Load(); err != nil || cfg.OPDSAllowInsecureHTTP {
		t.Fatalf("expected disabled default, got %t, error %v", cfg.OPDSAllowInsecureHTTP, err)
	}
}
