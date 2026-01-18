package worker

// RolePermissions defines what actions a role can perform.
type RolePermissions struct {
	CanDelegate   bool   // Can assign jobs to other workers
	CanReview     bool   // Can review code
	CanAssignJobs bool   // Can create and assign jobs
	CanSupervise  []Role // List of roles this role can supervise
}

// rolePermissionsMap defines permissions for each role.
var rolePermissionsMap = map[Role]RolePermissions{
	RoleDon: {
		CanDelegate:   true,
		CanReview:     true,
		CanAssignJobs: true,
		CanSupervise:  []Role{RoleUnderboss, RoleConsigliere, RoleCapo, RoleSoldato, RoleAssociate, RoleLookout, RoleCleaner},
	},
	RoleUnderboss: {
		CanDelegate:   true,
		CanReview:     false,
		CanAssignJobs: true,
		CanSupervise:  []Role{RoleCapo, RoleSoldato, RoleAssociate, RoleLookout, RoleCleaner},
	},
	RoleConsigliere: {
		CanDelegate:   false,
		CanReview:     true,
		CanAssignJobs: false,
		CanSupervise:  []Role{},
	},
	RoleCapo: {
		CanDelegate:   true,
		CanReview:     false,
		CanAssignJobs: true,
		CanSupervise:  []Role{RoleSoldato, RoleAssociate},
	},
	RoleSoldato: {
		CanDelegate:   false,
		CanReview:     false,
		CanAssignJobs: false,
		CanSupervise:  []Role{},
	},
	RoleAssociate: {
		CanDelegate:   false,
		CanReview:     false,
		CanAssignJobs: false,
		CanSupervise:  []Role{},
	},
	RoleLookout: {
		CanDelegate:   false,
		CanReview:     false,
		CanAssignJobs: false,
		CanSupervise:  []Role{},
	},
	RoleCleaner: {
		CanDelegate:   false,
		CanReview:     false,
		CanAssignJobs: false,
		CanSupervise:  []Role{},
	},
}

// GetPermissions returns the permissions for a role.
func GetPermissions(role Role) RolePermissions {
	if perms, ok := rolePermissionsMap[role]; ok {
		return perms
	}
	// Default to no permissions for unknown roles
	return RolePermissions{}
}

// CanDelegate returns whether the role can delegate work.
func CanDelegate(role Role) bool {
	return GetPermissions(role).CanDelegate
}

// CanReview returns whether the role can review code.
func CanReview(role Role) bool {
	return GetPermissions(role).CanReview
}

// CanAssignJobs returns whether the role can assign jobs.
func CanAssignJobs(role Role) bool {
	return GetPermissions(role).CanAssignJobs
}

// CanSupervise returns whether the supervisor role can supervise the target role.
func CanSupervise(supervisor, target Role) bool {
	perms := GetPermissions(supervisor)
	for _, r := range perms.CanSupervise {
		if r == target {
			return true
		}
	}
	return false
}

// ValidRoles returns all valid worker roles.
func ValidRoles() []Role {
	return []Role{
		RoleDon,
		RoleUnderboss,
		RoleConsigliere,
		RoleCapo,
		RoleSoldato,
		RoleAssociate,
		RoleLookout,
		RoleCleaner,
	}
}

// IsValidRole returns whether the role is valid.
func IsValidRole(role Role) bool {
	for _, r := range ValidRoles() {
		if r == role {
			return true
		}
	}
	return false
}
