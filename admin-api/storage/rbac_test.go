package storage

import "testing"

func TestCanRead(t *testing.T) {
	tests := []struct {
		role   string
		entity string
		want   bool
	}{
		// Admin — full read access
		{"admin", "services", true},
		{"admin", "routes", true},
		{"admin", "plugins", true},
		{"admin", "consumers", true},
		{"admin", "upstreams", true},
		{"admin", "targets", true},
		{"admin", "workspaces", true},
		{"admin", "users", true},
		// Editor — read all entities (read-only upstreams/plugins)
		{"editor", "services", true},
		{"editor", "routes", true},
		{"editor", "plugins", true},
		{"editor", "consumers", true},
		{"editor", "upstreams", true},
		{"editor", "targets", true},
		{"editor", "workspaces", true},
		{"editor", "users", true},
		// Viewer — read all entities
		{"viewer", "services", true},
		{"viewer", "routes", true},
		{"viewer", "plugins", true},
		{"viewer", "consumers", true},
		{"viewer", "upstreams", true},
		{"viewer", "targets", true},
		{"viewer", "workspaces", true},
		{"viewer", "users", true},
		// Unknown role defaults to viewer — can read (viewer has read all)
		{"unknown_role", "services", true},
		{"unknown_role", "routes", true},
	}

	for _, tt := range tests {
		t.Run(tt.role+"_"+tt.entity, func(t *testing.T) {
			got := CanRead(tt.role, tt.entity)
			if got != tt.want {
				t.Errorf("CanRead(%q, %q) = %v, want %v", tt.role, tt.entity, got, tt.want)
			}
		})
	}
}

func TestCanWrite(t *testing.T) {
	tests := []struct {
		role   string
		entity string
		want   bool
	}{
		// Admin — full write access
		{"admin", "services", true},
		{"admin", "routes", true},
		{"admin", "plugins", true},
		{"admin", "consumers", true},
		{"admin", "upstreams", true},
		{"admin", "targets", true},
		{"admin", "workspaces", true},
		{"admin", "users", true},
		// Editor — write services/routes/consumers/targets only
		{"editor", "services", true},
		{"editor", "routes", true},
		{"editor", "plugins", false},
		{"editor", "consumers", true},
		{"editor", "upstreams", false},
		{"editor", "targets", true},
		{"editor", "workspaces", false},
		{"editor", "users", false},
		// Viewer — no write
		{"viewer", "services", false},
		{"viewer", "routes", false},
		{"viewer", "plugins", false},
		{"viewer", "consumers", false},
		{"viewer", "upstreams", false},
		{"viewer", "targets", false},
		{"viewer", "workspaces", false},
		{"viewer", "users", false},
		// Unknown role defaults to viewer — cannot write
		{"unknown_role", "services", false},
	}

	for _, tt := range tests {
		t.Run(tt.role+"_"+tt.entity, func(t *testing.T) {
			got := CanWrite(tt.role, tt.entity)
			if got != tt.want {
				t.Errorf("CanWrite(%q, %q) = %v, want %v", tt.role, tt.entity, got, tt.want)
			}
		})
	}
}

func TestCanDelete(t *testing.T) {
	tests := []struct {
		role   string
		entity string
		want   bool
	}{
		// Admin — full delete access
		{"admin", "services", true},
		{"admin", "routes", true},
		{"admin", "plugins", true},
		{"admin", "consumers", true},
		{"admin", "upstreams", true},
		{"admin", "targets", true},
		{"admin", "workspaces", true},
		{"admin", "users", true},
		// Editor — delete services/routes/consumers/targets only
		{"editor", "services", true},
		{"editor", "routes", true},
		{"editor", "plugins", false},
		{"editor", "consumers", true},
		{"editor", "upstreams", false},
		{"editor", "targets", true},
		{"editor", "workspaces", false},
		{"editor", "users", false},
		// Viewer — no delete
		{"viewer", "services", false},
		{"viewer", "routes", false},
		{"viewer", "plugins", false},
		{"viewer", "consumers", false},
		{"viewer", "upstreams", false},
		{"viewer", "targets", false},
		{"viewer", "workspaces", false},
		{"viewer", "users", false},
		// Unknown role defaults to viewer — cannot delete
		{"unknown_role", "services", false},
	}

	for _, tt := range tests {
		t.Run(tt.role+"_"+tt.entity, func(t *testing.T) {
			got := CanDelete(tt.role, tt.entity)
			if got != tt.want {
				t.Errorf("CanDelete(%q, %q) = %v, want %v", tt.role, tt.entity, got, tt.want)
			}
		})
	}
}

func TestGetPermissions(t *testing.T) {
	tests := []struct {
		role string
		want RolePermissions
	}{
		{"admin", PermissionMatrix["admin"]},
		{"editor", PermissionMatrix["editor"]},
		{"viewer", PermissionMatrix["viewer"]},
	}

	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			got := GetPermissions(tt.role)
			if got == nil {
				t.Errorf("GetPermissions(%q) returned nil", tt.role)
			}
			// Compare with expected
			for entity, wantPerm := range tt.want {
				gotPerm, ok := got[entity]
				if !ok {
					t.Errorf("GetPermissions(%q) missing entity %q", tt.role, entity)
					continue
				}
				if gotPerm.Read != wantPerm.Read || gotPerm.Write != wantPerm.Write || gotPerm.Delete != wantPerm.Delete {
					t.Errorf("GetPermissions(%q)[%q] = %+v, want %+v", tt.role, entity, gotPerm, wantPerm)
				}
			}
		})
	}
}

func TestGetPermissions_UnknownRole(t *testing.T) {
	got := GetPermissions("totally_unknown_role")
	// Unknown role should default to viewer permissions
	want := PermissionMatrix["viewer"]
	for entity, wantPerm := range want {
		gotPerm, ok := got[entity]
		if !ok {
			t.Errorf("GetPermissions unknown role should include entity %q", entity)
			continue
		}
		if gotPerm.Read != wantPerm.Read || gotPerm.Write != wantPerm.Write || gotPerm.Delete != wantPerm.Delete {
			t.Errorf("GetPermissions unknown role[%q] = %+v, want %+v", entity, gotPerm, wantPerm)
		}
	}
}