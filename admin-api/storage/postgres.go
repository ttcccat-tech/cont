package storage

import (
	"context"
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

func (s *Store) Ping() error {
	return s.db.Ping()
}

func (s *Store) PingRedis() error {
	if s.rdb == nil {
		return fmt.Errorf("redis not configured")
	}
	return s.rdb.Ping(context.Background())
}
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

		// Auth Groups
		`CREATE TABLE IF NOT EXISTS auth_groups (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name TEXT UNIQUE NOT NULL,
			label TEXT,
			description TEXT,
			permissions JSONB DEFAULT '[]',
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)`,

		// Resources
		`CREATE TABLE IF NOT EXISTS resources (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			path TEXT NOT NULL,
			type TEXT
		)`,

		// Audit Logs
		`CREATE TABLE IF NOT EXISTS audit_logs (
			id SERIAL PRIMARY KEY,
			audit_type TEXT NOT NULL,
			target_type TEXT NOT NULL,
			target_id TEXT,
			actor_user_id TEXT,
			actor_username TEXT,
			description TEXT,
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_created ON audit_logs(created_at DESC)`,

		// Alert Rules
		`CREATE TABLE IF NOT EXISTS alert_rules (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT,
			metric_type TEXT NOT NULL,
			service_name TEXT,
			threshold_value REAL NOT NULL,
			operator TEXT NOT NULL,
			duration_seconds INTEGER DEFAULT 60,
			enabled BOOLEAN DEFAULT true,
			notification_channels TEXT,
			slack_webhook_url TEXT,
			email_webhook_url TEXT,
			discord_webhook_url TEXT,
			alert_suppress_seconds INTEGER DEFAULT 300,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)`,

		// API Key Requests
		`CREATE TABLE IF NOT EXISTS api_key_requests (
			id SERIAL PRIMARY KEY,
			key_name TEXT NOT NULL,
			consumer_name TEXT,
			description TEXT,
			status TEXT DEFAULT 'pending',
			applicant_user_id TEXT,
			applicant_username TEXT,
			reviewed_by TEXT,
			reviewed_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)`,

		// Config Snapshots
		`CREATE TABLE IF NOT EXISTS config_snapshots (
			id SERIAL PRIMARY KEY,
			version_label TEXT NOT NULL,
			diff_from_prev TEXT,
			actor_user_id TEXT,
			actor_username TEXT,
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`,
	}
	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil {
			return fmt.Errorf("migration failed: %w\nSQL: %s", err, m)
		}
	}
	return nil
}
