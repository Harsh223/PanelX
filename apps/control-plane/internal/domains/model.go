package domains

import "time"

// Status represents high-level lifecycle state of a managed domain.
type Status string

const (
	StatusPending  Status = "pending"
	StatusActive   Status = "active"
	StatusDegraded Status = "degraded"
	StatusDisabled Status = "disabled"
	StatusError    Status = "error"
)

// SSLProvider identifies how certificates are managed.
type SSLProvider string

const (
	SSLProviderLetsEncrypt SSLProvider = "letsencrypt"
	SSLProviderManual      SSLProvider = "manual"
)

// SSLStatus indicates certificate health for a domain.
type SSLStatus string

const (
	SSLStatusNone     SSLStatus = "none"
	SSLStatusPending  SSLStatus = "pending"
	SSLStatusIssued   SSLStatus = "issued"
	SSLStatusExpiring SSLStatus = "expiring"
	SSLStatusExpired  SSLStatus = "expired"
	SSLStatusFailed   SSLStatus = "failed"
	SSLStatusRevoked  SSLStatus = "revoked"
	SSLStatusUnknown  SSLStatus = "unknown"
)

// CanonicalHost controls host normalization behavior.
type CanonicalHost string

const (
	CanonicalNone   CanonicalHost = "none"
	CanonicalRoot   CanonicalHost = "root" // example.com
	CanonicalWWW    CanonicalHost = "www"  // www.example.com
	CanonicalCustom CanonicalHost = "custom"
)

// RedirectMode controls HTTP redirect behavior.
type RedirectMode string

const (
	RedirectModeOff       RedirectMode = "off"
	RedirectModeHTTPToTLS RedirectMode = "http_to_https"
	RedirectModeCustom    RedirectMode = "custom"
)

// TLSProfile stores SSL/TLS state and metadata for a domain.
type TLSProfile struct {
	Enabled         bool        `json:"enabled"`
	Provider        SSLProvider `json:"provider"`
	Status          SSLStatus   `json:"status"`
	AutoRenew       bool        `json:"autoRenew"`
	CertificateCN   string      `json:"certificateCn,omitempty"`
	IssuedAt        *time.Time  `json:"issuedAt,omitempty"`
	ExpiresAt       *time.Time  `json:"expiresAt,omitempty"`
	LastAttemptAt   *time.Time  `json:"lastAttemptAt,omitempty"`
	LastError       string      `json:"lastError,omitempty"`
	LastErrorCode   string      `json:"lastErrorCode,omitempty"`
	RenewalWindowIn int         `json:"renewalWindowInDays,omitempty"`
}

// RedirectPolicy stores canonical and redirect configuration.
type RedirectPolicy struct {
	Mode            RedirectMode  `json:"mode"`
	CanonicalHost   CanonicalHost `json:"canonicalHost"`
	CanonicalTarget string        `json:"canonicalTarget,omitempty"`
	CustomTargetURL string        `json:"customTargetUrl,omitempty"`
	PreservePath    bool          `json:"preservePath"`
	PreserveQuery   bool          `json:"preserveQuery"`
	Temporary       bool          `json:"temporary"` // true => 302/307, false => 301/308
}

// HealthSummary captures runtime validation for domain routing.
type HealthSummary struct {
	ResolvesToServerIP bool      `json:"resolvesToServerIp"`
	ResolvedIPs        []string  `json:"resolvedIps,omitempty"`
	NginxConfigPresent bool      `json:"nginxConfigPresent"`
	NginxEnabled       bool      `json:"nginxEnabled"`
	LastCheckedAt      time.Time `json:"lastCheckedAt"`
	Warnings           []string  `json:"warnings,omitempty"`
}

// Domain is the primary persisted entity for managed hostnames.
type Domain struct {
	ID           string         `json:"id"`
	Hostname     string         `json:"hostname"`
	RootDomain   string         `json:"rootDomain,omitempty"` // set for subdomains
	IsSubdomain  bool           `json:"isSubdomain"`
	DocumentRoot string         `json:"documentRoot"`
	SitePath     string         `json:"sitePath,omitempty"`   // panel-owned site path when managed by provisioning
	OwnerLabel   string         `json:"ownerLabel,omitempty"` // optional operator label
	Notes        string         `json:"notes,omitempty"`
	Status       Status         `json:"status"`
	TLS          TLSProfile     `json:"tls"`
	Redirects    RedirectPolicy `json:"redirects"`
	Health       HealthSummary  `json:"health"`
	CreatedAt    time.Time      `json:"createdAt"`
	UpdatedAt    time.Time      `json:"updatedAt"`
	LastSyncedAt *time.Time     `json:"lastSyncedAt,omitempty"`
}

// DomainList is a transport-friendly list payload shape.
type DomainList struct {
	Items []Domain `json:"items"`
}
