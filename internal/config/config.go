package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ProviderType       string
	Provider           string // Deprecated alias for ProviderType.
	ICSURL             string
	BindAddress        string
	UnixSocketPath     string
	RequireBearerToken bool
	BearerToken        string
	RequestTimeout     time.Duration
	LogLevel           string
	EnableTray         bool
}

func Load() (Config, error) {
	providerType := getenvDefault("PCB_PROVIDER", "ics")
	cfg := Config{
		ProviderType:       providerType,
		Provider:           providerType,
		ICSURL:             strings.TrimSpace(os.Getenv("PCB_ICS_URL")),
		BindAddress:        getenvDefault("PCB_BIND_ADDRESS", "127.0.0.1:9842"),
		UnixSocketPath:     strings.TrimSpace(os.Getenv("PCB_UNIX_SOCKET")),
		RequireBearerToken: getenvBool("PCB_REQUIRE_TOKEN", true),
		BearerToken:        strings.TrimSpace(os.Getenv("PCB_BEARER_TOKEN")),
		RequestTimeout:     getenvDuration("PCB_REQUEST_TIMEOUT", 10*time.Second),
		LogLevel:           getenvDefault("PCB_LOG_LEVEL", "info"),
		EnableTray:         getenvBool("PCB_ENABLE_TRAY", false),
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	providerType := strings.TrimSpace(c.ProviderType)
	if providerType == "" {
		providerType = strings.TrimSpace(c.Provider)
	}
	if providerType == "" {
		return errors.New("provider is required")
	}
	switch providerType {
	case "ics":
		if c.ICSURL == "" {
			return errors.New("PCB_ICS_URL is required when provider=ics")
		}
	case "proton":
		// Proton provider can boot without preconfigured ICS URL.
	default:
		return fmt.Errorf("invalid provider type: %s", providerType)
	}
	if c.BindAddress == "" && c.UnixSocketPath == "" {
		return errors.New("either bind address or unix socket path must be configured")
	}
	if c.RequireBearerToken && c.BearerToken == "" {
		return errors.New("PCB_BEARER_TOKEN is required when token auth is enabled")
	}
	if c.RequestTimeout <= 0 {
		return errors.New("request timeout must be > 0")
	}
	switch strings.ToLower(c.LogLevel) {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("invalid log level: %s", c.LogLevel)
	}
	return nil
}

func getenvDefault(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return fallback
}

func getenvBool(key string, fallback bool) bool {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	return parsed
}

func getenvDuration(key string, fallback time.Duration) time.Duration {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	d, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	return d
}
