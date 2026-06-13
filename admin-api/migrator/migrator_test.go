package migrator

import (
	"database/sql"
	"testing"

	_ "github.com/lib/pq"
)

func TestMigrationManager(t *testing.T) {
	// Use a test DB connection
	db, err := sql.Open("postgres", "postgres://postgres:postgres@localhost:5432/cont_test?sslmode=disable")
	if err != nil {
		t.Skip("Skipping migrator test: no test DB available")
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Skip("Skipping migrator test: cannot ping test DB")
	}

	// Clean up before test
	db.Exec("DROP TABLE IF EXISTS schema_migrations")
	db.Exec("DROP TABLE IF EXISTS users CASCADE")

	m := NewManager(db)
	m.Register(Migration{
		Version:     1,
		Description: "create users table",
		Up:          `CREATE TABLE IF NOT EXISTS users (id SERIAL PRIMARY KEY, username TEXT NOT NULL);`,
		Down:        `DROP TABLE IF EXISTS users;`,
	})
	m.Register(Migration{
		Version:     2,
		Description: "add email column",
		Up:          `ALTER TABLE users ADD COLUMN IF NOT EXISTS email TEXT;`,
		Down:        `ALTER TABLE users DROP COLUMN IF EXISTS email;`,
	})

	// Run migrations
	if err := m.Run(); err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Verify both applied
	applied, err := m.Status()
	if err != nil {
		t.Fatalf("Status() failed: %v", err)
	}
	if len(applied) != 2 {
		t.Errorf("Expected 2 applied migrations, got %d: %v", len(applied), applied)
	}

	// Run again — should skip
	if err := m.Run(); err != nil {
		t.Fatalf("Run() second time failed: %v", err)
	}
	applied2, _ := m.Status()
	if len(applied2) != 2 {
		t.Errorf("Expected 2 after second run, got %d", len(applied2))
	}

	// Rollback v2
	if err := m.Rollback(2); err != nil {
		t.Fatalf("Rollback(2) failed: %v", err)
	}
	applied, _ = m.Status()
	if len(applied) != 1 {
		t.Errorf("Expected 1 after rollback, got %d", len(applied))
	}

	// Verify email column gone
	var colExists int
	db.QueryRow("SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='email'").Scan(&colExists)
	if colExists != 1 {
		t.Error("email column should not exist after rollback")
	}

	t.Log("MigrationManager tests passed")
}
