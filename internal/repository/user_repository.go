package repository

import (
	"context"
	"database/sql"
	"fmt"
)

type User struct {
	ID       int
	Name     string
	Password string
	Role     string
}

type UserRepository interface {
	FindByUsername(ctx context.Context, username string) (*User, error)
}

type PostgresUserRepository struct {
	db *sql.DB
}

func NewPostgresUserRepository(db *sql.DB) *PostgresUserRepository {
	return &PostgresUserRepository{db: db}
}

func (r *PostgresUserRepository) FindByUsername(ctx context.Context, username string) (*User, error) {
	const query = `
		SELECT user_id, user_name, user_password, user_role
		FROM users
		WHERE user_name = $1
	`

	var user User
	err := r.db.QueryRowContext(ctx, query, username).Scan(
		&user.ID,
		&user.Name,
		&user.Password,
		&user.Role,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("find user by username: %w", err)
	}

	return &user, nil
}
