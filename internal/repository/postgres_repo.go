package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"url-shortener/internal/model"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/lib/pq"
)

// PostgresURLRepository — реализация URLRepository поверх PostgreSQL.
type PostgresURLRepository struct {
	db *sql.DB
}

// URLConflictError возвращается при попытке создать URL с original_url,
// который уже существует в хранилище (PostgreSQL unique violation).
//
// ExistingURL содержит найденную запись, которую можно вернуть клиенту.
type URLConflictError struct {
	ExistingURL *model.URL
}

// NewPostgresURLRepository создаёт репозиторий PostgreSQL и проверяет,
// что таблица urls существует (миграции применены).
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
		return fmt.Errorf("check urls table exists: %w", err)
	}

	if !exists {
		return fmt.Errorf("urls table does not exist, run migrations first")
	}

	return nil
}

func (r *PostgresURLRepository) Create(url *model.URL) error {
	query := `INSERT INTO urls (id, original_url, short_url, user_id) VALUES ($1, $2, $3, $4)`
	_, err := r.db.Exec(query, url.ID, url.Original, url.Short, url.UserID)
	if err != nil {
		// Проверяем, является ли ошибка нарушением уникальности
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			// URL уже существует, находим существующую запись
			var existing *model.URL
			existing, err = r.FindByOriginalURL(url.Original)
			if err != nil {
				return fmt.Errorf("failed to find existing URL: %w", err)
			}
			if existing != nil {
				return &URLConflictError{ExistingURL: existing}
			}
		}
		return fmt.Errorf("failed to insert URL: %w", err)
	}

	return nil
}

// Error реализует интерфейс error.
func (e *URLConflictError) Error() string {
	return "URL already exists"
}

// IsURLConflictError проверяет, что ошибка имеет тип URLConflictError.
func IsURLConflictError(err error) bool {
	_, ok := err.(*URLConflictError)
	return ok
}

func (r *PostgresURLRepository) FindByID(id string) (*model.URL, error) {
	query := `SELECT id, original_url, short_url, user_id, is_deleted FROM urls WHERE id = $1`
	row := r.db.QueryRow(query, id)

	var url model.URL
	err := row.Scan(&url.ID, &url.Original, &url.Short, &url.UserID, &url.IsDeleted)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &url, nil
}

func (r *PostgresURLRepository) FindByOriginalURL(originalURL string) (*model.URL, error) {
	query := `SELECT id, original_url, short_url, user_id, is_deleted FROM urls WHERE original_url = $1`
	row := r.db.QueryRow(query, originalURL)

	var url model.URL
	err := row.Scan(&url.ID, &url.Original, &url.Short, &url.UserID, &url.IsDeleted)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &url, nil
}

func (r *PostgresURLRepository) FindByUserID(userID string) ([]*model.URL, error) {
	query := `SELECT id, original_url, short_url, user_id, is_deleted FROM urls WHERE user_id = $1`

	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []*model.URL
	for rows.Next() {
		var url model.URL
		if err := rows.Scan(&url.ID, &url.Original, &url.Short, &url.UserID, &url.IsDeleted); err != nil {
			return nil, err
		}
		res = append(res, &url)
	}
	return res, rows.Err()
}

// Close закрывает соединение с БД.
func (r *PostgresURLRepository) Close() error {
	return r.db.Close()
}

// Ping проверяет доступность БД.
func (r *PostgresURLRepository) Ping() error {
	return r.db.Ping()
}

func (r *PostgresURLRepository) CreateBatch(urls []*model.URL) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	insertStmt, err := tx.Prepare("INSERT INTO urls (id, original_url, short_url) VALUES ($1, $2, $3)")
	if err != nil {
		return fmt.Errorf("failed to prepare insert statement: %w", err)
	}
	defer insertStmt.Close()

	checkStmt, err := tx.Prepare("SELECT id, short_url FROM urls WHERE original_url = $1")
	if err != nil {
		return fmt.Errorf("failed to prepare check statement: %w", err)
	}
	defer checkStmt.Close()

	for _, url := range urls {
		var existingID, existingShort string
		err := checkStmt.QueryRow(url.Original).Scan(&existingID, &existingShort)

		if err == nil {
			url.ID = existingID
			url.Short = existingShort
			continue
		} else if err != sql.ErrNoRows {
			return fmt.Errorf("failed to check existing URL: %w", err)
		}

		if _, err := insertStmt.Exec(url.ID, url.Original, url.Short); err != nil {
			return fmt.Errorf("failed to insert URL: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (r *PostgresURLRepository) MarkAsDeleted(ctx context.Context, userID string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	query := "UPDATE urls SET is_deleted = TRUE WHERE user_id = $1 AND id = ANY($2)"
	if _, err := r.db.ExecContext(ctx, query, userID, pq.Array(ids)); err != nil {
		return fmt.Errorf("mark urls as deleted: %w", err)
	}

	return nil
}

func (r *PostgresURLRepository) GetStats(ctx context.Context) (int, int, error) {
	var urls int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM urls`).Scan(&urls); err != nil {
		return 0, 0, fmt.Errorf("count urls: %w", err)
	}

	var users int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT user_id) FROM urls`).Scan(&users); err != nil {
		return 0, 0, fmt.Errorf("count users: %w", err)
	}

	return urls, users, nil
}
