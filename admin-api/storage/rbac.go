package storage

// RBAC Permission Matrix
// Defines what each role can do across entities

type Permission struct {
	Read  bool `json:"read"`
	Write bool `json:"write"`
	Delete bool `json:"delete"`
}

type RolePermissions map[string]Permission

// PermissionMatrix defines role-based access control
// admin: full CRUD all entities
// editor: CRUD services/routes/consumers, read plugins/upstreams
// viewer: read-only all
var PermissionMatrix = map[string]RolePermissions{
	"admin": {
		"services":          Permission{Read: true, Write: true, Delete: true},
		"routes":            Permission{Read: true, Write: true, Delete: true},
		"plugins":           Permission{Read: true, Write: true, Delete: true},
		"consumers":         Permission{Read: true, Write: true, Delete: true},
		"upstreams":         Permission{Read: true, Write: true, Delete: true},
		"targets":           Permission{Read: true, Write: true, Delete: true},
		"workspaces":        Permission{Read: true, Write: true, Delete: true},
		"users":             Permission{Read: true, Write: true, Delete: true},
		"groups":            Permission{Read: true, Write: true, Delete: true},
		"api_keys":          Permission{Read: true, Write: true, Delete: true},
		"config_snapshots":  Permission{Read: true, Write: true, Delete: true},
		"audit_logs":        Permission{Read: true, Write: true, Delete: true},
	},
	"editor": {
		"services":          Permission{Read: true, Write: true, Delete: true},
		"routes":            Permission{Read: true, Write: true, Delete: true},
		"plugins":           Permission{Read: true, Write: false, Delete: false},
		"consumers":         Permission{Read: true, Write: true, Delete: true},
		"upstreams":         Permission{Read: true, Write: false, Delete: false},
		"targets":           Permission{Read: true, Write: true, Delete: true},
		"workspaces":        Permission{Read: true, Write: false, Delete: false},
		"users":             Permission{Read: true, Write: false, Delete: false},
		"groups":            Permission{Read: true, Write: false, Delete: false},
		"api_keys":          Permission{Read: true, Write: false, Delete: false},
		"config_snapshots":  Permission{Read: true, Write: false, Delete: false},
		"audit_logs":        Permission{Read: true, Write: false, Delete: false},
	},
	"viewer": {
		"services":          Permission{Read: true, Write: false, Delete: false},
		"routes":            Permission{Read: true, Write: false, Delete: false},
		"plugins":           Permission{Read: true, Write: false, Delete: false},
		"consumers":         Permission{Read: true, Write: false, Delete: false},
		"upstreams":         Permission{Read: true, Write: false, Delete: false},
		"targets":           Permission{Read: true, Write: false, Delete: false},
		"workspaces":        Permission{Read: true, Write: false, Delete: false},
		"users":             Permission{Read: true, Write: false, Delete: false},
		"groups":            Permission{Read: true, Write: false, Delete: false},
		"api_keys":          Permission{Read: true, Write: false, Delete: false},
		"config_snapshots":  Permission{Read: true, Write: false, Delete: false},
		"audit_logs":        Permission{Read: true, Write: false, Delete: false},
	},
}

// Roles is the list of available roles
var Roles = []string{"admin", "editor", "viewer"}

// GetPermissions returns the permission map for a given role
func GetPermissions(role string) RolePermissions {
	if perms, ok := PermissionMatrix[role]; ok {
		return perms
	}
	// Default to viewer permissions for unknown roles
	return PermissionMatrix["viewer"]
}

// CanRead checks if a role has read permission for an entity
func CanRead(role, entity string) bool {
	perms := GetPermissions(role)
	if p, ok := perms[entity]; ok {
		return p.Read
	}
	return false
}

// CanWrite checks if a role has write permission for an entity
func CanWrite(role, entity string) bool {
	perms := GetPermissions(role)
	if p, ok := perms[entity]; ok {
		return p.Write
	}
	return false
}

// CanDelete checks if a role has delete permission for an entity
func CanDelete(role, entity string) bool {
	perms := GetPermissions(role)
	if p, ok := perms[entity]; ok {
		return p.Delete
	}
	return false
}
