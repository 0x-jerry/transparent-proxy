package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("PROXY_ADDR", "")
	t.Setenv("PROXY_TIMEOUT", "")
	t.Setenv("PROXY_INSECURE_TLS", "")
	t.Setenv("PROXY_CORS", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Addr != "127.0.0.1:8080" {
		t.Errorf("Addr = %q", cfg.Addr)
	}
	if cfg.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v", cfg.Timeout)
	}
	if cfg.InsecureTLS {
		t.Error("InsecureTLS should default to false")
	}
	if !cfg.CORS {
		t.Error("CORS should default to true")
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("PROXY_ADDR", ":9000")
	t.Setenv("PROXY_TIMEOUT", "5s")
	t.Setenv("PROXY_INSECURE_TLS", "true")
	t.Setenv("PROXY_CORS", "false")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Addr != ":9000" || cfg.Timeout != 5*time.Second || !cfg.InsecureTLS || cfg.CORS {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestLoadInvalid(t *testing.T) {
	t.Setenv("PROXY_TIMEOUT", "soon")
	if _, err := Load(); err == nil {
		t.Error("want error for invalid timeout")
	}

	t.Setenv("PROXY_TIMEOUT", "")
	t.Setenv("PROXY_INSECURE_TLS", "maybe")
	if _, err := Load(); err == nil {
		t.Error("want error for invalid bool")
	}
}
