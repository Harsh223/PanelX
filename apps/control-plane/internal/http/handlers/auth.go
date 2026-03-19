package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Harsh223/PanelX/apps/control-plane/internal/auth"
)

// AuthHandler exposes bootstrap auth and authorization contract endpoints.
type AuthHandler struct {
	authService *auth.Service
}

// NewAuthHandler builds an auth contract handler set.
func NewAuthHandler(authService *auth.Service) *AuthHandler {
	return &AuthHandler{authService: authService}
}

// Context returns a bootstrap principal summary from request headers.
func (h *AuthHandler) Context(w http.ResponseWriter, r *http.Request) {
	principal, err := h.resolvePrincipalFromHeaders(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_principal", err.Error())
		return
	}

	roles := make([]string, 0, len(principal.Roles))
	for _, role := range principal.Roles {
		roles = append(roles, string(role))
	}

	writeJSON(w, http.StatusOK, PrincipalSummary{
		SubjectID: principal.SubjectID,
		TenantID:  principal.TenantID,
		Roles:     roles,
	})
}

// Authorize evaluates a permission against resolved principal context.
func (h *AuthHandler) Authorize(w http.ResponseWriter, r *http.Request) {
	principal, err := h.resolvePrincipalFromHeaders(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_principal", err.Error())
		return
	}

	var req AuthorizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
		return
	}

	permission := auth.Permission(strings.TrimSpace(req.Permission))
	if permission == "" {
		writeError(w, http.StatusBadRequest, "invalid_permission", "permission is required")
		return
	}

	err = h.authService.Authorize(principal, permission)
	writeJSON(w, http.StatusOK, AuthorizeResponse{Allowed: err == nil})
}

func (h *AuthHandler) resolvePrincipalFromHeaders(r *http.Request) (auth.Principal, error) {
	subjectID := strings.TrimSpace(r.Header.Get("X-Subject-ID"))
	tenantID := strings.TrimSpace(r.Header.Get("X-Tenant-ID"))
	rawRoles := strings.TrimSpace(r.Header.Get("X-Roles"))

	roles := make([]auth.Role, 0)
	if rawRoles != "" {
		for _, role := range strings.Split(rawRoles, ",") {
			trimmed := strings.TrimSpace(role)
			if trimmed == "" {
				continue
			}
			roles = append(roles, auth.Role(trimmed))
		}
	}

	if len(roles) == 0 {
		roles = append(roles, auth.RoleTenantViewer)
	}

	return h.authService.BuildPrincipal(subjectID, tenantID, roles)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, APIError{Code: code, Message: message})
}
