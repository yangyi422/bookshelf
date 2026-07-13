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

func TestOPDSProductionRequiresHTTPS(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("SESSION_SECRET", "this-is-a-long-enough-session-secret-value")
	t.Setenv("OPDS_ENABLED", "true")
	t.Setenv("OPDS_USERNAME", "reader")
	t.Setenv("OPDS_PASSWORD", "strong-reader-password")
	t.Setenv("PUBLIC_BASE_URL", "http://books.example.com")
	if _, err := Load(); err == nil {
		t.Fatal("expected HTTP OPDS base URL rejection")
	}
}
