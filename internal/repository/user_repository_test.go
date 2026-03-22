package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestNewPostgresUserRepository(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	repo := NewPostgresUserRepository(db)
	if repo == nil {
		t.Fatal("expected repo, got nil")
	}
	if repo.db != db {
		t.Fatal("expected db to be assigned")
	}
}

func TestPostgresUserRepository_FindByUsername_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	repo := NewPostgresUserRepository(db)

	const username = "admin"
	rows := sqlmock.NewRows([]string{
		"user_id",
		"user_name",
		"user_password",
		"user_role",
	}).AddRow(1, "admin", "secret", "admin")

	mock.ExpectQuery(`SELECT user_id, user_name, user_password, user_role FROM users WHERE user_name = \$1`).
		WithArgs(username).
		WillReturnRows(rows)

	user, err := repo.FindByUsername(context.Background(), username)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user == nil {
		t.Fatal("expected user, got nil")
	}
	if user.ID != 1 {
		t.Fatalf("expected ID=1, got %d", user.ID)
	}
	if user.Name != "admin" {
		t.Fatalf("expected Name=admin, got %s", user.Name)
	}
	if user.Password != "secret" {
		t.Fatalf("expected Password=secret, got %s", user.Password)
	}
	if user.Role != "admin" {
		t.Fatalf("expected Role=admin, got %s", user.Role)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestPostgresUserRepository_FindByUsername_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	repo := NewPostgresUserRepository(db)

	const username = "missing"

	mock.ExpectQuery(`SELECT user_id, user_name, user_password, user_role FROM users WHERE user_name = \$1`).
		WithArgs(username).
		WillReturnError(sql.ErrNoRows)

	user, err := repo.FindByUsername(context.Background(), username)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user != nil {
		t.Fatalf("expected nil user, got %+v", user)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestPostgresUserRepository_FindByUsername_QueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	repo := NewPostgresUserRepository(db)

	const username = "admin"
	dbErr := errors.New("db failure")

	mock.ExpectQuery(`SELECT user_id, user_name, user_password, user_role FROM users WHERE user_name = \$1`).
		WithArgs(username).
		WillReturnError(dbErr)

	user, err := repo.FindByUsername(context.Background(), username)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if user != nil {
		t.Fatalf("expected nil user, got %+v", user)
	}
	if !strings.Contains(err.Error(), "find user by username") {
		t.Fatalf("expected wrapped error, got %v", err)
	}
	if !strings.Contains(err.Error(), "db failure") {
		t.Fatalf("expected original error text, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}
