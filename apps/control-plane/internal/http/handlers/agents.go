package handlers

import (
	"encoding/json"
	"net"
	"net/http"
	"regexp"
	"strings"
)

var (
	agentIDRegexp  = regexp.MustCompile(`^[a-zA-Z0-9._:-]{3,128}$`)
	hostnameRegexp = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9.-]{0,252}[a-zA-Z0-9]$`)
)

// AgentHandler exposes node-agent registration and heartbeat endpoints.
type AgentHandler struct {
	registrationToken string
}

// NewAgentHandler builds an agent registration handler.
func NewAgentHandler(registrationToken string) *AgentHandler {
	return &AgentHandler{
		registrationToken: strings.TrimSpace(registrationToken),
	}
}

// Register validates node-agent registration requests and returns an acknowledgement.
//
// Security model:
// - Requires Authorization: Bearer <token>
// - Token must match configured registration token
//
// Payload model:
// - agentId: required, constrained format
// - hostname: required, constrained format
// - ipAddress: required, valid IP
func (h *AgentHandler) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use POST")
		return
	}

	if h.registrationToken == "" {
		writeError(w, http.StatusInternalServerError, "registration_unavailable", "registration token is not configured")
		return
	}

	token := bearerToken(r.Header.Get("Authorization"))
	if token == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized", "missing bearer token")
		return
	}
	if token != h.registrationToken {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid bearer token")
		return
	}

	var req AgentRegisterRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
		return
	}

	req.AgentID = strings.TrimSpace(req.AgentID)
	req.Hostname = strings.TrimSpace(req.Hostname)
	req.IPAddress = strings.TrimSpace(req.IPAddress)

	if req.AgentID == "" {
		writeError(w, http.StatusBadRequest, "invalid_agent_id", "agentId is required")
		return
	}
	if !agentIDRegexp.MatchString(req.AgentID) {
		writeError(w, http.StatusBadRequest, "invalid_agent_id", "agentId contains invalid characters")
		return
	}

	if req.Hostname == "" {
		writeError(w, http.StatusBadRequest, "invalid_hostname", "hostname is required")
		return
	}
	if !hostnameRegexp.MatchString(req.Hostname) {
		writeError(w, http.StatusBadRequest, "invalid_hostname", "hostname format is invalid")
		return
	}

	if req.IPAddress == "" {
		writeError(w, http.StatusBadRequest, "invalid_ip_address", "ipAddress is required")
		return
	}
	if net.ParseIP(req.IPAddress) == nil {
		writeError(w, http.StatusBadRequest, "invalid_ip_address", "ipAddress must be a valid IPv4 or IPv6 address")
		return
	}

	writeJSON(w, http.StatusOK, AgentRegisterResponse{
		Accepted: true,
		NodeID:   req.AgentID,
		Message:  "agent registration accepted",
	})
}

func bearerToken(header string) string {
	header = strings.TrimSpace(header)
	if header == "" {
		return ""
	}

	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return ""
	}

	return strings.TrimSpace(strings.TrimPrefix(header, prefix))
}
