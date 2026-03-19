package registration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Harsh223/PanelX/apps/node-agent/internal/config"
)

func TestRegistrationEndpoint_Normalization(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		baseURL string
		want    string
	}{
		{
			name:    "plain host",
			baseURL: "http://127.0.0.1:8080",
			want:    "http://127.0.0.1:8080/v1/agents/register",
		},
		{
			name:    "trailing slash",
			baseURL: "http://127.0.0.1:8080/",
			want:    "http://127.0.0.1:8080/v1/agents/register",
		},
		{
			name:    "base path is ignored for absolute register route",
			baseURL: "http://127.0.0.1:8080/some/prefix",
			want:    "http://127.0.0.1:8080/v1/agents/register",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := registrationEndpoint(tc.baseURL)
			if err != nil {
				t.Fatalf("registrationEndpoint() error = %v", err)
			}
			if got != tc.want {
				t.Fatalf("registrationEndpoint() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestClientRegister_UsesNormalizedEndpointAndAuthorization(t *testing.T) {
	t.Parallel()

	var sawRequest bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawRequest = true

		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/v1/agents/register" {
			t.Fatalf("path = %q, want %q", r.URL.Path, "/v1/agents/register")
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("authorization = %q, want %q", got, "Bearer test-token")
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("content-type = %q, want %q", got, "application/json")
		}

		var payload RegistrationPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload error = %v", err)
		}
		if payload.AgentID != "node-local" || payload.Hostname != "host-a" || payload.IPAddress != "127.0.0.1" {
			t.Fatalf("unexpected payload: %#v", payload)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(RegistrationResult{
			Accepted: true,
			NodeID:   payload.AgentID,
			Message:  "ok",
		})
	}))
	defer srv.Close()

	client := NewClient(config.Config{
		ControlPlaneURL:   "   " + srv.URL + "/nested/path   ",
		RegistrationToken: "test-token",
	})

	res, err := client.Register(context.Background(), RegistrationPayload{
		AgentID:   "node-local",
		Hostname:  "host-a",
		IPAddress: "127.0.0.1",
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if !sawRequest {
		t.Fatalf("expected request to be received by test server")
	}
	if !res.Accepted || res.NodeID != "node-local" {
		t.Fatalf("unexpected registration result: %#v", res)
	}
}

func TestClientRegister_ErrorDiagnosticsIncludeEndpointStatusAndBody(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"code":"not_found","message":"missing route"}`, http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewClient(config.Config{
		ControlPlaneURL:   srv.URL,
		RegistrationToken: "test-token",
	})

	_, err := client.Register(context.Background(), RegistrationPayload{
		AgentID:   "node-local",
		Hostname:  "host-a",
		IPAddress: "127.0.0.1",
	})
	if err == nil {
		t.Fatalf("Register() error = nil, want non-nil")
	}

	msg := err.Error()
	wantEndpoint := "endpoint=" + srv.URL + "/v1/agents/register"
	if !strings.Contains(msg, wantEndpoint) {
		t.Fatalf("error %q does not contain %q", msg, wantEndpoint)
	}
	if !strings.Contains(msg, "status=404 Not Found") {
		t.Fatalf("error %q does not contain status details", msg)
	}
	if !strings.Contains(msg, "body=") {
		t.Fatalf("error %q does not contain response body diagnostics", msg)
	}
	if !strings.Contains(msg, "not_found") || !strings.Contains(msg, "missing route") {
		t.Fatalf("error %q does not contain expected error payload details", msg)
	}
}

func TestClientRegister_ErrorDiagnosticsWithoutBody(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	client := NewClient(config.Config{
		ControlPlaneURL:   srv.URL + "/anything",
		RegistrationToken: "test-token",
	})

	_, err := client.Register(context.Background(), RegistrationPayload{
		AgentID:   "node-local",
		Hostname:  "host-a",
		IPAddress: "127.0.0.1",
	})
	if err == nil {
		t.Fatalf("Register() error = nil, want non-nil")
	}

	msg := err.Error()
	wantEndpoint := "endpoint=" + srv.URL + "/v1/agents/register"
	if !strings.Contains(msg, wantEndpoint) {
		t.Fatalf("error %q does not contain %q", msg, wantEndpoint)
	}
	if !strings.Contains(msg, "status=502 Bad Gateway") {
		t.Fatalf("error %q does not contain status details", msg)
	}
	if strings.Contains(msg, "body=") {
		t.Fatalf("error %q unexpectedly contains body diagnostics", msg)
	}
}

func TestClientRegister_InvalidControlPlaneURL(t *testing.T) {
	t.Parallel()

	client := NewClient(config.Config{
		ControlPlaneURL:   "://bad-url",
		RegistrationToken: "test-token",
	})

	_, err := client.Register(context.Background(), RegistrationPayload{
		AgentID:   "node-local",
		Hostname:  "host-a",
		IPAddress: "127.0.0.1",
	})
	if err == nil {
		t.Fatalf("Register() error = nil, want non-nil")
	}

	msg := err.Error()
	if !strings.Contains(msg, "invalid control-plane URL") {
		t.Fatalf("error %q does not contain invalid URL diagnostics", msg)
	}
}

func TestClientRegister_RequiresRegistrationToken(t *testing.T) {
	t.Parallel()

	client := NewClient(config.Config{
		ControlPlaneURL:   "http://127.0.0.1:8080",
		RegistrationToken: "",
	})

	_, err := client.Register(context.Background(), RegistrationPayload{
		AgentID:   "node-local",
		Hostname:  "host-a",
		IPAddress: "127.0.0.1",
	})
	if err == nil {
		t.Fatalf("Register() error = nil, want non-nil")
	}
	if got := err.Error(); !strings.Contains(got, "registration token is required") {
		t.Fatalf("error = %q, want token-required diagnostics", got)
	}
}
