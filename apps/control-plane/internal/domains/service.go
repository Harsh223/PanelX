package domains

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// CommandRunner abstracts shell command execution for testability.
type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) error
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("command %s %v failed: %w output=%s", name, args, err, strings.TrimSpace(string(output)))
	}
	return nil
}

// DomainCreateRequest is used for domain creation flows.
type DomainCreateRequest struct {
	Hostname     string `json:"hostname"`
	DocumentRoot string `json:"documentRoot"`
	OwnerLabel   string `json:"ownerLabel"`
	Notes        string `json:"notes"`
	EnableNginx  bool   `json:"enableNginx"`
}

// DomainUpdateRequest is used for mutable domain fields.
type DomainUpdateRequest struct {
	DocumentRoot string `json:"documentRoot"`
	OwnerLabel   string `json:"ownerLabel"`
	Notes        string `json:"notes"`
	Status       Status `json:"status"`
}

// RedirectUpdateRequest configures canonical and redirect behavior.
type RedirectUpdateRequest struct {
	Mode            RedirectMode  `json:"mode"`
	CanonicalHost   CanonicalHost `json:"canonicalHost"`
	CanonicalTarget string        `json:"canonicalTarget"`
	CustomTargetURL string        `json:"customTargetUrl"`
	PreservePath    bool          `json:"preservePath"`
	PreserveQuery   bool          `json:"preserveQuery"`
	Temporary       bool          `json:"temporary"`
}

// SSLIssueRequest represents an SSL issuance action.
type SSLIssueRequest struct {
	Email    string      `json:"email"`
	Provider SSLProvider `json:"provider"`
}

// ServiceOptions controls runtime behavior for the domain service.
type ServiceOptions struct {
	SitesRoot              string
	NginxSitesAvailableDir string
	NginxSitesEnabledDir   string
	ManageNginx            bool
	Runner                 CommandRunner
	Now                    func() time.Time
}

// Service implements domain CRUD, health checks, redirect policy updates,
// and SSL action workflows.
type Service struct {
	store Store

	sitesRoot            string
	nginxSitesAvailable  string
	nginxSitesEnabled    string
	manageNginx          bool
	runner               CommandRunner
	now                  func() time.Time
	hostnameValidator    *regexp.Regexp
	discoverServerIPsFn  func(context.Context) ([]string, error)
	resolveHostnameIPsFn func(context.Context, string) ([]string, error)
}

// NewService builds a domain service with production-safe defaults.
func NewService(store Store, opts ServiceOptions) (*Service, error) {
	if store == nil {
		return nil, errors.New("domain store is required")
	}

	nowFn := opts.Now
	if nowFn == nil {
		nowFn = func() time.Time { return time.Now().UTC() }
	}

	runner := opts.Runner
	if runner == nil {
		runner = execRunner{}
	}

	sitesRoot := strings.TrimSpace(opts.SitesRoot)
	if sitesRoot == "" {
		sitesRoot = "/var/www/panelx/sites"
	}

	nginxAvail := strings.TrimSpace(opts.NginxSitesAvailableDir)
	if nginxAvail == "" {
		nginxAvail = "/etc/nginx/sites-available"
	}
	nginxEnabled := strings.TrimSpace(opts.NginxSitesEnabledDir)
	if nginxEnabled == "" {
		nginxEnabled = "/etc/nginx/sites-enabled"
	}

	svc := &Service{
		store:                store,
		sitesRoot:            sitesRoot,
		nginxSitesAvailable:  nginxAvail,
		nginxSitesEnabled:    nginxEnabled,
		manageNginx:          opts.ManageNginx,
		runner:               runner,
		now:                  nowFn,
		hostnameValidator:    regexp.MustCompile(`^[a-z0-9][a-z0-9.-]*[a-z0-9]$`),
		discoverServerIPsFn:  discoverServerIPs,
		resolveHostnameIPsFn: resolveHostnameIPs,
	}

	return svc, nil
}

// Create creates and persists a domain, optionally syncing Nginx.
func (s *Service) Create(ctx context.Context, req DomainCreateRequest) (Domain, error) {
	hostname, err := s.normalizeAndValidateHostname(req.Hostname)
	if err != nil {
		return Domain{}, err
	}

	if _, exists, err := s.store.GetByHostname(hostname); err != nil {
		return Domain{}, err
	} else if exists {
		return Domain{}, fmt.Errorf("domain %s already exists", hostname)
	}

	docRoot, err := s.normalizeDocumentRoot(hostname, req.DocumentRoot)
	if err != nil {
		return Domain{}, err
	}

	now := s.now()
	rootDomain, isSub := splitRootDomain(hostname)

	domain := Domain{
		ID:           newDomainID(now),
		Hostname:     hostname,
		RootDomain:   rootDomain,
		IsSubdomain:  isSub,
		DocumentRoot: docRoot,
		OwnerLabel:   strings.TrimSpace(req.OwnerLabel),
		Notes:        strings.TrimSpace(req.Notes),
		Status:       StatusPending,
		TLS: TLSProfile{
			Enabled:   false,
			Provider:  SSLProviderLetsEncrypt,
			Status:    SSLStatusNone,
			AutoRenew: true,
		},
		Redirects: RedirectPolicy{
			Mode:          RedirectModeOff,
			CanonicalHost: CanonicalNone,
			PreservePath:  true,
			PreserveQuery: true,
		},
		Health: HealthSummary{
			LastCheckedAt: now,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.store.Create(domain); err != nil {
		return Domain{}, err
	}

	shouldManageNginx := s.manageNginx || req.EnableNginx
	if shouldManageNginx {
		if err := s.syncNginxConfig(ctx, domain); err != nil {
			return Domain{}, err
		}
	}

	updated, err := s.RunHealthCheck(ctx, domain.ID)
	if err != nil {
		return Domain{}, err
	}

	return updated, nil
}

// List returns all managed domains.
func (s *Service) List(_ context.Context) ([]Domain, error) {
	return s.store.List()
}

// GetByID returns one domain by ID.
func (s *Service) GetByID(_ context.Context, id string) (Domain, bool, error) {
	return s.store.GetByID(strings.TrimSpace(id))
}

// GetByHostname returns one domain by hostname.
func (s *Service) GetByHostname(_ context.Context, hostname string) (Domain, bool, error) {
	return s.store.GetByHostname(strings.TrimSpace(hostname))
}

// Update updates mutable fields for a domain.
func (s *Service) Update(ctx context.Context, id string, req DomainUpdateRequest) (Domain, error) {
	domain, err := s.mustGet(id)
	if err != nil {
		return Domain{}, err
	}

	if strings.TrimSpace(req.DocumentRoot) != "" {
		docRoot, err := s.normalizeDocumentRoot(domain.Hostname, req.DocumentRoot)
		if err != nil {
			return Domain{}, err
		}
		domain.DocumentRoot = docRoot
	}
	if strings.TrimSpace(req.OwnerLabel) != "" {
		domain.OwnerLabel = strings.TrimSpace(req.OwnerLabel)
	}
	if strings.TrimSpace(req.Notes) != "" {
		domain.Notes = strings.TrimSpace(req.Notes)
	}
	if req.Status != "" {
		domain.Status = req.Status
	}

	domain.UpdatedAt = s.now()

	if err := s.store.Update(domain); err != nil {
		return Domain{}, err
	}

	if s.manageNginx {
		if err := s.syncNginxConfig(ctx, domain); err != nil {
			return Domain{}, err
		}
	}

	return domain, nil
}

// Delete removes a domain and its managed Nginx mapping (if enabled).
func (s *Service) Delete(ctx context.Context, id string) error {
	domain, err := s.mustGet(id)
	if err != nil {
		return err
	}

	if s.manageNginx {
		if err := s.removeNginxConfig(ctx, domain.Hostname); err != nil {
			return err
		}
	}

	return s.store.Delete(domain.ID)
}

// UpdateRedirects updates redirect/canonical policy and syncs Nginx.
func (s *Service) UpdateRedirects(ctx context.Context, id string, req RedirectUpdateRequest) (Domain, error) {
	domain, err := s.mustGet(id)
	if err != nil {
		return Domain{}, err
	}

	policy, err := s.validateRedirectPolicy(req)
	if err != nil {
		return Domain{}, err
	}

	domain.Redirects = policy
	domain.UpdatedAt = s.now()

	if err := s.store.Update(domain); err != nil {
		return Domain{}, err
	}

	if s.manageNginx {
		if err := s.syncNginxConfig(ctx, domain); err != nil {
			return Domain{}, err
		}
	}

	return domain, nil
}

// IssueSSL attempts certificate issuance and updates TLS status.
func (s *Service) IssueSSL(ctx context.Context, id string, req SSLIssueRequest) (Domain, error) {
	domain, err := s.mustGet(id)
	if err != nil {
		return Domain{}, err
	}

	email := strings.TrimSpace(req.Email)
	if email == "" || !strings.Contains(email, "@") {
		return Domain{}, errors.New("valid email is required for SSL issuance")
	}

	provider := req.Provider
	if provider == "" {
		provider = SSLProviderLetsEncrypt
	}
	if provider != SSLProviderLetsEncrypt && provider != SSLProviderManual {
		return Domain{}, fmt.Errorf("unsupported SSL provider: %s", provider)
	}

	now := s.now()
	domain.TLS.Provider = provider
	domain.TLS.Enabled = true
	domain.TLS.Status = SSLStatusPending
	domain.TLS.LastAttemptAt = &now
	domain.TLS.LastError = ""
	domain.TLS.LastErrorCode = ""
	domain.UpdatedAt = now

	if err := s.store.Update(domain); err != nil {
		return Domain{}, err
	}

	if provider == SSLProviderLetsEncrypt {
		if err := s.runner.Run(ctx, "certbot",
			"--nginx",
			"--non-interactive",
			"--agree-tos",
			"-m", email,
			"-d", domain.Hostname,
			"--redirect",
		); err != nil {
			domain.TLS.Status = SSLStatusFailed
			domain.TLS.LastError = err.Error()
			domain.TLS.LastErrorCode = "certbot_issue_failed"
			domain.TLS.Enabled = false
			domain.UpdatedAt = s.now()
			_ = s.store.Update(domain)
			return Domain{}, err
		}
	}

	issuedAt := s.now()
	expiresAt := issuedAt.Add(90 * 24 * time.Hour)
	domain.TLS.Enabled = true
	domain.TLS.AutoRenew = true
	domain.TLS.Status = SSLStatusIssued
	domain.TLS.IssuedAt = &issuedAt
	domain.TLS.ExpiresAt = &expiresAt
	domain.TLS.LastError = ""
	domain.TLS.LastErrorCode = ""
	domain.UpdatedAt = issuedAt

	if err := s.store.Update(domain); err != nil {
		return Domain{}, err
	}

	if s.manageNginx {
		if err := s.syncNginxConfig(ctx, domain); err != nil {
			return Domain{}, err
		}
	}

	return domain, nil
}

// RenewSSL attempts certificate renewal and updates TLS metadata.
func (s *Service) RenewSSL(ctx context.Context, id string) (Domain, error) {
	domain, err := s.mustGet(id)
	if err != nil {
		return Domain{}, err
	}

	now := s.now()
	domain.TLS.LastAttemptAt = &now
	domain.UpdatedAt = now

	if err := s.store.Update(domain); err != nil {
		return Domain{}, err
	}

	if domain.TLS.Provider == SSLProviderLetsEncrypt {
		if err := s.runner.Run(ctx, "certbot", "renew", "--cert-name", domain.Hostname, "--non-interactive"); err != nil {
			domain.TLS.Status = SSLStatusFailed
			domain.TLS.LastError = err.Error()
			domain.TLS.LastErrorCode = "certbot_renew_failed"
			domain.UpdatedAt = s.now()
			_ = s.store.Update(domain)
			return Domain{}, err
		}
	}

	renewedAt := s.now()
	expiresAt := renewedAt.Add(90 * 24 * time.Hour)
	domain.TLS.Enabled = true
	domain.TLS.Status = SSLStatusIssued
	domain.TLS.IssuedAt = &renewedAt
	domain.TLS.ExpiresAt = &expiresAt
	domain.TLS.LastError = ""
	domain.TLS.LastErrorCode = ""
	domain.UpdatedAt = renewedAt

	if err := s.store.Update(domain); err != nil {
		return Domain{}, err
	}

	return domain, nil
}

// RevokeSSL revokes certificate state. For Let's Encrypt, certbot revoke is invoked.
func (s *Service) RevokeSSL(ctx context.Context, id string) (Domain, error) {
	domain, err := s.mustGet(id)
	if err != nil {
		return Domain{}, err
	}

	if domain.TLS.Provider == SSLProviderLetsEncrypt && domain.TLS.Enabled {
		certPath := filepath.Join("/etc/letsencrypt/live", domain.Hostname, "cert.pem")
		_ = s.runner.Run(ctx, "certbot", "revoke", "--non-interactive", "--cert-path", certPath)
	}

	now := s.now()
	domain.TLS.Enabled = false
	domain.TLS.Status = SSLStatusRevoked
	domain.TLS.LastAttemptAt = &now
	domain.TLS.LastError = ""
	domain.TLS.LastErrorCode = ""
	domain.UpdatedAt = now

	if err := s.store.Update(domain); err != nil {
		return Domain{}, err
	}

	if s.manageNginx {
		if err := s.syncNginxConfig(ctx, domain); err != nil {
			return Domain{}, err
		}
	}

	return domain, nil
}

// RunHealthCheck validates DNS and Nginx mapping for one domain.
func (s *Service) RunHealthCheck(ctx context.Context, id string) (Domain, error) {
	domain, err := s.mustGet(id)
	if err != nil {
		return Domain{}, err
	}

	serverIPs, err := s.discoverServerIPsFn(ctx)
	if err != nil {
		serverIPs = nil
	}

	resolvedIPs, resolveErr := s.resolveHostnameIPsFn(ctx, domain.Hostname)

	availableConf := filepath.Join(s.nginxSitesAvailable, s.domainConfName(domain.Hostname))
	enabledConf := filepath.Join(s.nginxSitesEnabled, s.domainConfName(domain.Hostname))

	_, availErr := os.Stat(availableConf)
	_, enabledErr := os.Stat(enabledConf)

	health := HealthSummary{
		ResolvedIPs:        resolvedIPs,
		ResolvesToServerIP: anyIPMatches(serverIPs, resolvedIPs),
		NginxConfigPresent: availErr == nil,
		NginxEnabled:       enabledErr == nil,
		LastCheckedAt:      s.now(),
	}

	if resolveErr != nil {
		health.Warnings = append(health.Warnings, "dns_lookup_failed: "+resolveErr.Error())
	}
	if !health.ResolvesToServerIP && len(serverIPs) > 0 {
		health.Warnings = append(health.Warnings, "domain_does_not_resolve_to_server_ip")
	}
	if !health.NginxConfigPresent {
		health.Warnings = append(health.Warnings, "nginx_config_missing")
	}
	if !health.NginxEnabled {
		health.Warnings = append(health.Warnings, "nginx_site_not_enabled")
	}

	domain.Health = health
	if len(health.Warnings) == 0 {
		domain.Status = StatusActive
	} else {
		domain.Status = StatusDegraded
	}
	domain.UpdatedAt = s.now()

	if err := s.store.Update(domain); err != nil {
		return Domain{}, err
	}
	return domain, nil
}

// RefreshHealthForAll runs health checks for all domains and returns the updated results.
func (s *Service) RefreshHealthForAll(ctx context.Context) ([]Domain, error) {
	items, err := s.store.List()
	if err != nil {
		return nil, err
	}

	out := make([]Domain, 0, len(items))
	for _, d := range items {
		item, checkErr := s.RunHealthCheck(ctx, d.ID)
		if checkErr != nil {
			return nil, checkErr
		}
		out = append(out, item)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].Hostname < out[j].Hostname
		}
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})

	return out, nil
}

func (s *Service) mustGet(id string) (Domain, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Domain{}, errors.New("domain id is required")
	}
	item, ok, err := s.store.GetByID(id)
	if err != nil {
		return Domain{}, err
	}
	if !ok {
		return Domain{}, fmt.Errorf("domain with id %s not found", id)
	}
	return item, nil
}

func (s *Service) normalizeAndValidateHostname(hostname string) (string, error) {
	host := strings.ToLower(strings.TrimSpace(hostname))
	host = strings.TrimSuffix(host, ".")
	if host == "" {
		return "", errors.New("hostname is required")
	}
	if strings.Contains(host, " ") {
		return "", errors.New("hostname cannot contain spaces")
	}
	if !s.hostnameValidator.MatchString(host) {
		return "", errors.New("hostname contains invalid characters")
	}
	if strings.Count(host, ".") < 1 {
		return "", errors.New("hostname must include at least one dot")
	}
	labels := strings.Split(host, ".")
	for _, label := range labels {
		if label == "" || len(label) > 63 {
			return "", errors.New("hostname contains invalid DNS labels")
		}
		if strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return "", errors.New("hostname labels cannot start or end with dash")
		}
	}
	return host, nil
}

func (s *Service) normalizeDocumentRoot(hostname, input string) (string, error) {
	docRoot := strings.TrimSpace(input)
	if docRoot == "" {
		docRoot = filepath.Join(s.sitesRoot, hostname, "public_html")
	}
	if !filepath.IsAbs(docRoot) {
		return "", errors.New("documentRoot must be an absolute path")
	}
	clean := filepath.Clean(docRoot)
	if clean == "/" {
		return "", errors.New("documentRoot cannot be filesystem root")
	}
	return clean, nil
}

func splitRootDomain(hostname string) (rootDomain string, isSubdomain bool) {
	parts := strings.Split(hostname, ".")
	if len(parts) <= 2 {
		return hostname, false
	}
	return strings.Join(parts[len(parts)-2:], "."), true
}

func (s *Service) validateRedirectPolicy(req RedirectUpdateRequest) (RedirectPolicy, error) {
	mode := req.Mode
	if mode == "" {
		mode = RedirectModeOff
	}
	switch mode {
	case RedirectModeOff, RedirectModeHTTPToTLS, RedirectModeCustom:
	default:
		return RedirectPolicy{}, fmt.Errorf("unsupported redirect mode: %s", mode)
	}

	canonical := req.CanonicalHost
	if canonical == "" {
		canonical = CanonicalNone
	}
	switch canonical {
	case CanonicalNone, CanonicalRoot, CanonicalWWW, CanonicalCustom:
	default:
		return RedirectPolicy{}, fmt.Errorf("unsupported canonical host: %s", canonical)
	}

	customTarget := strings.TrimSpace(req.CustomTargetURL)
	if mode == RedirectModeCustom {
		if customTarget == "" {
			return RedirectPolicy{}, errors.New("customTargetUrl is required when mode=custom")
		}
		if _, err := url.ParseRequestURI(customTarget); err != nil {
			return RedirectPolicy{}, errors.New("customTargetUrl must be a valid URL")
		}
	}

	canonicalTarget := strings.ToLower(strings.TrimSpace(req.CanonicalTarget))
	if canonical == CanonicalCustom {
		if canonicalTarget == "" {
			return RedirectPolicy{}, errors.New("canonicalTarget is required when canonicalHost=custom")
		}
	}

	return RedirectPolicy{
		Mode:            mode,
		CanonicalHost:   canonical,
		CanonicalTarget: canonicalTarget,
		CustomTargetURL: customTarget,
		PreservePath:    req.PreservePath,
		PreserveQuery:   req.PreserveQuery,
		Temporary:       req.Temporary,
	}, nil
}

func (s *Service) syncNginxConfig(ctx context.Context, d Domain) error {
	if !s.manageNginx {
		return nil
	}

	if err := os.MkdirAll(s.nginxSitesAvailable, 0o755); err != nil {
		return fmt.Errorf("create nginx sites-available: %w", err)
	}
	if err := os.MkdirAll(s.nginxSitesEnabled, 0o755); err != nil {
		return fmt.Errorf("create nginx sites-enabled: %w", err)
	}

	confName := s.domainConfName(d.Hostname)
	available := filepath.Join(s.nginxSitesAvailable, confName)
	enabled := filepath.Join(s.nginxSitesEnabled, confName)

	content := s.buildNginxConfig(d)
	if err := os.WriteFile(available, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write nginx domain config: %w", err)
	}

	_ = os.Remove(enabled)
	if err := os.Symlink(available, enabled); err != nil {
		return fmt.Errorf("enable nginx domain config: %w", err)
	}

	if err := s.runner.Run(ctx, "nginx", "-t"); err != nil {
		return err
	}
	if err := s.runner.Run(ctx, "systemctl", "reload", "nginx"); err != nil {
		return err
	}

	now := s.now()
	d.LastSyncedAt = &now
	d.UpdatedAt = now
	if err := s.store.Update(d); err != nil {
		return err
	}
	return nil
}

func (s *Service) removeNginxConfig(ctx context.Context, hostname string) error {
	confName := s.domainConfName(hostname)
	available := filepath.Join(s.nginxSitesAvailable, confName)
	enabled := filepath.Join(s.nginxSitesEnabled, confName)

	_ = os.Remove(enabled)
	_ = os.Remove(available)

	if s.manageNginx {
		if err := s.runner.Run(ctx, "nginx", "-t"); err != nil {
			return err
		}
		if err := s.runner.Run(ctx, "systemctl", "reload", "nginx"); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) domainConfName(hostname string) string {
	return "panelx-domain-" + strings.ReplaceAll(hostname, "*", "_") + ".conf"
}

func (s *Service) buildNginxConfig(d Domain) string {
	redirectCode := "301"
	if d.Redirects.Temporary {
		redirectCode = "302"
	}

	lines := []string{
		"server {",
		"    listen 80;",
		"    listen [::]:80;",
		fmt.Sprintf("    server_name %s;", d.Hostname),
		fmt.Sprintf("    root %s;", d.DocumentRoot),
		"    index index.php index.html index.htm;",
		"    client_max_body_size 128M;",
	}

	switch d.Redirects.Mode {
	case RedirectModeHTTPToTLS:
		lines = append(lines, fmt.Sprintf("    return %s https://$host$request_uri;", redirectCode))
	case RedirectModeCustom:
		target := d.Redirects.CustomTargetURL
		if d.Redirects.PreservePath || d.Redirects.PreserveQuery {
			target = strings.TrimRight(target, "/")
			suffix := ""
			if d.Redirects.PreservePath {
				suffix += "$request_uri"
			}
			if !d.Redirects.PreservePath && d.Redirects.PreserveQuery {
				suffix += "?$query_string"
			}
			target += suffix
		}
		lines = append(lines, fmt.Sprintf("    return %s %s;", redirectCode, target))
	default:
		lines = append(lines,
			"    location / {",
			"        try_files $uri $uri/ /index.php?$args;",
			"    }",
		)
	}

	lines = append(lines,
		"    location ~ \\.php$ {",
		"        include snippets/fastcgi-php.conf;",
		"        fastcgi_pass unix:/run/php/php-fpm.sock;",
		"    }",
		"    location ~ /\\.ht {",
		"        deny all;",
		"    }",
		"}",
	)

	// Optional TLS server block for issued certs.
	if d.TLS.Enabled && (d.TLS.Status == SSLStatusIssued || d.TLS.Status == SSLStatusExpiring) {
		lines = append(lines,
			"",
			"server {",
			"    listen 443 ssl http2;",
			"    listen [::]:443 ssl http2;",
			fmt.Sprintf("    server_name %s;", d.Hostname),
			fmt.Sprintf("    root %s;", d.DocumentRoot),
			"    index index.php index.html index.htm;",
			fmt.Sprintf("    ssl_certificate /etc/letsencrypt/live/%s/fullchain.pem;", d.Hostname),
			fmt.Sprintf("    ssl_certificate_key /etc/letsencrypt/live/%s/privkey.pem;", d.Hostname),
			"    client_max_body_size 128M;",
			"    location / {",
			"        try_files $uri $uri/ /index.php?$args;",
			"    }",
			"    location ~ \\.php$ {",
			"        include snippets/fastcgi-php.conf;",
			"        fastcgi_pass unix:/run/php/php-fpm.sock;",
			"    }",
			"}",
		)
	}

	return strings.Join(lines, "\n") + "\n"
}

func discoverServerIPs(_ context.Context) ([]string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	ips := make([]string, 0, 8)
	seen := map[string]struct{}{}

	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ip, _, err := net.ParseCIDR(addr.String())
			if err != nil || ip == nil {
				continue
			}
			if ip.IsLoopback() {
				continue
			}
			v4 := ip.To4()
			if v4 == nil {
				continue
			}
			s := v4.String()
			if _, ok := seen[s]; ok {
				continue
			}
			seen[s] = struct{}{}
			ips = append(ips, s)
		}
	}

	sort.Strings(ips)
	return ips, nil
}

func resolveHostnameIPs(_ context.Context, hostname string) ([]string, error) {
	addrs, err := net.LookupIP(hostname)
	if err != nil {
		return nil, err
	}

	out := make([]string, 0, len(addrs))
	seen := map[string]struct{}{}
	for _, ip := range addrs {
		if ip == nil {
			continue
		}
		s := ip.String()
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}

	sort.Strings(out)
	return out, nil
}

func anyIPMatches(serverIPs, resolved []string) bool {
	if len(resolved) == 0 {
		return false
	}
	if len(serverIPs) == 0 {
		// If local server IP discovery is unavailable, we at least confirm DNS resolves.
		return true
	}

	set := make(map[string]struct{}, len(serverIPs))
	for _, ip := range serverIPs {
		set[ip] = struct{}{}
	}
	for _, ip := range resolved {
		if _, ok := set[ip]; ok {
			return true
		}
	}
	return false
}

func newDomainID(now time.Time) string {
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	return fmt.Sprintf("dom-%d-%x", now.Unix(), buf)
}
