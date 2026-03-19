package auth

import "time"

// Role defines built-in role identifiers for provider and tenant scopes.
type Role string

const (
	RoleProviderAdmin Role = "provider_admin"
	RoleResellerAdmin Role = "reseller_admin"
	RoleTenantAdmin   Role = "tenant_admin"
	RoleTenantViewer  Role = "tenant_viewer"
)

// Permission is an action identifier used by policy checks.
type Permission string

const (
	PermTenantRead      Permission = "tenant.read"
	PermTenantWrite     Permission = "tenant.write"
	PermSiteProvision   Permission = "site.provision"
	PermWorkflowManage  Permission = "workflow.manage"
	PermSecurityView    Permission = "security.view"
	PermSecurityManage  Permission = "security.manage"
	PermMigrationManage Permission = "migration.manage"
)

// Principal represents an authenticated identity resolved from token/session.
type Principal struct {
	SubjectID   string
	TenantID    string
	Roles       []Role
	Permissions map[Permission]struct{}
	IssuedAt    time.Time
	ExpiresAt   time.Time
}

// HasPermission checks if principal has a specific permission.
func (p Principal) HasPermission(perm Permission) bool {
	_, ok := p.Permissions[perm]
	return ok
}
