package auth

import "fmt"

// Service provides authorization decisions for API handlers and workflows.
type Service struct {
	rolePermissions map[Role][]Permission
}

// NewService initializes built-in RBAC role mappings.
func NewService() *Service {
	return &Service{
		rolePermissions: map[Role][]Permission{
			RoleProviderAdmin: {
				PermTenantRead,
				PermTenantWrite,
				PermSiteProvision,
				PermWorkflowManage,
				PermSecurityView,
				PermSecurityManage,
				PermMigrationManage,
			},
			RoleResellerAdmin: {
				PermTenantRead,
				PermTenantWrite,
				PermSiteProvision,
				PermWorkflowManage,
			},
			RoleTenantAdmin: {
				PermTenantRead,
				PermTenantWrite,
				PermSiteProvision,
			},
			RoleTenantViewer: {
				PermTenantRead,
				PermSecurityView,
			},
		},
	}
}

// BuildPrincipal resolves effective permissions from declared roles.
func (s *Service) BuildPrincipal(subjectID, tenantID string, roles []Role) (Principal, error) {
	if subjectID == "" {
		return Principal{}, fmt.Errorf("subject id is required")
	}

	perms := make(map[Permission]struct{})
	for _, role := range roles {
		mapped, ok := s.rolePermissions[role]
		if !ok {
			continue
		}
		for _, perm := range mapped {
			perms[perm] = struct{}{}
		}
	}

	return Principal{
		SubjectID:   subjectID,
		TenantID:    tenantID,
		Roles:       roles,
		Permissions: perms,
	}, nil
}

// Authorize checks whether a principal can perform a permissioned action.
func (s *Service) Authorize(p Principal, required Permission) error {
	if p.HasPermission(required) {
		return nil
	}
	return fmt.Errorf("permission denied: missing %s", required)
}
