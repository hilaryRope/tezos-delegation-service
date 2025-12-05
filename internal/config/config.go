package config

import (
	"fmt"
	"os"
	"time"
)

// Config holds all configuration for the service.
type Config struct {
	DB_DSN            string
	HTTPAddr          string
	TzktBaseURL       string
	HTTPClientTimeout time.Duration
	PollerInterval    time.Duration
	PollerBatchSize   int
}

// Load returns a new Config struct populated from environment variables.
func Load() Config {
	return Config{
		DB_DSN:            getenv("DB_DSN", "postgres://xtz:xtz@localhost:5432/xtz?sslmode=disable"),
		HTTPAddr:          getenv("HTTP_ADDR", ":8080"),
		TzktBaseURL:       getenv("TZKT_BASE_URL", "https://api.tzkt.io/v1"),
		HTTPClientTimeout: getenvDuration("HTTP_CLIENT_TIMEOUT", 10*time.Second),
		PollerInterval:    getenvDuration("POLLER_INTERVAL", 15*time.Second),
		PollerBatchSize:   getenvInt("POLLER_BATCH_SIZE", 10000),
	}
}

func getenv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return def
}

func getenvDuration(key string, def time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func getenvInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok {
		var i int
		if _, err := fmt.Sscanf(v, "%d", &i); err == nil {
			return i
		}
	}
	return def
}
