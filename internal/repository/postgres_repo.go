package repository

import (
	"database/sql"
	"fmt"
	"url-shortener/internal/model"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type PostgresURLRepository struct {
	db *sql.DB
}

func NewPostgresURLRepository(db *sql.DB) (*PostgresURLRepository, error) {
	if err := checkTableExists(db); err != nil {
		return nil, fmt.Errorf("table check failed: %w", err)
	}

	return &PostgresURLRepository{db: db}, nil
}

func checkTableExists(db *sql.DB) error {
	var exists bool
	err := db.QueryRow(`
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_name = 'urls'
		)
	`).Scan(&exists)

	if err != nil {
		return err
	}

	if !exists {
		return fmt.Errorf("urls table does not exist, run migrations first")
	}

	return nil
}

func (r *PostgresURLRepository) Create(url *model.URL) error {
	query := `INSERT INTO urls (id, original_url, short_url) VALUES ($1, $2, $3)`
	_, err := r.db.Exec(query, url.ID, url.Original, url.Short)
	return err
}

func (r *PostgresURLRepository) FindByID(id string) (*model.URL, error) {
	query := `SELECT id, original_url, short_url FROM urls WHERE id = $1`
	row := r.db.QueryRow(query, id)

	var url model.URL
	err := row.Scan(&url.ID, &url.Original, &url.Short)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &url, nil
}

func (r *PostgresURLRepository) FindByOriginalURL(originalURL string) (*model.URL, error) {
	query := `SELECT id, original_url, short_url FROM urls WHERE original_url = $1`
	row := r.db.QueryRow(query, originalURL)

	var url model.URL
	err := row.Scan(&url.ID, &url.Original, &url.Short)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &url, nil
}

func (r *PostgresURLRepository) Close() error {
	return r.db.Close()
}

func (r *PostgresURLRepository) Ping() error {
	return r.db.Ping()
}
