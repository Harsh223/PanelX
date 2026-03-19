package handlers

// APIError defines standard error shape for v1 endpoints.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// PrincipalSummary is exposed in auth context responses.
type PrincipalSummary struct {
	SubjectID string   `json:"subjectId"`
	TenantID  string   `json:"tenantId,omitempty"`
	Roles     []string `json:"roles"`
}

// AuthorizeRequest declares a permission check input contract.
type AuthorizeRequest struct {
	Permission string `json:"permission"`
}

// AuthorizeResponse returns authorization verdict for a permission.
type AuthorizeResponse struct {
	Allowed bool `json:"allowed"`
}

// AgentRegisterRequest defines node-agent registration/heartbeat payload.
type AgentRegisterRequest struct {
	AgentID   string `json:"agentId"`
	Hostname  string `json:"hostname"`
	IPAddress string `json:"ipAddress"`
}

// AgentRegisterResponse defines control-plane registration acknowledgement.
type AgentRegisterResponse struct {
	Accepted bool   `json:"accepted"`
	NodeID   string `json:"nodeId"`
	Message  string `json:"message"`
}
