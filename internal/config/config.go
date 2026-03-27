package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ListenAddr  string
	DatabaseURL string
	LogLevel    string

	SMTPHost     string
	SMTPPort     int
	SMTPUser     string
	SMTPPassword string
	SMTPFrom     string

	VerificationTokenTTL time.Duration
	PrototypeMode        bool
	AllowDevAuthFallback bool
	SessionSecret        string
	SessionCookieName    string
	SessionTTL           time.Duration
	SessionCookieSecure  bool
	SessionCookieDomain  string
	RuntimeIngestSecret  string
}

func Load() (*Config, error) {
	ttlRaw := getenv("VERIFICATION_TOKEN_TTL_SECONDS", "3600")
	ttlSeconds, err := strconv.Atoi(ttlRaw)
	if err != nil {
		return nil, fmt.Errorf("invalid VERIFICATION_TOKEN_TTL_SECONDS: %w", err)
	}

	smtpPortRaw := getenv("SMTP_PORT", "587")
	smtpPort, err := strconv.Atoi(smtpPortRaw)
	if err != nil {
		return nil, fmt.Errorf("invalid SMTP_PORT: %w", err)
	}

	prototypeMode, err := getenvBool("PROTOTYPE_MODE", false)
	if err != nil {
		return nil, fmt.Errorf("invalid PROTOTYPE_MODE: %w", err)
	}
	allowDevAuthFallback, err := getenvBool("ALLOW_DEV_AUTH_FALLBACK", prototypeMode)
	if err != nil {
		return nil, fmt.Errorf("invalid ALLOW_DEV_AUTH_FALLBACK: %w", err)
	}
	cookieSecure, err := getenvBool("SESSION_COOKIE_SECURE", !prototypeMode)
	if err != nil {
		return nil, fmt.Errorf("invalid SESSION_COOKIE_SECURE: %w", err)
	}

	sessionTTLRaw := getenv("SESSION_TTL_SECONDS", "604800")
	sessionTTLSeconds, err := strconv.Atoi(sessionTTLRaw)
	if err != nil {
		return nil, fmt.Errorf("invalid SESSION_TTL_SECONDS: %w", err)
	}

	cfg := &Config{
		ListenAddr:           os.Getenv("LISTEN_ADDR"),
		DatabaseURL:          os.Getenv("DATABASE_URL"),
		LogLevel:             getenv("LOG_LEVEL", "info"),
		SMTPHost:             os.Getenv("SMTP_HOST"),
		SMTPPort:             smtpPort,
		SMTPUser:             os.Getenv("SMTP_USER"),
		SMTPPassword:         os.Getenv("SMTP_PASSWORD"),
		SMTPFrom:             os.Getenv("SMTP_FROM"),
		VerificationTokenTTL: time.Duration(ttlSeconds) * time.Second,
		PrototypeMode:        prototypeMode,
		AllowDevAuthFallback: allowDevAuthFallback,
		SessionSecret:        os.Getenv("SESSION_SECRET"),
		SessionCookieName:    getenv("SESSION_COOKIE_NAME", "bloop_session"),
		SessionTTL:           time.Duration(sessionTTLSeconds) * time.Second,
		SessionCookieSecure:  cookieSecure,
		SessionCookieDomain:  os.Getenv("SESSION_COOKIE_DOMAIN"),
		RuntimeIngestSecret:  os.Getenv("RUNTIME_INGEST_SECRET"),
	}

	if cfg.ListenAddr == "" {
		cfg.ListenAddr = ":8081"
	}
	if strings.TrimSpace(cfg.SessionSecret) == "" && !cfg.AllowDevAuthFallback {
		return nil, fmt.Errorf("SESSION_SECRET is required when ALLOW_DEV_AUTH_FALLBACK is false")
	}
	return cfg, nil
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getenvBool(key string, fallback bool) (bool, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, err
	}
	return value, nil
}
