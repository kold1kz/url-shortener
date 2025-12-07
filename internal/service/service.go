package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"time"
	"url-shortener/internal/model"
	"url-shortener/internal/repository"
)

var (
	ErrURLAlreadyExists = errors.New("url already exists")
	ErrURLDeleted       = errors.New("url deleted")
)

type URLService interface {
	ShortenURL(ctx context.Context, original string, userID string) (*model.URL, error)
	GetOriginalURL(ctx context.Context, id string) (string, error)
	ShortenURLBatch(ctx context.Context, batch []model.BatchRequest, userID string) ([]model.BatchResponse, error)
	FindByUserID(ctx context.Context, userID string) ([]*model.URL, error)
	DeleteUserURLs(ctx context.Context, userID string, ids []string) error
	Close() error
}

type deleteRequest struct {
	userID string
	ids    []string
}

type urlService struct {
	repo     repository.URLRepository
	baseURL  string
	deleteCh chan deleteRequest
	stopCh   chan struct{}
}

func NewURLService(repo repository.URLRepository, baseURL string) URLService {
	s := &urlService{
		repo:     repo,
		baseURL:  baseURL,
		deleteCh: make(chan deleteRequest, 1024),
		stopCh:   make(chan struct{}),
	}

	go s.deleteWorker()

	return s
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
	if url.IsDeleted {
		return "", ErrURLDeleted
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

func (s *urlService) DeleteUserURLs(ctx context.Context, userID string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	req := deleteRequest{
		userID: userID,
		ids:    ids,
	}

	select {
	case s.deleteCh <- req:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *urlService) deleteWorker() {
	const flushInterval = time.Second
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	batches := make(map[string][]string)

	flush := func() {
		for userID, ids := range batches {
			if len(ids) == 0 {
				continue
			}
			if err := s.repo.MarkAsDeleted(context.Background(), userID, ids); err != nil {
				log.Println("failed to mark URLs as deleted", "userID", userID, "ids", ids, "err", err)
				continue
			}
		}
		batches = make(map[string][]string)
	}

	for {
		select {
		case req := <-s.deleteCh:
			if len(req.ids) == 0 {
				continue
			}
			batches[req.userID] = append(batches[req.userID], req.ids...)

		case <-ticker.C:
			if len(batches) == 0 {
				continue
			}
			flush()

		case <-s.stopCh:
			if len(batches) > 0 {
				flush()
			}
			return
		}
	}
}

func (s *urlService) Close() error {
	close(s.stopCh)
	return nil
}
