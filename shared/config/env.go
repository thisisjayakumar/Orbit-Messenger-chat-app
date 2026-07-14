package config

import (
	"os"
	"strconv"
	"time"
)

// GetEnv returns the value of an environment variable, or a default value if not set.
func GetEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// GetInt returns the value of an environment variable as an int,
// or a default value if not set or not a valid integer.
func GetInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

// GetDuration returns the value of an environment variable as a time.Duration,
// or a default value if not set or not a valid duration.
func GetDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if parsed, err := time.ParseDuration(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}
