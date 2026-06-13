package migrator

import (
	"database/sql"
	"fmt"
	"log"
	"sort"
)

type Migration struct {
	Version     int
	Description string
	Up          string
	Down        string
}

type Manager struct {
	db      *sql.DB
	migs    []Migration
	applied map[int]bool
}

func NewManager(db *sql.DB) *Manager {
	return &Manager{
		db:      db,
		migs:    []Migration{},
		applied: make(map[int]bool),
	}
}

func (m *Manager) Register(mig Migration) {
	m.migs = append(m.migs, mig)
}

func (m *Manager) Run() error {
	// Create migrations tracking table
	if err := m.createMigrationsTable(); err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	// Load already-applied migrations
	if err := m.loadApplied(); err != nil {
		return fmt.Errorf("load applied: %w", err)
	}

	// Sort by version
	sort.Slice(m.migs, func(i, j int) bool {
		return m.migs[i].Version < m.migs[j].Version
	})

	// Detect existing schema for fresh migrator installs (backward compat)
	if len(m.applied) == 0 {
		if err := m.detectExistingSchema(); err != nil {
			log.Printf("Warning: detectExistingSchema failed: %v (continuing anyway)", err)
		}
	}

	for _, mig := range m.migs {
		if m.applied[mig.Version] {
			log.Printf("Migration v%d already applied, skipping", mig.Version)
			continue
		}
		log.Printf("Applying migration v%d: %s", mig.Version, mig.Description)
		if _, err := m.db.Exec(mig.Up); err != nil {
			return fmt.Errorf("migration v%d up failed: %w", mig.Version, err)
		}
		if err := m.recordApplied(mig.Version); err != nil {
			return fmt.Errorf("record applied v%d: %w", mig.Version, err)
		}
		log.Printf("Migration v%d applied successfully", mig.Version)
	}
	return nil
}

// detectExistingSchema records all currently-installed schema versions as applied.
// This is only called when schema_migrations is empty (fresh migrator install on an
// existing database that was created by the old RunMigrations system).
func (m *Manager) detectExistingSchema() error {
	// Check if any tables exist at all
	var count int
	row := m.db.QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public'")
	if err := row.Scan(&count); err != nil {
		return fmt.Errorf("check existing tables: %w", err)
	}
	if count == 0 {
		log.Println("Fresh database, no existing schema detected")
		return nil
	}

	log.Printf("Detected %d existing tables — recording all migration versions as applied", count)

	// Record every migration version as applied (they're all already in the DB)
	for _, mig := range m.migs {
		if err := m.recordApplied(mig.Version); err != nil {
			return fmt.Errorf("record applied v%d: %w", mig.Version, err)
		}
	}
	log.Println("Existing schema recorded — migrator will only track new migrations going forward")
	return nil
}

func (m *Manager) Rollback(version int) error {
	for _, mig := range m.migs {
		if mig.Version == version && m.applied[version] {
			log.Printf("Rolling back migration v%d: %s", mig.Version, mig.Description)
			if _, err := m.db.Exec(mig.Down); err != nil {
				return fmt.Errorf("rollback v%d failed: %w", mig.Version, err)
			}
			if err := m.removeApplied(version); err != nil {
				return fmt.Errorf("remove applied v%d: %w", mig.Version, err)
			}
			log.Printf("Rollback v%d done", mig.Version)
			return nil
		}
	}
	return fmt.Errorf("migration v%d not found or not applied", version)
}

func (m *Manager) Status() ([]int, error) {
	var versions []int
	rows, err := m.db.Query("SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		return nil, fmt.Errorf("query migrations: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		versions = append(versions, v)
	}
	return versions, nil
}

func (m *Manager) createMigrationsTable() error {
	_, err := m.db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			description TEXT,
			applied_at TIMESTAMPTZ DEFAULT NOW()
		)
	`)
	return err
}

func (m *Manager) loadApplied() error {
	rows, err := m.db.Query("SELECT version FROM schema_migrations")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return err
		}
		m.applied[v] = true
	}
	return nil
}

func (m *Manager) recordApplied(version int) error {
	_, err := m.db.Exec(
		"INSERT INTO schema_migrations (version, description) VALUES ($1, $2)",
		version, m.findDescription(version),
	)
	return err
}

func (m *Manager) removeApplied(version int) error {
	_, err := m.db.Exec("DELETE FROM schema_migrations WHERE version = $1", version)
	return err
}

func (m *Manager) findDescription(version int) string {
	for _, mig := range m.migs {
		if mig.Version == version {
			return mig.Description
		}
	}
	return ""
}
