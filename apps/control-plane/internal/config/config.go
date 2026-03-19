package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config stores runtime settings for the PanelX control-plane service.
type Config struct {
	HTTP              HTTPConfig
	AdminToken        string
	RegistrationToken string
	WebRoot           string
	SitesRoot         string
	PHPSocketPath     string
	DBAdminHost       string
	DBAdminPort       int
	DBAdminUser       string
	DBAdminPassword   string
}

// HTTPConfig contains HTTP server settings.
type HTTPConfig struct {
	Host         string
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

// Load reads configuration from environment variables with safe defaults.
func Load() (Config, error) {
	cfg := Config{
		HTTP: HTTPConfig{
			Host:         getEnv("PANELX_HTTP_HOST", "0.0.0.0"),
			Port:         getEnvInt("PANELX_HTTP_PORT", 8080),
			ReadTimeout:  getEnvDuration("PANELX_HTTP_READ_TIMEOUT", 15*time.Second),
			WriteTimeout: getEnvDuration("PANELX_HTTP_WRITE_TIMEOUT", 15*time.Second),
			IdleTimeout:  getEnvDuration("PANELX_HTTP_IDLE_TIMEOUT", 60*time.Second),
		},
		AdminToken:        getEnv("PANELX_ADMIN_TOKEN", "change-me"),
		RegistrationToken: getEnv("PANELX_REGISTRATION_TOKEN", ""),
		WebRoot:           getEnv("PANELX_WEB_ROOT", "/opt/panelx/web"),
		SitesRoot:         getEnv("PANELX_SITES_ROOT", "/var/www/panelx/sites"),
		PHPSocketPath:     getEnv("PANELX_PHP_FPM_SOCKET", "/run/php/php8.3-fpm.sock"),
		DBAdminHost:       getEnv("PANELX_DB_ADMIN_HOST", "127.0.0.1"),
		DBAdminPort:       getEnvInt("PANELX_DB_ADMIN_PORT", 3306),
		DBAdminUser:       getEnv("PANELX_DB_ADMIN_USER", "panelx_admin"),
		DBAdminPassword:   getEnv("PANELX_DB_ADMIN_PASSWORD", ""),
	}

	if cfg.HTTP.Port < 1 || cfg.HTTP.Port > 65535 {
		return Config{}, fmt.Errorf("invalid PANELX_HTTP_PORT %d: must be between 1 and 65535", cfg.HTTP.Port)
	}
	if cfg.AdminToken == "" || cfg.AdminToken == "change-me" {
		return Config{}, fmt.Errorf("PANELX_ADMIN_TOKEN must be set to a secure value")
	}
	if cfg.RegistrationToken == "" {
		cfg.RegistrationToken = cfg.AdminToken
	}
	if cfg.RegistrationToken == "" || cfg.RegistrationToken == "change-me" {
		return Config{}, fmt.Errorf("PANELX_REGISTRATION_TOKEN must be set to a secure value")
	}
	if cfg.DBAdminPassword == "" {
		return Config{}, fmt.Errorf("PANELX_DB_ADMIN_PASSWORD is required")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func getEnvInt(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}

	value, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}
	return value
}
