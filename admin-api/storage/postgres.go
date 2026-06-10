package storage

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

type Store struct {
	db  *sql.DB
	rdb *Redis
}

func NewPostgres(url string) (*sql.DB, error) {
	if url == "" {
		url = "postgres://kong:kongpass@localhost:5432/cont?sslmode=disable"
	}
	db, err := sql.Open("postgres", url)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("postgres ping failed: %w", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	return db, nil
}

// RoleColumnMigration adds role column if it doesn't exist (for existing dbs)
const RoleColumnMigration = `
ALTER TABLE users ADD COLUMN IF NOT EXISTS role TEXT DEFAULT 'viewer'
`

func RunMigrations(db *sql.DB) error {
	// Add role column if it doesn't exist
	if _, err := db.Exec(RoleColumnMigration); err != nil {
		return fmt.Errorf("role column migration failed: %w", err)
	}

	migrations := []string{
		`CREATE TABLE IF NOT EXISTS services (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name TEXT UNIQUE,
			protocol TEXT DEFAULT 'http',
			host TEXT NOT NULL,
			port INTEGER DEFAULT 80,
			path TEXT DEFAULT '/',
			url TEXT,
			retries INTEGER DEFAULT 5,
			connect_timeout INTEGER DEFAULT 60000,
			read_timeout INTEGER DEFAULT 60000,
			write_timeout INTEGER DEFAULT 60000,
			enabled BOOLEAN DEFAULT true,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS routes (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name TEXT UNIQUE,
			service_id UUID REFERENCES services(id) ON DELETE CASCADE,
			protocols TEXT[] DEFAULT '{http,https}',
			hosts TEXT[],
			paths TEXT[],
			methods TEXT[],
			strip_path BOOLEAN DEFAULT true,
			preserve_host BOOLEAN DEFAULT false,
			regex_priority INTEGER DEFAULT 0,
			https_redirect_status_code INTEGER DEFAULT 426,
			connection_timeout INTEGER DEFAULT 60000,
			enabled BOOLEAN DEFAULT true,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS upstreams (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name TEXT UNIQUE NOT NULL,
			algorithm TEXT DEFAULT 'roundrobin',
			slots INTEGER DEFAULT 10000,
			healthchecks TEXT, -- JSON
			enabled BOOLEAN DEFAULT true,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS targets (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			upstream_id UUID REFERENCES upstreams(id) ON DELETE CASCADE,
			target TEXT NOT NULL,
			weight INTEGER DEFAULT 100,
			enabled BOOLEAN DEFAULT true,
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS consumers (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			username TEXT UNIQUE,
			custom_id TEXT,
			enabled BOOLEAN DEFAULT true,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS plugins (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name TEXT NOT NULL,
			route_id UUID REFERENCES routes(id) ON DELETE CASCADE,
			service_id UUID REFERENCES services(id) ON DELETE CASCADE,
			consumer_id UUID REFERENCES consumers(id) ON DELETE CASCADE,
			config JSONB DEFAULT '{}',
			enabled BOOLEAN DEFAULT true,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW(),
			UNIQUE(route_id, service_id, consumer_id, name)
		)`,
		`CREATE TABLE IF NOT EXISTS workspaces (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name TEXT UNIQUE NOT NULL,
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS users (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			username TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			display_name TEXT,
			email TEXT UNIQUE,
			role TEXT DEFAULT 'user',
			enabled BOOLEAN DEFAULT true,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_users_username ON users(username)`,
		`CREATE INDEX IF NOT EXISTS idx_users_email ON users(email)`,
		// Indexes for route matching performance
		`CREATE INDEX IF NOT EXISTS idx_routes_service ON routes(service_id)`,
		`CREATE INDEX IF NOT EXISTS idx_plugins_route ON plugins(route_id)`,
		`CREATE INDEX IF NOT EXISTS idx_plugins_service ON plugins(service_id)`,
		`CREATE INDEX IF NOT EXISTS idx_plugins_consumer ON plugins(consumer_id)`,
		`CREATE INDEX IF NOT EXISTS idx_targets_upstream ON targets(upstream_id)`,
	}
	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil {
			return fmt.Errorf("migration failed: %w\nSQL: %s", err, m)
		}
	}
	return nil
}
