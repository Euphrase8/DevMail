// Package config loads backend configuration from environment variables
// with safe defaults for the local development database.
package config

import (
	"os"
	"strings"
	"time"
)

// Config holds all runtime configuration for the API service.
type Config struct {
	PGDumpPath           string
	BackupDirectory      string
	HTTPAddr             string
	DatabaseURL          string
	AllowedOrigins       []string
	AllowPrivateNetworks bool
	Env                  string

	// SecretKey is the passphrase used to derive the key that encrypts
	// credentials and private keys at rest. Override in production.
	SecretKey string

	// MailEngine selects the provisioning adapter: "none" (default, records
	// only) or "sql" (Dovecot/Postfix-style virtual users written to the
	// engine_* tables). A real mail server consumes the SQL rows.
	MailEngine string

	// Timeouts used by connectivity/DNS checks.
	DialTimeout     time.Duration
	ResolverTimeout time.Duration
}

// Load reads configuration from the environment.
func Load() Config {
	cfg := Config{
		PGDumpPath:           getenv("PG_DUMP_PATH", ""),
		BackupDirectory:      getenv("BACKUP_DIRECTORY", ""),
		HTTPAddr:             getenv("HTTP_ADDR", ":7000"),
		DatabaseURL:          getenv("DATABASE_URL", "postgres://postgres:1508@127.0.0.1:5432/devmail?sslmode=disable"),
		AllowPrivateNetworks: getenv("ALLOW_PRIVATE_NETWORKS", "false") == "true",
		Env:                  getenv("APP_ENV", "development"),
		SecretKey:            getenv("SECRET_KEY", "devmail-insecure-dev-key-change-me"),
		MailEngine:           getenv("MAIL_ENGINE", "none"),
		DialTimeout:          5 * time.Second,
		ResolverTimeout:      5 * time.Second,
	}
	for _, o := range strings.Split(getenv("CORS_ALLOWED_ORIGINS", "http://localhost:8080,http://localhost:8081,http://localhost:3001"), ",") {
		if o = strings.TrimSpace(o); o != "" {
			cfg.AllowedOrigins = append(cfg.AllowedOrigins, o)
		}
	}
	return cfg
}

// IsDevelopment reports whether the app is running in development mode.
func (c Config) IsDevelopment() bool { return c.Env == "development" }

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
