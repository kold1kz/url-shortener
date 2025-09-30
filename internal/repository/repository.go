package repository

import (
	"fmt"
	"sync"
	"url-shortener/internal/model"
)

type URLRepository interface {
	Create(url *model.URL) error
	FindByID(id string) (*model.URL, error)
	FindByOriginalURL(originalURL string) (*model.URL, error)
}

type InMemoryURLRepository struct {
	mu           sync.RWMutex
	data         map[string]*model.URL
	originalURLs map[string]string
}

func NewInMemoryURLRepository() *InMemoryURLRepository {
	return &InMemoryURLRepository{
		data:         make(map[string]*model.URL),
		originalURLs: make(map[string]string),
	}
}

func (r *InMemoryURLRepository) Create(url *model.URL) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.originalURLs[url.Original]; exists {
		return fmt.Errorf("URL already exists")
	}
	r.data[url.ID] = url
	r.originalURLs[url.Original] = url.ID
	return nil
}

func (r *InMemoryURLRepository) FindByID(id string) (*model.URL, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	url, exists := r.data[id]
	if !exists {
		return nil, nil
	}
	return url, nil
}

func (r *InMemoryURLRepository) FindByOriginalURL(originalURL string) (*model.URL, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	id, exists := r.originalURLs[originalURL]
	if !exists {
		return nil, nil
	}

	url, exists := r.data[id]
	if !exists {
		return nil, nil
	}
	return url, nil
}
