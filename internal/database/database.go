package database

import (
	"database/sql"
	"fmt"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type Pinger interface {
	Ping() error
}

type DB struct {
	db *sql.DB
}

func NewDB(dsn string) (*DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{db: db}, nil
}

func (d *DB) Ping() error {
	return d.db.Ping()
}

func (d *DB) Close() error {
	return d.db.Close()
}

func (d *DB) GetDB() *sql.DB {
	return d.db
}
