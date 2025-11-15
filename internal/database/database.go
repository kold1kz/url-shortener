// database/database.go
package database

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type Pinger interface {
	Ping() error
}

type DB struct {
	db *sql.DB
}

func NewDB(dsn string) (*DB, error) {
	if dsn == "" {
		return &DB{db: nil}, nil
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	if err := AutoMigrate(db); err != nil {
		return nil, fmt.Errorf("failed to apply migrations: %w", err)
	}

	log.Printf("Database connected and migrations applied")
	return &DB{db: db}, nil
}

func (d *DB) Ping() error {
	if d == nil || d.db == nil {
		return fmt.Errorf("database not configured")
	}
	return d.db.Ping()
}

func (d *DB) Close() error {
	if d == nil || d.db == nil {
		return nil
	}
	return d.db.Close()
}

func (d *DB) GetDB() *sql.DB {
	return d.db
}

func (d *DB) IsConfigured() bool {
	return d.db != nil
}
