package httpserver

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/Harsh223/PanelX/apps/control-plane/internal/auth"
	"github.com/Harsh223/PanelX/apps/control-plane/internal/config"
	"github.com/Harsh223/PanelX/apps/control-plane/internal/domains"
	"github.com/Harsh223/PanelX/apps/control-plane/internal/filesvc"
	"github.com/Harsh223/PanelX/apps/control-plane/internal/http/handlers"
	"github.com/Harsh223/PanelX/apps/control-plane/internal/installations"
	"github.com/Harsh223/PanelX/apps/control-plane/internal/panelauth"
	"github.com/Harsh223/PanelX/apps/control-plane/internal/provision"
	"github.com/Harsh223/PanelX/apps/control-plane/internal/systemstatus"
)

// Server wraps the HTTP server for the control-plane API.
type Server struct {
	httpServer *http.Server
}

// New constructs an HTTP server with MVP panel routes.
func New(cfg config.Config) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", handlers.Health)

	authHandler := handlers.NewAuthHandler(auth.NewService())
	mux.HandleFunc("/v1/auth/context", authHandler.Context)
	mux.HandleFunc("/v1/auth/authorize", authHandler.Authorize)

	agentHandler := handlers.NewAgentHandler(cfg.RegistrationToken)
	mux.HandleFunc("/v1/agents/register", agentHandler.Register)

	provisionService := provision.NewService(cfg.SitesRoot, cfg.PHPSocketPath, provision.DBAdminConfig{
		Host:     cfg.DBAdminHost,
		Port:     cfg.DBAdminPort,
		User:     cfg.DBAdminUser,
		Password: cfg.DBAdminPassword,
	})
	fileService := filesvc.NewService(cfg.SitesRoot)
	systemStatusService := systemstatus.NewService(cfg.SitesRoot, cfg.PHPSocketPath)

	panelAuthService, err := panelauth.NewService(filepath.Join(cfg.SitesRoot, ".panelx", "panel-auth.json"), 24*time.Hour)
	if err != nil {
		panic(fmt.Errorf("failed to initialize panel auth service: %w", err))
	}

	installStore, err := installations.NewFileStore(filepath.Join(cfg.SitesRoot, ".panelx", "installations.json"))
	if err != nil {
		panic(fmt.Errorf("failed to initialize installation store: %w", err))
	}
	installationsService := installations.NewService(installStore, provisionService)

	domainStore, err := domains.NewFileStore(filepath.Join(cfg.SitesRoot, ".panelx", "domains.json"))
	if err != nil {
		panic(fmt.Errorf("failed to initialize domain store: %w", err))
	}
	domainService, err := domains.NewService(domainStore, domains.ServiceOptions{
		SitesRoot:              cfg.SitesRoot,
		NginxSitesAvailableDir: "/etc/nginx/sites-available",
		NginxSitesEnabledDir:   "/etc/nginx/sites-enabled",
		ManageNginx:            true,
	})
	if err != nil {
		panic(fmt.Errorf("failed to initialize domain service: %w", err))
	}

	panelHandler := handlers.NewPanelHandler(installationsService, fileService, systemStatusService, panelAuthService, cfg.AdminToken).
		SetDomainService(domainService).
		SetNginxLogDir("/var/log/nginx")

	// Public panel-auth routes must always bypass token middleware to avoid first-run lockout.
	panelPublicAPI := http.NewServeMux()
	panelPublicAPI.HandleFunc("/v1/panel/auth/status", panelHandler.PanelAuthStatus)
	panelPublicAPI.HandleFunc("/v1/panel/auth/status/", panelHandler.PanelAuthStatus)
	panelPublicAPI.HandleFunc("/v1/panel/auth/setup", panelHandler.PanelSetup)
	panelPublicAPI.HandleFunc("/v1/panel/auth/setup/", panelHandler.PanelSetup)
	panelPublicAPI.HandleFunc("/v1/panel/auth/login", panelHandler.PanelLogin)
	panelPublicAPI.HandleFunc("/v1/panel/auth/login/", panelHandler.PanelLogin)
	panelPublicAPI.HandleFunc("/v1/panel/auth/logout", panelHandler.PanelLogout)
	panelPublicAPI.HandleFunc("/v1/panel/auth/logout/", panelHandler.PanelLogout)
	panelPublicAPI.HandleFunc("/v1/panel/me", panelHandler.PanelMe)
	panelPublicAPI.HandleFunc("/v1/panel/me/", panelHandler.PanelMe)
	mux.Handle("/v1/panel/", panelPublicAPI)

	api := http.NewServeMux()
	api.HandleFunc("/v1/wordpress/install", panelHandler.InstallWordPress)
	api.HandleFunc("/v1/installations", panelHandler.ListInstallations)
	api.HandleFunc("/v1/system/status", panelHandler.VPSStatus)

	api.HandleFunc("/v1/domains", panelHandler.ListDomains)
	api.HandleFunc("/v1/domains/get", panelHandler.GetDomain)
	api.HandleFunc("/v1/domains/create", panelHandler.CreateDomain)
	api.HandleFunc("/v1/domains/update", panelHandler.UpdateDomain)
	api.HandleFunc("/v1/domains/delete", panelHandler.DeleteDomain)
	api.HandleFunc("/v1/domains/redirects", panelHandler.UpdateDomainRedirects)
	api.HandleFunc("/v1/domains/health", panelHandler.DomainHealth)
	api.HandleFunc("/v1/domains/logs", panelHandler.DomainLogs)
	api.HandleFunc("/v1/domains/ssl/issue", panelHandler.IssueDomainSSL)
	api.HandleFunc("/v1/domains/ssl/renew", panelHandler.RenewDomainSSL)
	api.HandleFunc("/v1/domains/ssl/revoke", panelHandler.RevokeDomainSSL)

	api.HandleFunc("/v1/files/list", panelHandler.ListFiles)
	api.HandleFunc("/v1/files/read", panelHandler.ReadFile)
	api.HandleFunc("/v1/files/write", panelHandler.WriteFile)
	api.HandleFunc("/v1/files/delete", panelHandler.DeleteFile)

	mux.Handle("/v1/", authTokenMiddleware(cfg.AdminToken, panelAuthService, api))
	mux.Handle("/panel", handlers.ServePanel(cfg.WebRoot))
	mux.Handle("/panel/", handlers.ServePanel(cfg.WebRoot))
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir(filepath.Join(cfg.WebRoot, "assets")))))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/panel", http.StatusTemporaryRedirect)
			return
		}
		http.NotFound(w, r)
	})

	httpSrv := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", cfg.HTTP.Host, cfg.HTTP.Port),
		Handler:           loggingMiddleware(mux),
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
	}

	return &Server{httpServer: httpSrv}
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe() error {
	return s.httpServer.ListenAndServe()
}

func authTokenMiddleware(expectedToken string, panelAuthService *panelauth.Service, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimSpace(r.Header.Get("X-PanelX-Token"))
		if token == "" {
			authz := strings.TrimSpace(r.Header.Get("Authorization"))
			if strings.HasPrefix(authz, "Bearer ") {
				token = strings.TrimSpace(strings.TrimPrefix(authz, "Bearer "))
			}
		}

		if token != "" && token == expectedToken {
			next.ServeHTTP(w, r)
			return
		}

		if panelAuthService != nil {
			if c, err := r.Cookie(panelauth.SessionCookieName); err == nil {
				if _, err := panelAuthService.ValidateSession(strings.TrimSpace(c.Value)); err == nil {
					next.ServeHTTP(w, r)
					return
				}
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"code":"unauthorized","message":"invalid or missing admin token/session"}`))
	})
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		_ = start
	})
}
