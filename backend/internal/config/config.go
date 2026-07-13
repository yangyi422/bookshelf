package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Environment, Port, DataDir, PublicBaseURL, AdminUsername, AdminPassword string
	SessionSecret                                                           string
	OPDSEnabled                                                             bool
	OPDSUsername, OPDSPassword                                              string
	SessionTTL                                                              time.Duration
	MaxUploadBytes                                                          int64
	SQLiteBusyTimeoutMS                                                     int
	BackupRetentionDays                                                     int
	LogLevel                                                                string
}

func Load() (Config, error) {
	c := Config{
		Environment: env("APP_ENV", "development"), Port: env("APP_PORT", "8080"),
		DataDir: env("DATA_DIR", "./data"), PublicBaseURL: env("PUBLIC_BASE_URL", "http://localhost:8080"),
		AdminUsername: strings.TrimSpace(os.Getenv("ADMIN_USERNAME")), AdminPassword: os.Getenv("ADMIN_PASSWORD"),
		SessionSecret: os.Getenv("SESSION_SECRET"), LogLevel: env("LOG_LEVEL", "info"),
		OPDSUsername: strings.TrimSpace(os.Getenv("OPDS_USERNAME")), OPDSPassword: os.Getenv("OPDS_PASSWORD"),
	}
	var err error
	c.OPDSEnabled, err = strconv.ParseBool(env("OPDS_ENABLED", "true"))
	if err != nil {
		return c, errors.New("OPDS_ENABLED must be true or false")
	}
	hours, err := positiveInt("SESSION_TTL_HOURS", 168)
	if err != nil {
		return c, err
	}
	c.SessionTTL = time.Duration(hours) * time.Hour
	mb, err := positiveInt("MAX_UPLOAD_SIZE_MB", 500)
	if err != nil {
		return c, err
	}
	c.MaxUploadBytes = int64(mb) * 1024 * 1024
	c.SQLiteBusyTimeoutMS, err = positiveInt("SQLITE_BUSY_TIMEOUT_MS", 5000)
	if err != nil {
		return c, err
	}
	c.BackupRetentionDays, err = positiveInt("BACKUP_RETENTION_DAYS", 30)
	if err != nil {
		return c, err
	}
	publicURL, parseErr := url.ParseRequestURI(c.PublicBaseURL)
	if parseErr != nil || publicURL.Host == "" || (publicURL.Scheme != "http" && publicURL.Scheme != "https") || (publicURL.Path != "" && publicURL.Path != "/") || publicURL.RawQuery != "" || publicURL.Fragment != "" {
		return c, errors.New("PUBLIC_BASE_URL must be an absolute HTTP(S) URL")
	}
	if c.Environment == "production" {
		if len(c.SessionSecret) < 32 {
			return c, errors.New("SESSION_SECRET must contain at least 32 characters in production")
		}
		if c.AdminUsername != "" && weakPassword(c.AdminPassword) {
			return c, errors.New("ADMIN_PASSWORD is weak or still uses the example value")
		}
		if c.OPDSEnabled && weakPassword(c.OPDSPassword) {
			return c, errors.New("OPDS_PASSWORD must be strong in production")
		}
		if c.OPDSEnabled && publicURL.Scheme != "https" {
			return c, errors.New("PUBLIC_BASE_URL must use HTTPS when OPDS is enabled in production")
		}
	}
	if (c.AdminUsername == "") != (c.AdminPassword == "") {
		return c, errors.New("ADMIN_USERNAME and ADMIN_PASSWORD must be configured together")
	}
	if c.OPDSEnabled && (c.OPDSUsername == "" || c.OPDSPassword == "") {
		return c, errors.New("OPDS_USERNAME and OPDS_PASSWORD are required when OPDS is enabled")
	}
	return c, nil
}

func env(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}
func positiveInt(k string, fallback int) (int, error) {
	v := env(k, strconv.Itoa(fallback))
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", k)
	}
	return n, nil
}
func weakPassword(v string) bool {
	l := strings.ToLower(v)
	return len(v) < 12 || l == "password" || l == "admin" || strings.Contains(l, "change-me") || strings.Contains(l, "replace-with")
}
