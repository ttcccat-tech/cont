// CLI tool for database migration management.
//
// Usage:
//   go run ./migrator/cmd/main.go status <POSTGRES_URL>
//   go run ./migrator/cmd/main.go migrate <POSTGRES_URL>
//   go run ./migrator/cmd/main.go rollback <POSTGRES_URL> [version]
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/lib/pq"
	"github.com/ttcccat-tech/cont/admin-api/migrator"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: migrate <status|migrate|rollback> <POSTGRES_URL> [version]")
		os.Exit(1)
	}

	cmd := os.Args[1]
	dbURL := os.Args[2]

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping DB: %v", err)
	}

	m := migrator.NewManager(db)
	migrator.RegisterAllMigrations(m)

	switch cmd {
	case "status":
		applied, err := m.Status()
		if err != nil {
			log.Fatalf("Status failed: %v", err)
		}
		fmt.Printf("Applied migrations (%d): %v\n", len(applied), applied)

	case "migrate":
		fmt.Println("Running migrations...")
		if err := m.Run(); err != nil {
			log.Fatalf("Migration failed: %v", err)
		}
		fmt.Println("Migrations complete.")

	case "rollback":
		var version int
		if len(os.Args) >= 4 {
			version, err = strconv.Atoi(os.Args[3])
			if err != nil {
				log.Fatalf("Invalid version: %s", os.Args[3])
			}
			fmt.Printf("Rolling back migration v%d...\n", version)
			if err := m.Rollback(version); err != nil {
				log.Fatalf("Rollback v%d failed: %v", version, err)
			}
		} else {
			applied, err := m.Status()
			if err != nil {
				log.Fatalf("Status failed: %v", err)
			}
			if len(applied) == 0 {
				fmt.Println("No migrations to rollback.")
			} else {
				version = applied[len(applied)-1]
				fmt.Printf("Rolling back last migration v%d...\n", version)
				if err := m.Rollback(version); err != nil {
					log.Fatalf("Rollback v%d failed: %v", version, err)
				}
			}
		}
		fmt.Printf("Rollback of v%d complete.\n", version)

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		os.Exit(1)
	}
}

func init() {
	// Silence pq header warning
	pq.Driver{}
}
