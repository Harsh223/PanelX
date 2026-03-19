package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAgentHandlerRegisterMethodGuard(t *testing.T) {
	t.Parallel()

	handler := NewAgentHandler("secret-token")

	req := httptest.NewRequest(http.MethodGet, "/v1/agents/register", nil)
	rr := httptest.NewRecorder()

	handler.Register(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}

	var payload APIError
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v body=%q", err, rr.Body.String())
	}
	if payload.Code != "method_not_allowed" {
		t.Fatalf("code = %q, want %q", payload.Code, "method_not_allowed")
	}
}

func TestAgentHandlerRegisterUnavailableWhenTokenNotConfigured(t *testing.T) {
	t.Parallel()

	handler := NewAgentHandler("")

	req := newRegisterRequest(t, "secret-token", map[string]any{
		"agentId":   "node-local-1",
		"hostname":  "bootstrap-host",
		"ipAddress": "127.0.0.1",
	})
	rr := httptest.NewRecorder()

	handler.Register(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}

	var payload APIError
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v body=%q", err, rr.Body.String())
	}
	if payload.Code != "registration_unavailable" {
		t.Fatalf("code = %q, want %q", payload.Code, "registration_unavailable")
	}
}

func TestAgentHandlerRegisterRejectsMissingBearerToken(t *testing.T) {
	t.Parallel()

	handler := NewAgentHandler("secret-token")

	req := newRegisterRequestWithoutAuth(t, map[string]any{
		"agentId":   "node-local-1",
		"hostname":  "bootstrap-host",
		"ipAddress": "127.0.0.1",
	})
	rr := httptest.NewRecorder()

	handler.Register(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}

	var payload APIError
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v body=%q", err, rr.Body.String())
	}
	if payload.Code != "unauthorized" {
		t.Fatalf("code = %q, want %q", payload.Code, "unauthorized")
	}
}

func TestAgentHandlerRegisterRejectsInvalidBearerToken(t *testing.T) {
	t.Parallel()

	handler := NewAgentHandler("secret-token")

	req := newRegisterRequest(t, "wrong-token", map[string]any{
		"agentId":   "node-local-1",
		"hostname":  "bootstrap-host",
		"ipAddress": "127.0.0.1",
	})
	rr := httptest.NewRecorder()

	handler.Register(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestAgentHandlerRegisterRejectsInvalidJSON(t *testing.T) {
	t.Parallel()

	handler := NewAgentHandler("secret-token")

	req := httptest.NewRequest(http.MethodPost, "/v1/agents/register", bytes.NewBufferString("{invalid"))
	req.Header.Set("Authorization", "Bearer secret-token")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.Register(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}

	var payload APIError
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v body=%q", err, rr.Body.String())
	}
	if payload.Code != "invalid_request" {
		t.Fatalf("code = %q, want %q", payload.Code, "invalid_request")
	}
}

func TestAgentHandlerRegisterRejectsUnknownFields(t *testing.T) {
	t.Parallel()

	handler := NewAgentHandler("secret-token")

	req := newRegisterRequest(t, "secret-token", map[string]any{
		"agentId":   "node-local-1",
		"hostname":  "bootstrap-host",
		"ipAddress": "127.0.0.1",
		"extra":     "nope",
	})
	rr := httptest.NewRecorder()

	handler.Register(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestAgentHandlerRegisterRejectsInvalidPayloadFields(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		body       map[string]any
		wantStatus int
		wantCode   string
	}{
		{
			name: "missing agentId",
			body: map[string]any{
				"hostname":  "bootstrap-host",
				"ipAddress": "127.0.0.1",
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_agent_id",
		},
		{
			name: "invalid agentId format",
			body: map[string]any{
				"agentId":   "x",
				"hostname":  "bootstrap-host",
				"ipAddress": "127.0.0.1",
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_agent_id",
		},
		{
			name: "missing hostname",
			body: map[string]any{
				"agentId":   "node-local-1",
				"ipAddress": "127.0.0.1",
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_hostname",
		},
		{
			name: "invalid hostname",
			body: map[string]any{
				"agentId":   "node-local-1",
				"hostname":  "-bad-host-",
				"ipAddress": "127.0.0.1",
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_hostname",
		},
		{
			name: "missing ipAddress",
			body: map[string]any{
				"agentId":  "node-local-1",
				"hostname": "bootstrap-host",
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_ip_address",
		},
		{
			name: "invalid ipAddress",
			body: map[string]any{
				"agentId":   "node-local-1",
				"hostname":  "bootstrap-host",
				"ipAddress": "not-an-ip",
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_ip_address",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			handler := NewAgentHandler("secret-token")
			req := newRegisterRequest(t, "secret-token", tc.body)
			rr := httptest.NewRecorder()

			handler.Register(rr, req)

			if rr.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rr.Code, tc.wantStatus)
			}

			var payload APIError
			if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
				t.Fatalf("failed to decode response: %v body=%q", err, rr.Body.String())
			}
			if payload.Code != tc.wantCode {
				t.Fatalf("code = %q, want %q", payload.Code, tc.wantCode)
			}
		})
	}
}

func TestAgentHandlerRegisterSuccess(t *testing.T) {
	t.Parallel()

	handler := NewAgentHandler("secret-token")

	req := newRegisterRequest(t, "secret-token", map[string]any{
		"agentId":   "node-local-1",
		"hostname":  "bootstrap-host",
		"ipAddress": "127.0.0.1",
	})
	rr := httptest.NewRecorder()

	handler.Register(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var payload AgentRegisterResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v body=%q", err, rr.Body.String())
	}

	if !payload.Accepted {
		t.Fatalf("accepted = false, want true")
	}
	if payload.NodeID != "node-local-1" {
		t.Fatalf("nodeId = %q, want %q", payload.NodeID, "node-local-1")
	}
	if payload.Message == "" {
		t.Fatalf("message should not be empty")
	}
}

func newRegisterRequest(t *testing.T, token string, body map[string]any) *http.Request {
	t.Helper()

	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/agents/register", bytes.NewReader(raw))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	return req
}

func newRegisterRequestWithoutAuth(t *testing.T, body map[string]any) *http.Request {
	t.Helper()

	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/agents/register", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	return req
}
