package handlers

import (
	"bufio"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Harsh223/PanelX/apps/control-plane/internal/domains"
	"github.com/Harsh223/PanelX/apps/control-plane/internal/filesvc"
	"github.com/Harsh223/PanelX/apps/control-plane/internal/installations"
	"github.com/Harsh223/PanelX/apps/control-plane/internal/panelauth"
	"github.com/Harsh223/PanelX/apps/control-plane/internal/provision"
	"github.com/Harsh223/PanelX/apps/control-plane/internal/systemstatus"
)

var domainLogHostnameRegexp = regexp.MustCompile(`^[a-zA-Z0-9.-]+$`)

// PanelHandler exposes MVP control panel APIs.
type PanelHandler struct {
	installationsService *installations.Service
	fileService          *filesvc.Service
	systemStatusService  *systemstatus.Service

	domainService *domains.Service
	nginxLogDir   string

	panelAuthService *panelauth.Service
	bootstrapToken   string
}

// NewPanelHandler builds handler dependencies for panel APIs.
func NewPanelHandler(
	installationsService *installations.Service,
	fileService *filesvc.Service,
	systemStatusService *systemstatus.Service,
	panelAuthService *panelauth.Service,
	bootstrapToken string,
) *PanelHandler {
	return &PanelHandler{
		installationsService: installationsService,
		fileService:          fileService,
		systemStatusService:  systemStatusService,
		panelAuthService:     panelAuthService,
		bootstrapToken:       bootstrapToken,
		nginxLogDir:          "/var/log/nginx",
	}
}

// SetDomainService wires domain API service into handler.
func (h *PanelHandler) SetDomainService(domainService *domains.Service) *PanelHandler {
	h.domainService = domainService
	return h
}

// SetNginxLogDir overrides default nginx log directory.
func (h *PanelHandler) SetNginxLogDir(path string) *PanelHandler {
	path = strings.TrimSpace(path)
	if path != "" {
		h.nginxLogDir = path
	}
	return h
}

// InstallWordPress provisions Nginx+PHP+MariaDB+WordPress for a domain.
func (h *PanelHandler) InstallWordPress(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use POST")
		return
	}

	var req provision.WordPressInstallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}

	result, err := h.installationsService.CreateWordPress(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, "install_failed", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// ListInstallations returns all managed WordPress installations.
func (h *PanelHandler) ListInstallations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use GET")
		return
	}

	items, err := h.installationsService.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_installations_failed", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// VPSStatus returns real-time VPS and service status for the dashboard.
func (h *PanelHandler) VPSStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use GET")
		return
	}

	if h.systemStatusService == nil {
		writeError(w, http.StatusInternalServerError, "system_status_unavailable", "system status service is not configured")
		return
	}

	snapshot := h.systemStatusService.Collect(r.Context())
	writeJSON(w, http.StatusOK, snapshot)
}

// PanelAuthStatus returns panel bootstrap/auth state for UI flow control.
func (h *PanelHandler) PanelAuthStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use GET")
		return
	}
	if h.panelAuthService == nil {
		writeError(w, http.StatusInternalServerError, "panel_auth_unavailable", "panel auth service is not configured")
		return
	}

	profile, configured := h.panelAuthService.Profile()
	writeJSON(w, http.StatusOK, map[string]any{
		"configured": configured,
		"admin":      profile,
	})
}

// PanelSetup initializes admin credentials once using installer bootstrap token.
func (h *PanelHandler) PanelSetup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use POST")
		return
	}
	if h.panelAuthService == nil {
		writeError(w, http.StatusInternalServerError, "panel_auth_unavailable", "panel auth service is not configured")
		return
	}

	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}

	bootstrapToken := bootstrapTokenFromRequest(r)
	profile, err := h.panelAuthService.Setup(h.bootstrapToken, bootstrapToken, req.Username, req.Email, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, panelauth.ErrBootstrapInvalid):
			writeError(w, http.StatusUnauthorized, "invalid_bootstrap_token", err.Error())
		case errors.Is(err, panelauth.ErrAlreadyConfigured):
			writeError(w, http.StatusConflict, "already_configured", err.Error())
		case errors.Is(err, panelauth.ErrInvalidUsername),
			errors.Is(err, panelauth.ErrInvalidEmail),
			errors.Is(err, panelauth.ErrInvalidPassword):
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "setup_failed", err.Error())
		}
		return
	}

	session, err := h.panelAuthService.Login(req.Username, req.Password)
	if err == nil {
		setSessionCookie(w, r, session.Token, session.ExpiresAt)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"configured": true,
		"admin":      profile,
		"message":    "panel admin configured",
	})
}

// PanelLogin authenticates panel admin and returns session cookie.
func (h *PanelHandler) PanelLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use POST")
		return
	}
	if h.panelAuthService == nil {
		writeError(w, http.StatusInternalServerError, "panel_auth_unavailable", "panel auth service is not configured")
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}

	session, err := h.panelAuthService.Login(req.Username, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, panelauth.ErrNotConfigured):
			writeError(w, http.StatusPreconditionFailed, "not_configured", err.Error())
		case errors.Is(err, panelauth.ErrInvalidLogin):
			writeError(w, http.StatusUnauthorized, "invalid_login", err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "login_failed", err.Error())
		}
		return
	}

	setSessionCookie(w, r, session.Token, session.ExpiresAt)
	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": true,
		"expiresAt":     session.ExpiresAt,
	})
}

// PanelLogout invalidates current session cookie.
func (h *PanelHandler) PanelLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use POST")
		return
	}
	if h.panelAuthService == nil {
		writeError(w, http.StatusInternalServerError, "panel_auth_unavailable", "panel auth service is not configured")
		return
	}

	if c, err := r.Cookie(panelauth.SessionCookieName); err == nil {
		h.panelAuthService.Logout(c.Value)
	}
	clearSessionCookie(w, r)

	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": false,
		"message":       "logged out",
	})
}

// PanelMe returns authenticated admin profile from session cookie.
func (h *PanelHandler) PanelMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use GET")
		return
	}
	if h.panelAuthService == nil {
		writeError(w, http.StatusInternalServerError, "panel_auth_unavailable", "panel auth service is not configured")
		return
	}

	cookie, err := r.Cookie(panelauth.SessionCookieName)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized", "missing session")
		return
	}

	session, err := h.panelAuthService.ValidateSession(cookie.Value)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", err.Error())
		return
	}

	profile, ok := h.panelAuthService.Profile()
	if !ok {
		writeError(w, http.StatusPreconditionFailed, "not_configured", "panel admin is not configured")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": true,
		"session": map[string]any{
			"username":  session.Username,
			"expiresAt": session.ExpiresAt,
		},
		"admin": profile,
	})
}

// ListFiles returns directory entries for domain path.
func (h *PanelHandler) ListFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use GET")
		return
	}

	domain := strings.TrimSpace(r.URL.Query().Get("domain"))
	path := strings.TrimSpace(r.URL.Query().Get("path"))
	entries, err := h.fileService.List(domain, path)
	if err != nil {
		writeError(w, http.StatusBadRequest, "list_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries})
}

// ReadFile reads file content for domain path.
func (h *PanelHandler) ReadFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use GET")
		return
	}

	domain := strings.TrimSpace(r.URL.Query().Get("domain"))
	path := strings.TrimSpace(r.URL.Query().Get("path"))
	content, err := h.fileService.Read(domain, path)
	if err != nil {
		writeError(w, http.StatusBadRequest, "read_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"content": content})
}

// WriteFile writes file content for domain path.
func (h *PanelHandler) WriteFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use POST")
		return
	}

	var req struct {
		Domain  string `json:"domain"`
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}

	if err := h.fileService.Write(req.Domain, req.Path, req.Content); err != nil {
		writeError(w, http.StatusBadRequest, "write_failed", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "file written"})
}

// DeleteFile deletes a file or directory for domain path.
func (h *PanelHandler) DeleteFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use POST")
		return
	}

	var req struct {
		Domain string `json:"domain"`
		Path   string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}

	if err := h.fileService.Delete(req.Domain, req.Path); err != nil {
		writeError(w, http.StatusBadRequest, "delete_failed", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "path deleted"})
}

// ListDomains returns all managed domains.
// Query param: refreshHealth=true to recalculate and persist health.
func (h *PanelHandler) ListDomains(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use GET")
		return
	}
	if h.domainService == nil {
		writeError(w, http.StatusInternalServerError, "domain_service_unavailable", "domain service is not configured")
		return
	}

	refreshHealth := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("refreshHealth")), "true")
	var (
		items []domains.Domain
		err   error
	)

	if refreshHealth {
		items, err = h.domainService.RefreshHealthForAll(r.Context())
	} else {
		items, err = h.domainService.List(r.Context())
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_domains_failed", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// GetDomain returns one domain by ID or hostname.
func (h *PanelHandler) GetDomain(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use GET")
		return
	}
	if h.domainService == nil {
		writeError(w, http.StatusInternalServerError, "domain_service_unavailable", "domain service is not configured")
		return
	}

	id := strings.TrimSpace(r.URL.Query().Get("id"))
	hostname := strings.TrimSpace(r.URL.Query().Get("hostname"))

	var (
		item domains.Domain
		ok   bool
		err  error
	)

	switch {
	case id != "":
		item, ok, err = h.domainService.GetByID(r.Context(), id)
	case hostname != "":
		item, ok, err = h.domainService.GetByHostname(r.Context(), hostname)
	default:
		writeError(w, http.StatusBadRequest, "invalid_request", "either id or hostname is required")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get_domain_failed", err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "domain_not_found", "domain not found")
		return
	}

	writeJSON(w, http.StatusOK, item)
}

// CreateDomain creates a managed domain.
func (h *PanelHandler) CreateDomain(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use POST")
		return
	}
	if h.domainService == nil {
		writeError(w, http.StatusInternalServerError, "domain_service_unavailable", "domain service is not configured")
		return
	}

	var req domains.DomainCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}

	item, err := h.domainService.Create(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, "create_domain_failed", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, item)
}

// UpdateDomain updates mutable fields for a domain.
func (h *PanelHandler) UpdateDomain(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use POST")
		return
	}
	if h.domainService == nil {
		writeError(w, http.StatusInternalServerError, "domain_service_unavailable", "domain service is not configured")
		return
	}

	var req struct {
		ID string `json:"id"`
		domains.DomainUpdateRequest
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}

	item, err := h.domainService.Update(r.Context(), req.ID, req.DomainUpdateRequest)
	if err != nil {
		writeError(w, http.StatusBadRequest, "update_domain_failed", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, item)
}

// DeleteDomain deletes a managed domain.
func (h *PanelHandler) DeleteDomain(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use POST")
		return
	}
	if h.domainService == nil {
		writeError(w, http.StatusInternalServerError, "domain_service_unavailable", "domain service is not configured")
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}
	if strings.TrimSpace(req.ID) == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "id is required")
		return
	}

	if err := h.domainService.Delete(r.Context(), req.ID); err != nil {
		writeError(w, http.StatusBadRequest, "delete_domain_failed", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"deleted": true,
		"id":      req.ID,
	})
}

// UpdateDomainRedirects updates per-domain redirect and canonical policy.
func (h *PanelHandler) UpdateDomainRedirects(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use POST")
		return
	}
	if h.domainService == nil {
		writeError(w, http.StatusInternalServerError, "domain_service_unavailable", "domain service is not configured")
		return
	}

	var req struct {
		ID string `json:"id"`
		domains.RedirectUpdateRequest
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}

	item, err := h.domainService.UpdateRedirects(r.Context(), req.ID, req.RedirectUpdateRequest)
	if err != nil {
		writeError(w, http.StatusBadRequest, "update_redirects_failed", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, item)
}

// IssueDomainSSL issues certificate for a domain.
func (h *PanelHandler) IssueDomainSSL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use POST")
		return
	}
	if h.domainService == nil {
		writeError(w, http.StatusInternalServerError, "domain_service_unavailable", "domain service is not configured")
		return
	}

	var req struct {
		ID       string              `json:"id"`
		Email    string              `json:"email"`
		Provider domains.SSLProvider `json:"provider"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}

	item, err := h.domainService.IssueSSL(r.Context(), req.ID, domains.SSLIssueRequest{
		Email:    req.Email,
		Provider: req.Provider,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, "ssl_issue_failed", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, item)
}

// RenewDomainSSL renews certificate for a domain.
func (h *PanelHandler) RenewDomainSSL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use POST")
		return
	}
	if h.domainService == nil {
		writeError(w, http.StatusInternalServerError, "domain_service_unavailable", "domain service is not configured")
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}

	item, err := h.domainService.RenewSSL(r.Context(), req.ID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "ssl_renew_failed", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, item)
}

// RevokeDomainSSL revokes certificate for a domain.
func (h *PanelHandler) RevokeDomainSSL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use POST")
		return
	}
	if h.domainService == nil {
		writeError(w, http.StatusInternalServerError, "domain_service_unavailable", "domain service is not configured")
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}

	item, err := h.domainService.RevokeSSL(r.Context(), req.ID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "ssl_revoke_failed", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, item)
}

// DomainHealth runs health check for one domain (id) or all domains (all=true).
func (h *PanelHandler) DomainHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use GET or POST")
		return
	}
	if h.domainService == nil {
		writeError(w, http.StatusInternalServerError, "domain_service_unavailable", "domain service is not configured")
		return
	}

	all := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("all")), "true")
	id := strings.TrimSpace(r.URL.Query().Get("id"))

	if r.Method == http.MethodPost {
		var req struct {
			ID  string `json:"id"`
			All bool   `json:"all"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			if strings.TrimSpace(req.ID) != "" {
				id = strings.TrimSpace(req.ID)
			}
			if req.All {
				all = true
			}
		}
	}

	if all {
		items, err := h.domainService.RefreshHealthForAll(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "refresh_health_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
		return
	}

	if id == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "id is required unless all=true")
		return
	}

	item, err := h.domainService.RunHealthCheck(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusBadRequest, "health_check_failed", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, item)
}

// DomainLogs reads per-domain nginx logs.
// Query params:
// - id or domain
// - type: access|error (default access)
// - lines: 1..5000 (default 200)
func (h *PanelHandler) DomainLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use GET")
		return
	}

	id := strings.TrimSpace(r.URL.Query().Get("id"))
	domainParam := strings.TrimSpace(r.URL.Query().Get("domain"))
	logType := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("type")))
	if logType == "" {
		logType = "access"
	}
	if logType != "access" && logType != "error" {
		writeError(w, http.StatusBadRequest, "invalid_request", "type must be access or error")
		return
	}

	lines := 200
	if raw := strings.TrimSpace(r.URL.Query().Get("lines")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			writeError(w, http.StatusBadRequest, "invalid_request", "lines must be a positive integer")
			return
		}
		if parsed > 5000 {
			parsed = 5000
		}
		lines = parsed
	}

	hostname := domainParam
	if id != "" {
		if h.domainService == nil {
			writeError(w, http.StatusInternalServerError, "domain_service_unavailable", "domain service is required when id is provided")
			return
		}
		item, ok, err := h.domainService.GetByID(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "get_domain_failed", err.Error())
			return
		}
		if !ok {
			writeError(w, http.StatusNotFound, "domain_not_found", "domain not found")
			return
		}
		hostname = item.Hostname
	}

	hostname = strings.TrimSpace(hostname)
	if hostname == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "domain or id is required")
		return
	}
	if !domainLogHostnameRegexp.MatchString(hostname) {
		writeError(w, http.StatusBadRequest, "invalid_request", "domain contains invalid characters")
		return
	}

	suffix := ".access.log"
	if logType == "error" {
		suffix = ".error.log"
	}
	logPath := filepath.Join(h.nginxLogDir, hostname+suffix)

	linesOut, err := readLastLines(logPath, lines)
	if err != nil {
		writeError(w, http.StatusBadRequest, "read_logs_failed", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"domain": hostname,
		"type":   logType,
		"path":   logPath,
		"lines":  linesOut,
		"count":  len(linesOut),
	})
}

func bootstrapTokenFromRequest(r *http.Request) string {
	token := strings.TrimSpace(r.Header.Get("X-PanelX-Token"))
	if token != "" {
		return token
	}

	authz := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(authz, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(authz, "Bearer "))
	}
	return ""
}

func setSessionCookie(w http.ResponseWriter, r *http.Request, token string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     panelauth.SessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
		Expires:  expiresAt.UTC(),
		MaxAge:   int(time.Until(expiresAt).Seconds()),
	})
}

func clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     panelauth.SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(0, 0).UTC(),
		MaxAge:   -1,
	})
}

// ServePanel serves built web panel static assets.
func ServePanel(webRoot string) http.Handler {
	if webRoot == "" {
		webRoot = "/opt/panelx/web"
	}

	fs := http.FileServer(http.Dir(webRoot))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath := strings.TrimPrefix(r.URL.Path, "/panel")
		if requestPath == "" || requestPath == "/" {
			http.ServeFile(w, r, filepath.Join(webRoot, "index.html"))
			return
		}

		relPath := strings.TrimPrefix(filepath.Clean("/"+requestPath), "/")
		candidate := filepath.Join(webRoot, relPath)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/" + relPath
			fs.ServeHTTP(w, r2)
			return
		}

		http.ServeFile(w, r, filepath.Join(webRoot, "index.html"))
	})
}

func readLastLines(path string, maxLines int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if maxLines < 1 {
		maxLines = 1
	}

	lines := make([]string, 0, maxLines)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if len(lines) < maxLines {
			lines = append(lines, line)
			continue
		}
		copy(lines, lines[1:])
		lines[len(lines)-1] = line
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}
