// Package auth implements administrator authentication, session management,
// password hashing, and role-based authorization for the DevMail API.
package auth

import "strings"

// Role is a named RBAC role with an associated privilege rank.
type Role string

const (
	RoleSuperAdmin Role = "super_admin"
	RoleServerAdmin Role = "server_admin"
	RoleDomainAdmin Role = "domain_admin"
	RoleOperator    Role = "operator"
)

var ranks = map[Role]int{
	RoleSuperAdmin: 4,
	RoleServerAdmin: 3,
	RoleDomainAdmin: 2,
	RoleOperator:    1,
}

// ParseRole converts a database string into a Role.
func ParseRole(s string) (Role, bool) {
	r := Role(strings.ToLower(strings.TrimSpace(s)))
	if _, ok := ranks[r]; ok {
		return r, true
	}
	return "", false
}

// Allowed reports whether the role is at least as privileged as min.
func (r Role) Allowed(min Role) bool {
	return ranks[r] >= ranks[min]
}

// Valid reports whether the role is a known role.
func (r Role) Valid() bool {
	_, ok := ranks[r]
	return ok
}