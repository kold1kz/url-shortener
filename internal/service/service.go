package service

import (
	"crypto/rand"
	"encoding/base64"
	"url-shortener/internal/model"
	"url-shortener/internal/repository"
)

type URLService interface {
	ShortenURL(original string) (*model.URL, error)
	GetOriginalURL(id string) (string, error)
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

func (s *urlService) ShortenURL(originalURL string) (*model.URL, error) {

	existingURL, err := s.repo.FindByOriginalURL(originalURL)
	if err != nil {
		return nil, err
	}
	if existingURL != nil {
		return existingURL, nil
	}

	var id string
	for {
		id = generateID(10)
		u, _ := s.repo.FindByID(id)
		if u == nil {
			break
		}
	}

	url := &model.URL{
		ID:       id,
		Original: originalURL,
		Short:    s.baseURL + "/" + id,
	}

	if err := s.repo.Create(url); err != nil {
		return nil, err
	}

	return url, nil
}

func (s *urlService) GetOriginalURL(id string) (string, error) {
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
