package config

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const sessionSecretRelativePath = "config/session_secret"

type SessionSecretSource string

const (
	SessionSecretEnvironment SessionSecretSource = "environment"
	SessionSecretLoaded      SessionSecretSource = "loaded"
	SessionSecretCreated     SessionSecretSource = "created"
)

func ResolveSessionSecret(dataDir, environmentValue string) (string, SessionSecretSource, error) {
	if environmentValue != "" {
		if len(environmentValue) < 32 {
			return "", "", errors.New("SESSION_SECRET must contain at least 32 characters when explicitly configured")
		}
		return environmentValue, SessionSecretEnvironment, nil
	}

	configDir := filepath.Join(dataDir, "config")
	if err := os.MkdirAll(configDir, 0o750); err != nil {
		return "", "", fmt.Errorf("create session secret directory: %w", err)
	}
	if err := os.Chmod(configDir, 0o750); err != nil {
		return "", "", fmt.Errorf("secure session secret directory: %w", err)
	}
	path := filepath.Join(configDir, "session_secret")
	secret, err := loadSessionSecret(path)
	if err == nil {
		return secret, SessionSecretLoaded, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", "", err
	}

	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", "", fmt.Errorf("generate session secret: %w", err)
	}
	secret = hex.EncodeToString(random)
	temp, err := os.CreateTemp(configDir, ".session-secret-")
	if err != nil {
		return "", "", fmt.Errorf("create temporary session secret: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return "", "", fmt.Errorf("secure temporary session secret: %w", err)
	}
	if _, err := temp.WriteString(secret + "\n"); err != nil {
		temp.Close()
		return "", "", fmt.Errorf("write temporary session secret: %w", err)
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return "", "", fmt.Errorf("sync temporary session secret: %w", err)
	}
	if err := temp.Close(); err != nil {
		return "", "", fmt.Errorf("close temporary session secret: %w", err)
	}

	// Linking a fully written file creates the destination atomically without
	// replacing a secret that a concurrent process may have created first.
	if err := os.Link(tempPath, path); err != nil {
		if errors.Is(err, os.ErrExist) {
			loaded, loadErr := loadSessionSecret(path)
			if loadErr != nil {
				return "", "", loadErr
			}
			return loaded, SessionSecretLoaded, nil
		}
		return "", "", fmt.Errorf("persist session secret: %w", err)
	}
	if err := syncDirectory(configDir); err != nil {
		return "", "", err
	}
	return secret, SessionSecretCreated, nil
}

func loadSessionSecret(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	secret := strings.TrimSpace(string(b))
	decoded, decodeErr := hex.DecodeString(secret)
	if secret == "" || decodeErr != nil || len(decoded) < 32 {
		return "", fmt.Errorf("persisted session secret is empty or invalid")
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return "", fmt.Errorf("secure persisted session secret: %w", err)
	}
	return secret, nil
}

func syncDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open session secret directory: %w", err)
	}
	defer dir.Close()
	if err := dir.Sync(); err != nil {
		return fmt.Errorf("sync session secret directory: %w", err)
	}
	return nil
}
