package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"url-shortener/internal/auth"
	"url-shortener/internal/repository"
)

type stubUserRepo struct {
	user *repository.User
	err  error
}

func (r *stubUserRepo) FindByUsername(ctx context.Context, username string) (*repository.User, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.user, nil
}

func TestNewLoginService(t *testing.T) {
	repo := &stubUserRepo{}
	svc := NewLoginService(repo)
	if svc == nil {
		t.Fatal("expected service, got nil")
	}
}

func TestLoginService_Login_Success(t *testing.T) {
	repo := &stubUserRepo{
		user: &repository.User{
			ID:       42,
			Name:     "admin",
			Password: "secret",
			Role:     "admin",
		},
	}

	svc := NewLoginService(repo)

	token, err := svc.Login(context.Background(), "admin", "secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	userID, err := auth.GetUserIDFromCookieStrict(token)
	if err != nil {
		t.Fatalf("token should be valid, got error: %v", err)
	}
	if userID != "42" {
		t.Fatalf("expected userID=42, got %q", userID)
	}
}

func TestLoginService_Login_UserNotFound(t *testing.T) {
	repo := &stubUserRepo{
		user: nil,
	}

	svc := NewLoginService(repo)

	token, err := svc.Login(context.Background(), "missing", "secret")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if token != "" {
		t.Fatalf("expected empty token, got %q", token)
	}
	if err.Error() != "invalid credentials" {
		t.Fatalf("expected invalid credentials, got %v", err)
	}
}

func TestLoginService_Login_WrongPassword(t *testing.T) {
	repo := &stubUserRepo{
		user: &repository.User{
			ID:       7,
			Name:     "admin",
			Password: "correct-password",
			Role:     "admin",
		},
	}

	svc := NewLoginService(repo)

	token, err := svc.Login(context.Background(), "admin", "wrong-password")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if token != "" {
		t.Fatalf("expected empty token, got %q", token)
	}
	if err.Error() != "invalid credentials" {
		t.Fatalf("expected invalid credentials, got %v", err)
	}
}

func TestLoginService_Login_RepositoryError(t *testing.T) {
	repo := &stubUserRepo{
		err: errors.New("db down"),
	}

	svc := NewLoginService(repo)

	token, err := svc.Login(context.Background(), "admin", "secret")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if token != "" {
		t.Fatalf("expected empty token, got %q", token)
	}
	if !strings.Contains(err.Error(), "find user by username") {
		t.Fatalf("expected wrapped repo error, got %v", err)
	}
	if !strings.Contains(err.Error(), "db down") {
		t.Fatalf("expected original repo error text, got %v", err)
	}
}
