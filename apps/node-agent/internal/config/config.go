package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config stores runtime settings for the PanelX node-agent.
type Config struct {
	AgentID             string
	ControlPlaneURL     string
	RegistrationToken   string
	HeartbeatInterval   time.Duration
	HTTPListenAddr      string
	InsecureSkipTLSVerify bool
}

// Load reads node-agent configuration from environment.
func Load() (Config, error) {
	cfg := Config{
		AgentID:               getEnv("PANELX_AGENT_ID", "agent-local"),
		ControlPlaneURL:       getEnv("PANELX_CONTROL_PLANE_URL", "http://127.0.0.1:8080"),
		RegistrationToken:     getEnv("PANELX_REGISTRATION_TOKEN", ""),
		HeartbeatInterval:     getEnvDuration("PANELX_HEARTBEAT_INTERVAL", 30*time.Second),
		HTTPListenAddr:        getEnv("PANELX_AGENT_HTTP_ADDR", "0.0.0.0:8090"),
		InsecureSkipTLSVerify: getEnvBool("PANELX_INSECURE_SKIP_TLS_VERIFY", false),
	}

	if cfg.ControlPlaneURL == "" {
		return Config{}, fmt.Errorf("PANELX_CONTROL_PLANE_URL is required")
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

func getEnvBool(key string, fallback bool) bool {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return value
}
