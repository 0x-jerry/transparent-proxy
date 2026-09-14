package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Addr        string
	Timeout     time.Duration
	InsecureTLS bool
	CORS        bool
}

func Load() (Config, error) {
	cfg := Config{
		Addr:    "127.0.0.1:8080",
		Timeout: 30 * time.Second,
		CORS:    true,
	}

	if v := os.Getenv("PROXY_ADDR"); v != "" {
		cfg.Addr = v
	}

	timeout, err := durationEnv("PROXY_TIMEOUT", cfg.Timeout)
	if err != nil {
		return Config{}, err
	}
	cfg.Timeout = timeout

	insecure, err := boolEnv("PROXY_INSECURE_TLS", false)
	if err != nil {
		return Config{}, err
	}
	cfg.InsecureTLS = insecure

	cors, err := boolEnv("PROXY_CORS", cfg.CORS)
	if err != nil {
		return Config{}, err
	}
	cfg.CORS = cors

	return cfg, nil
}

func durationEnv(key string, def time.Duration) (time.Duration, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return def, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	if d < 0 {
		return 0, fmt.Errorf("%s: must not be negative", key)
	}
	return d, nil
}

func boolEnv(key string, def bool) (bool, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return def, nil
	}
	b, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("%s: %w", key, err)
	}
	return b, nil
}
