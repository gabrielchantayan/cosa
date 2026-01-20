package worker

import (
	"testing"
)

func TestGetPermissions(t *testing.T) {
	tests := []struct {
		role         Role
		canDelegate  bool
		canReview    bool
		canAssign    bool
		supervisees  int
	}{
		{RoleDon, true, true, true, 7},
		{RoleUnderboss, true, false, true, 5},
		{RoleConsigliere, false, true, false, 0},
		{RoleCapo, true, false, true, 2},
		{RoleSoldato, false, false, false, 0},
		{RoleAssociate, false, false, false, 0},
		{RoleLookout, false, false, false, 0},
		{RoleCleaner, false, false, false, 0},
	}

	for _, tt := range tests {
		t.Run(string(tt.role), func(t *testing.T) {
			perms := GetPermissions(tt.role)

			if perms.CanDelegate != tt.canDelegate {
				t.Errorf("CanDelegate: expected %v, got %v", tt.canDelegate, perms.CanDelegate)
			}
			if perms.CanReview != tt.canReview {
				t.Errorf("CanReview: expected %v, got %v", tt.canReview, perms.CanReview)
			}
			if perms.CanAssignJobs != tt.canAssign {
				t.Errorf("CanAssignJobs: expected %v, got %v", tt.canAssign, perms.CanAssignJobs)
			}
			if len(perms.CanSupervise) != tt.supervisees {
				t.Errorf("CanSupervise count: expected %d, got %d", tt.supervisees, len(perms.CanSupervise))
			}
		})
	}
}

func TestGetPermissions_UnknownRole(t *testing.T) {
	perms := GetPermissions(Role("unknown"))

	// Unknown roles should have no permissions
	if perms.CanDelegate {
		t.Error("unknown role should not be able to delegate")
	}
	if perms.CanReview {
		t.Error("unknown role should not be able to review")
	}
	if perms.CanAssignJobs {
		t.Error("unknown role should not be able to assign jobs")
	}
	if len(perms.CanSupervise) != 0 {
		t.Error("unknown role should not be able to supervise anyone")
	}
}

func TestCanDelegate(t *testing.T) {
	delegators := []Role{RoleDon, RoleUnderboss, RoleCapo}
	nonDelegators := []Role{RoleConsigliere, RoleSoldato, RoleAssociate, RoleLookout, RoleCleaner}

	for _, role := range delegators {
		if !CanDelegate(role) {
			t.Errorf("%s should be able to delegate", role)
		}
	}

	for _, role := range nonDelegators {
		if CanDelegate(role) {
			t.Errorf("%s should not be able to delegate", role)
		}
	}
}

func TestCanReview(t *testing.T) {
	reviewers := []Role{RoleDon, RoleConsigliere}
	nonReviewers := []Role{RoleUnderboss, RoleCapo, RoleSoldato, RoleAssociate, RoleLookout, RoleCleaner}

	for _, role := range reviewers {
		if !CanReview(role) {
			t.Errorf("%s should be able to review", role)
		}
	}

	for _, role := range nonReviewers {
		if CanReview(role) {
			t.Errorf("%s should not be able to review", role)
		}
	}
}

func TestCanAssignJobs(t *testing.T) {
	assigners := []Role{RoleDon, RoleUnderboss, RoleCapo}
	nonAssigners := []Role{RoleConsigliere, RoleSoldato, RoleAssociate, RoleLookout, RoleCleaner}

	for _, role := range assigners {
		if !CanAssignJobs(role) {
			t.Errorf("%s should be able to assign jobs", role)
		}
	}

	for _, role := range nonAssigners {
		if CanAssignJobs(role) {
			t.Errorf("%s should not be able to assign jobs", role)
		}
	}
}

func TestCanSupervise(t *testing.T) {
	// Don can supervise everyone except other Dons
	donTests := []struct {
		target   Role
		expected bool
	}{
		{RoleUnderboss, true},
		{RoleConsigliere, true},
		{RoleCapo, true},
		{RoleSoldato, true},
		{RoleAssociate, true},
		{RoleLookout, true},
		{RoleCleaner, true},
		{RoleDon, false}, // Can't supervise another Don
	}

	for _, tt := range donTests {
		if CanSupervise(RoleDon, tt.target) != tt.expected {
			t.Errorf("Don supervising %s: expected %v", tt.target, tt.expected)
		}
	}

	// Underboss can supervise lower ranks
	underbossTests := []struct {
		target   Role
		expected bool
	}{
		{RoleCapo, true},
		{RoleSoldato, true},
		{RoleAssociate, true},
		{RoleLookout, true},
		{RoleCleaner, true},
		{RoleDon, false},
		{RoleUnderboss, false},
		{RoleConsigliere, false},
	}

	for _, tt := range underbossTests {
		if CanSupervise(RoleUnderboss, tt.target) != tt.expected {
			t.Errorf("Underboss supervising %s: expected %v", tt.target, tt.expected)
		}
	}

	// Capo can only supervise Soldatos and Associates
	capoTests := []struct {
		target   Role
		expected bool
	}{
		{RoleSoldato, true},
		{RoleAssociate, true},
		{RoleDon, false},
		{RoleUnderboss, false},
		{RoleConsigliere, false},
		{RoleCapo, false},
		{RoleLookout, false},
		{RoleCleaner, false},
	}

	for _, tt := range capoTests {
		if CanSupervise(RoleCapo, tt.target) != tt.expected {
			t.Errorf("Capo supervising %s: expected %v", tt.target, tt.expected)
		}
	}

	// Consigliere can't supervise anyone
	for _, role := range ValidRoles() {
		if CanSupervise(RoleConsigliere, role) {
			t.Errorf("Consigliere should not be able to supervise %s", role)
		}
	}

	// Soldato can't supervise anyone
	for _, role := range ValidRoles() {
		if CanSupervise(RoleSoldato, role) {
			t.Errorf("Soldato should not be able to supervise %s", role)
		}
	}
}

func TestValidRoles(t *testing.T) {
	roles := ValidRoles()

	expectedRoles := []Role{
		RoleDon,
		RoleUnderboss,
		RoleConsigliere,
		RoleCapo,
		RoleSoldato,
		RoleAssociate,
		RoleLookout,
		RoleCleaner,
	}

	if len(roles) != len(expectedRoles) {
		t.Errorf("expected %d roles, got %d", len(expectedRoles), len(roles))
	}

	roleMap := make(map[Role]bool)
	for _, r := range roles {
		roleMap[r] = true
	}

	for _, expected := range expectedRoles {
		if !roleMap[expected] {
			t.Errorf("expected role %s to be in ValidRoles()", expected)
		}
	}
}

func TestIsValidRole(t *testing.T) {
	validRoles := []Role{
		RoleDon,
		RoleUnderboss,
		RoleConsigliere,
		RoleCapo,
		RoleSoldato,
		RoleAssociate,
		RoleLookout,
		RoleCleaner,
	}

	for _, role := range validRoles {
		if !IsValidRole(role) {
			t.Errorf("%s should be a valid role", role)
		}
	}

	invalidRoles := []Role{"invalid", "boss", "worker", ""}
	for _, role := range invalidRoles {
		if IsValidRole(role) {
			t.Errorf("%s should not be a valid role", role)
		}
	}
}

func TestRoleHierarchy(t *testing.T) {
	// Test that the hierarchy is maintained
	// Don > Underboss > Capo > Soldato/Associate

	// Don should be able to supervise all lower ranks
	lowerRanks := []Role{RoleUnderboss, RoleCapo, RoleSoldato, RoleAssociate}
	for _, lower := range lowerRanks {
		if !CanSupervise(RoleDon, lower) {
			t.Errorf("Don should supervise %s", lower)
		}
	}

	// Soldato should not be able to supervise anyone
	for _, role := range ValidRoles() {
		if CanSupervise(RoleSoldato, role) {
			t.Errorf("Soldato should not supervise anyone, but can supervise %s", role)
		}
	}

	// Associate should not be able to supervise anyone
	for _, role := range ValidRoles() {
		if CanSupervise(RoleAssociate, role) {
			t.Errorf("Associate should not supervise anyone, but can supervise %s", role)
		}
	}
}

func TestRoleConstants(t *testing.T) {
	// Verify role constants have expected values
	if RoleDon != "don" {
		t.Errorf("RoleDon should be 'don', got '%s'", RoleDon)
	}
	if RoleUnderboss != "underboss" {
		t.Errorf("RoleUnderboss should be 'underboss', got '%s'", RoleUnderboss)
	}
	if RoleConsigliere != "consigliere" {
		t.Errorf("RoleConsigliere should be 'consigliere', got '%s'", RoleConsigliere)
	}
	if RoleCapo != "capo" {
		t.Errorf("RoleCapo should be 'capo', got '%s'", RoleCapo)
	}
	if RoleSoldato != "soldato" {
		t.Errorf("RoleSoldato should be 'soldato', got '%s'", RoleSoldato)
	}
	if RoleAssociate != "associate" {
		t.Errorf("RoleAssociate should be 'associate', got '%s'", RoleAssociate)
	}
	if RoleLookout != "lookout" {
		t.Errorf("RoleLookout should be 'lookout', got '%s'", RoleLookout)
	}
	if RoleCleaner != "cleaner" {
		t.Errorf("RoleCleaner should be 'cleaner', got '%s'", RoleCleaner)
	}
}

func TestSpecialRoles(t *testing.T) {
	// Consigliere is the only reviewer besides Don
	if !CanReview(RoleConsigliere) {
		t.Error("Consigliere should be able to review")
	}
	if CanDelegate(RoleConsigliere) {
		t.Error("Consigliere should not be able to delegate")
	}
	if CanAssignJobs(RoleConsigliere) {
		t.Error("Consigliere should not be able to assign jobs")
	}

	// Lookout and Cleaner are background service roles
	if CanDelegate(RoleLookout) || CanReview(RoleLookout) || CanAssignJobs(RoleLookout) {
		t.Error("Lookout should have no permissions")
	}
	if CanDelegate(RoleCleaner) || CanReview(RoleCleaner) || CanAssignJobs(RoleCleaner) {
		t.Error("Cleaner should have no permissions")
	}
}
