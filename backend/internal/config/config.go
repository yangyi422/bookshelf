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
	Environment, Port, DataDir, WebDir, PublicBaseURL, AdminUsername, AdminPassword string
	SessionSecret                                                                   string
	OPDSEnabled, OPDSAllowInsecureHTTP                                              bool
	OPDSUsername, OPDSPassword, OPDSAccessMode                                      string
	SessionTTL                                                                      time.Duration
	MaxUploadBytes                                                                  int64
	SQLiteBusyTimeoutMS                                                             int
	BackupRetentionDays                                                             int
	LogLevel                                                                        string
	TrustedProxies                                                                  []string
}

func Load() (Config, error) {
	environment := env("APP_ENV", "development")
	defaultWebDir := ""
	if environment == "production" {
		defaultWebDir = "/app/web"
	}
	c := Config{
		Environment: environment, Port: env("APP_PORT", "8080"),
		DataDir: env("DATA_DIR", "./data"), WebDir: env("WEB_DIR", defaultWebDir), PublicBaseURL: strings.TrimRight(strings.TrimSpace(os.Getenv("PUBLIC_BASE_URL")), "/"),
		AdminUsername: strings.TrimSpace(os.Getenv("ADMIN_USERNAME")), AdminPassword: os.Getenv("ADMIN_PASSWORD"),
		SessionSecret: os.Getenv("SESSION_SECRET"), LogLevel: env("LOG_LEVEL", "info"),
		OPDSUsername: strings.TrimSpace(os.Getenv("OPDS_USERNAME")), OPDSPassword: os.Getenv("OPDS_PASSWORD"),
		TrustedProxies: splitCSV(env("TRUSTED_PROXIES", "127.0.0.1,::1")),
	}
	var err error
	c.OPDSEnabled, err = strconv.ParseBool(env("OPDS_ENABLED", "true"))
	if err != nil {
		return c, errors.New("OPDS_ENABLED must be true or false")
	}
	c.OPDSAllowInsecureHTTP, err = strconv.ParseBool(env("OPDS_ALLOW_INSECURE_HTTP", "false"))
	if err != nil {
		return c, errors.New("OPDS_ALLOW_INSECURE_HTTP must be true or false")
	}
	c.OPDSAccessMode = strings.TrimSpace(os.Getenv("OPDS_ACCESS_MODE"))
	if c.OPDSAccessMode == "" {
		if !c.OPDSEnabled {
			c.OPDSAccessMode = "disabled"
		} else if c.OPDSAllowInsecureHTTP {
			c.OPDSAccessMode = "http_and_https"
		} else {
			c.OPDSAccessMode = "https_only"
		}
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
	if c.PublicBaseURL != "" {
		publicURL, parseErr := url.ParseRequestURI(c.PublicBaseURL)
		if parseErr != nil || publicURL.Host == "" || (publicURL.Scheme != "http" && publicURL.Scheme != "https") || (publicURL.Path != "" && publicURL.Path != "/") || publicURL.RawQuery != "" || publicURL.Fragment != "" {
			return c, errors.New("PUBLIC_BASE_URL must be an absolute HTTP(S) URL")
		}
	}
	if c.SessionSecret != "" && len(c.SessionSecret) < 32 {
		return c, errors.New("SESSION_SECRET must contain at least 32 characters when explicitly configured")
	}
	if c.Environment == "production" {
		if c.AdminUsername != "" && weakPassword(c.AdminPassword) {
			return c, errors.New("ADMIN_PASSWORD is weak or still uses the example value")
		}
	}
	if (c.AdminUsername == "") != (c.AdminPassword == "") {
		return c, errors.New("ADMIN_USERNAME and ADMIN_PASSWORD must be configured together")
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
func splitCSV(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}
