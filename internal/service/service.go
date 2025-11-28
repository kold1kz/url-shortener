package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"url-shortener/internal/model"
	"url-shortener/internal/repository"
)

var ErrURLAlreadyExists = errors.New("url already exists")

type URLService interface {
	ShortenURL(ctx context.Context, original string, userID string) (*model.URL, error)
	GetOriginalURL(ctx context.Context, id string) (string, error)
	ShortenURLBatch(ctx context.Context, batch []model.BatchRequest, userID string) ([]model.BatchResponse, error)
	FindByUserID(ctx context.Context, userID string) ([]*model.URL, error)
}

type urlService struct {
	repo    repository.URLRepository
	baseURL string
}

func NewURLService(repo repository.URLRepository, baseURL string) URLService {
	return &urlService{
		repo:    repo,
		baseURL: baseURL,
	}
}

func (s *urlService) ShortenURL(ctx context.Context, originalURL, userID string) (*model.URL, error) {
	existingURL, err := s.repo.FindByOriginalURL(originalURL)
	if err != nil {
		return nil, err
	}
	if existingURL != nil {
		return existingURL, ErrURLAlreadyExists
	}

	var id string
	for {
		id = generateID(10)
		u, err := s.repo.FindByID(id)
		if err != nil {
			return nil, err
		}
		if u == nil {
			break
		}
	}

	url := &model.URL{
		ID:       id,
		Original: originalURL,
		Short:    s.baseURL + "/" + id,
		UserID:   userID, // 👈 важное место
	}

	err = s.repo.Create(url)
	if err != nil {
		if repository.IsURLConflictError(err) {
			if conflictErr, ok := err.(*repository.URLConflictError); ok && conflictErr.ExistingURL != nil {
				return conflictErr.ExistingURL, ErrURLAlreadyExists
			}
			return nil, ErrURLAlreadyExists
		}
		return nil, fmt.Errorf("failed to create URL: %w", err)
	}

	return url, nil
}

func (s *urlService) FindByUserID(ctx context.Context, userID string) ([]*model.URL, error) {
	return s.repo.FindByUserID(userID)
}

func (s *urlService) GetOriginalURL(ctx context.Context, id string) (string, error) {
	url, err := s.repo.FindByID(id)
	if err != nil {
		return "", err
	}
	if url == nil {
		return "", nil
	}
	return url.Original, nil
}

func generateID(length int) string {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(bytes)[:length]
}

func (s *urlService) ShortenURLBatch(ctx context.Context, batch []model.BatchRequest, userID string) ([]model.BatchResponse, error) {
	if len(batch) == 0 {
		return nil, fmt.Errorf("empty batch")
	}

	responses := make([]model.BatchResponse, 0, len(batch))

	for _, item := range batch {
		url, err := s.ShortenURL(ctx, item.OriginalURL, userID)
		if err != nil {
			return nil, fmt.Errorf("failed to shorten URL for correlation_id %s: %w", item.CorrelationID, err)
		}

		responses = append(responses, model.BatchResponse{
			CorrelationID: item.CorrelationID,
			ShortURL:      url.Short,
		})
	}

	return responses, nil
}
