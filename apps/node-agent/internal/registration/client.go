package registration

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Harsh223/PanelX/apps/node-agent/internal/config"
)

// Client handles secure bootstrap registration and heartbeat stubs.
type Client struct {
	httpClient *http.Client
	cfg        config.Config
}

// RegistrationPayload defines bootstrap identity sent to control-plane.
type RegistrationPayload struct {
	AgentID   string `json:"agentId"`
	Hostname  string `json:"hostname"`
	IPAddress string `json:"ipAddress"`
}

// RegistrationResult is the expected bootstrap response contract.
type RegistrationResult struct {
	Accepted bool   `json:"accepted"`
	NodeID   string `json:"nodeId"`
	Message  string `json:"message"`
}

// NewClient creates a registration client with secure defaults.
func NewClient(cfg config.Config) *Client {
	transport := &http.Transport{}
	if cfg.InsecureSkipTLSVerify {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // Explicitly controlled via env for local dev bootstrap only.
	}

	return &Client{
		httpClient: &http.Client{
			Timeout:   10 * time.Second,
			Transport: transport,
		},
		cfg: cfg,
	}
}

// Register sends a bootstrap registration request stub to control-plane.
func (c *Client) Register(ctx context.Context, payload RegistrationPayload) (RegistrationResult, error) {
	if c.cfg.RegistrationToken == "" {
		return RegistrationResult{}, fmt.Errorf("registration token is required")
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return RegistrationResult{}, fmt.Errorf("marshal registration payload: %w", err)
	}

	baseURL := strings.TrimSpace(c.cfg.ControlPlaneURL)
	if baseURL == "" {
		return RegistrationResult{}, fmt.Errorf("control-plane URL is required")
	}

	endpoint, err := registrationEndpoint(baseURL)
	if err != nil {
		return RegistrationResult{}, fmt.Errorf("invalid control-plane URL %q: %w", baseURL, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return RegistrationResult{}, fmt.Errorf("build registration request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.cfg.RegistrationToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return RegistrationResult{}, fmt.Errorf("perform registration request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		rawErr, readErr := io.ReadAll(io.LimitReader(resp.Body, 2048))
		if readErr != nil {
			return RegistrationResult{}, fmt.Errorf("registration failed: endpoint=%s status=%s (failed to read error body: %w)", endpoint, resp.Status, readErr)
		}

		errBody := strings.TrimSpace(string(rawErr))
		if errBody == "" {
			return RegistrationResult{}, fmt.Errorf("registration failed: endpoint=%s status=%s", endpoint, resp.Status)
		}

		return RegistrationResult{}, fmt.Errorf("registration failed: endpoint=%s status=%s body=%q", endpoint, resp.Status, errBody)
	}

	var result RegistrationResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return RegistrationResult{}, fmt.Errorf("decode registration response: %w", err)
	}

	return result, nil
}

func registrationEndpoint(controlPlaneURL string) (string, error) {
	base, err := url.Parse(controlPlaneURL)
	if err != nil {
		return "", err
	}

	if base.Scheme == "" || base.Host == "" {
		return "", fmt.Errorf("URL must include scheme and host")
	}

	registerPath, err := url.Parse("/v1/agents/register")
	if err != nil {
		return "", err
	}

	return base.ResolveReference(registerPath).String(), nil
}
