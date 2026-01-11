package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all configuration for the application
type Config struct {
	Port                     int
	PodName                  string
	PodNamespace             string
	SecretNames              []string
	AppTitle                 string
	AppVersion               string
	DashboardRefreshInterval time.Duration
	ShowSecretValues         bool
}

// LoadConfig loads configuration from environment variables
func LoadConfig() *Config {
	cfg := &Config{
		Port:         getEnvAsInt("PORT", 8080),
		PodName:      getEnv("POD_NAME", ""),
		PodNamespace: getEnv("POD_NAMESPACE", ""),
		AppTitle:     getEnv("APP_TITLE", "Bitwarden Secrets Reader"),
		AppVersion:   getEnv("APP_VERSION", "1.0.0"),
		ShowSecretValues: getEnvAsBool("SHOW_SECRET_VALUES", false),
	}

	// Parse secret names from comma-separated list
	secretNamesStr := getEnv("SECRET_NAMES", "")
	if secretNamesStr != "" {
		cfg.SecretNames = strings.Split(secretNamesStr, ",")
		// Trim whitespace from each secret name
		for i, name := range cfg.SecretNames {
			cfg.SecretNames[i] = strings.TrimSpace(name)
		}
	}

	// Parse dashboard refresh interval (in seconds)
	refreshInterval := getEnvAsInt("DASHBOARD_REFRESH_INTERVAL", 5)
	cfg.DashboardRefreshInterval = time.Duration(refreshInterval) * time.Second

	return cfg
}

// getEnv retrieves an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvAsInt retrieves an environment variable as an integer or returns a default value
func getEnvAsInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

// getEnvAsBool retrieves an environment variable as a boolean or returns a default value
func getEnvAsBool(key string, defaultValue bool) bool {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.ParseBool(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}
