package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSessionSecretGeneratedAndPersistedInProduction(t *testing.T) {
	dataDir := t.TempDir()
	secret, source, err := ResolveSessionSecret(dataDir, "")
	if err != nil {
		t.Fatal(err)
	}
	if source != SessionSecretCreated {
		t.Fatalf("source = %q", source)
	}
	if len(secret) != 64 {
		t.Fatalf("generated secret length = %d", len(secret))
	}
	path := filepath.Join(dataDir, sessionSecretRelativePath)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("secret mode = %o", info.Mode().Perm())
	}

	loaded, source, err := ResolveSessionSecret(dataDir, "")
	if err != nil {
		t.Fatal(err)
	}
	if source != SessionSecretLoaded || loaded != secret {
		t.Fatalf("persisted secret was not reused: source=%q", source)
	}
}

func TestSessionSecretEnvironmentOverride(t *testing.T) {
	dataDir := t.TempDir()
	explicit := "this-is-an-explicit-session-secret-value"
	secret, source, err := ResolveSessionSecret(dataDir, explicit)
	if err != nil {
		t.Fatal(err)
	}
	if secret != explicit || source != SessionSecretEnvironment {
		t.Fatalf("secret source = %q", source)
	}
	if _, err := os.Stat(filepath.Join(dataDir, sessionSecretRelativePath)); !os.IsNotExist(err) {
		t.Fatalf("environment override unexpectedly created a file: %v", err)
	}
}

func TestSessionSecretRejectsWeakEnvironmentOverride(t *testing.T) {
	if _, _, err := ResolveSessionSecret(t.TempDir(), "too-short"); err == nil {
		t.Fatal("expected weak explicit secret to be rejected")
	}
}

func TestSessionSecretRejectsDamagedPersistedFile(t *testing.T) {
	for _, content := range []string{"", "not-hex", "abcd"} {
		t.Run(content, func(t *testing.T) {
			dataDir := t.TempDir()
			configDir := filepath.Join(dataDir, "config")
			if err := os.MkdirAll(configDir, 0o750); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(configDir, "session_secret")
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, _, err := ResolveSessionSecret(dataDir, ""); err == nil {
				t.Fatal("expected damaged persisted secret to be rejected")
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != content {
				t.Fatalf("damaged secret was silently replaced: content=%q error=%v", got, err)
			}
		})
	}
}
