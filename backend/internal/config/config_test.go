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

func TestProductionAllowsMissingSessionSecret(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("SESSION_SECRET", "")
	t.Setenv("OPDS_ENABLED", "false")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SessionSecret != "" {
		t.Fatal("expected empty secret to be resolved after storage initialization")
	}
}

func TestSessionCookieSecureConfiguration(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("OPDS_ENABLED", "false")
	for _, mode := range []string{"auto", "always", "never"} {
		t.Setenv("SESSION_COOKIE_SECURE", mode)
		cfg, err := Load()
		if err != nil || cfg.SessionCookieSecure != mode {
			t.Fatalf("mode %q: config=%q error=%v", mode, cfg.SessionCookieSecure, err)
		}
	}
	t.Setenv("SESSION_COOKIE_SECURE", "sometimes")
	if _, err := Load(); err == nil {
		t.Fatal("expected invalid cookie security mode to fail")
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

func TestWebDirDefaultsForProductionAndCanBeOverridden(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("SESSION_SECRET", "this-is-a-long-enough-session-secret-value")
	t.Setenv("OPDS_ENABLED", "false")
	cfg, err := Load()
	if err != nil || cfg.WebDir != "/app/web" {
		t.Fatalf("WEB_DIR = %q, error %v", cfg.WebDir, err)
	}
	t.Setenv("WEB_DIR", "/srv/bookshelf-web")
	cfg, err = Load()
	if err != nil || cfg.WebDir != "/srv/bookshelf-web" {
		t.Fatalf("overridden WEB_DIR = %q, error %v", cfg.WebDir, err)
	}
}
