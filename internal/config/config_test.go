package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadSuccess(t *testing.T) {
	t.Setenv("PCB_PROVIDER", "ics")
	t.Setenv("PCB_ICS_URL", "https://example.test/calendar.ics")
	t.Setenv("PCB_BIND_ADDRESS", "127.0.0.1:9999")
	t.Setenv("PCB_REQUIRE_TOKEN", "true")
	t.Setenv("PCB_BEARER_TOKEN", "secret")
	t.Setenv("PCB_REQUEST_TIMEOUT", "5s")
	t.Setenv("PCB_LOG_LEVEL", "debug")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.RequestTimeout != 5*time.Second {
		t.Fatalf("unexpected timeout: %v", cfg.RequestTimeout)
	}
	if cfg.ProviderType != "ics" {
		t.Fatalf("unexpected provider type: %q", cfg.ProviderType)
	}
}

func TestValidateErrors(t *testing.T) {
	cases := []Config{
		{},
		{Provider: "ics", BindAddress: "127.0.0.1:1", RequireBearerToken: true},
		{Provider: "ics", ICSURL: "", RequireBearerToken: false, RequestTimeout: time.Second, BindAddress: "127.0.0.1:1"},
		{Provider: "ics", ICSURL: "x", RequireBearerToken: false, RequestTimeout: -1 * time.Second, BindAddress: "127.0.0.1:1"},
		{Provider: "ics", ICSURL: "x", RequireBearerToken: false, RequestTimeout: time.Second, LogLevel: "trace", BindAddress: "127.0.0.1:1"},
		{ProviderType: "bogus", RequireBearerToken: false, RequestTimeout: time.Second, LogLevel: "info", BindAddress: "127.0.0.1:1"},
	}
	for _, tc := range cases {
		if tc.RequestTimeout == 0 {
			tc.RequestTimeout = time.Second
		}
		if tc.LogLevel == "" {
			tc.LogLevel = "info"
		}
		if err := tc.Validate(); err == nil {
			t.Fatalf("expected validation error for %+v", tc)
		}
	}
}

func TestDefaultsWhenEnvInvalid(t *testing.T) {
	for _, key := range []string{"PCB_PROVIDER", "PCB_ICS_URL", "PCB_BEARER_TOKEN", "PCB_BIND_ADDRESS", "PCB_LOG_LEVEL", "PCB_REQUEST_TIMEOUT", "PCB_REQUIRE_TOKEN", "PCB_ENABLE_TRAY"} {
		_ = os.Unsetenv(key)
	}
	t.Setenv("PCB_ICS_URL", "https://example.test/calendar.ics")
	t.Setenv("PCB_BEARER_TOKEN", "secret")
	t.Setenv("PCB_REQUEST_TIMEOUT", "oops")
	t.Setenv("PCB_REQUIRE_TOKEN", "oops")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.RequestTimeout != 10*time.Second {
		t.Fatalf("expected default timeout, got %v", cfg.RequestTimeout)
	}
	if !cfg.RequireBearerToken {
		t.Fatalf("expected default true for RequireBearerToken")
	}
}
