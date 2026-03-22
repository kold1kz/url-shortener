package service

import (
	"context"
	"fmt"
	"strconv"

	"url-shortener/internal/auth"
	"url-shortener/internal/repository"
)

type LoginService interface {
	Login(ctx context.Context, username, password string) (string, error)
}

type loginService struct {
	users repository.UserRepository
}

func NewLoginService(users repository.UserRepository) LoginService {
	return &loginService{users: users}
}

func (s *loginService) Login(ctx context.Context, username, password string) (string, error) {
	user, err := s.users.FindByUsername(ctx, username)
	if err != nil {
		return "", fmt.Errorf("find user by username: %w", err)
	}
	if user == nil {
		return "", fmt.Errorf("invalid credentials")
	}
	if user.Password != password {
		return "", fmt.Errorf("invalid credentials")
	}

	token, err := auth.NewTokenForUserID(strconv.Itoa(user.ID))
	if err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}

	return token, nil
}
