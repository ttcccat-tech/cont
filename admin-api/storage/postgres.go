package storage

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
	"github.com/ttcccat-tech/cont/admin-api/migrator"
)

type Store struct {
	db  *sql.DB
	rdb *Redis
}

// DB returns the underlying sql.DB for use by routes packages
func (s *Store) DB() *sql.DB {
	return s.db
}

// Redis returns the underlying Redis client
func (s *Store) Redis() *Redis {
	return s.rdb
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

// DBPoolStats returns database connection pool statistics
func (s *Store) DBPoolStats() struct {
	MaxOpen int
	Open    int
	Idle    int
} {
	if s.db == nil {
		return struct {
			MaxOpen int
			Open    int
			Idle    int
		}{}
	}
	stats := s.db.Stats()
	return struct {
		MaxOpen int
		Open    int
		Idle    int
	}{stats.MaxOpenConnections, stats.OpenConnections, stats.Idle}
}

// RedisPoolStats returns Redis connection pool statistics
func (s *Store) RedisPoolStats() struct {
	TotalConns int
	IdleConns  int
} {
	if s.rdb == nil || s.rdb.client == nil {
		return struct {
			TotalConns int
			IdleConns  int
		}{}
	}
	stats := s.rdb.client.PoolStats()
	return struct {
		TotalConns int
		IdleConns  int
	}{int(stats.TotalConns), int(stats.IdleConns)}
}
const RoleColumnMigration = `` // DEPRECATED: role column now part of users table creation

func RunMigrations(db *sql.DB) error {
	return migrator.Migrate(db)
}
