package service

import (
	"crypto/rand"
	"encoding/base64"
	"url-shortener/internal/model"
	"url-shortener/internal/repository"
)

type URLServiceI interface {
	ShortenURL(original string) (*model.URL, error)
	GetOriginalURL(id string) (string, error)
}
type URLService struct {
	repo    repository.URLRepository
	baseURL string
}

func NewURLService(repo repository.URLRepository, baseURL string) *URLService {
	return &URLService{
		repo:    repo,
		baseURL: baseURL,
	}
}

func (s *URLService) ShortenURL(originalURL string) (*model.URL, error) {

	existingURL, _ := s.repo.FindByOriginalURL(originalURL)
	if existingURL != nil {
		return existingURL, nil
	}

	id := generateID(10)

	url := &model.URL{
		ID:       id,
		Original: originalURL,
		Short:    s.baseURL + "/" + id,
	}

	err := s.repo.Create(url)
	if err != nil {
		return nil, err
	}

	return url, nil
}

func (s *URLService) GetOriginalURL(id string) (string, error) {
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
	rand.Read(bytes)
	return base64.URLEncoding.EncodeToString(bytes)[:length]
}
