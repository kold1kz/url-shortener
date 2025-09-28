package repository

import (
	"sync"
	"url-shortener/internal/model"
)

type URLRepository interface {
	Create(url *model.URL) error
	FindByID(id string) (*model.URL, error)
	FindByOriginalURL(originalURL string) (*model.URL, error)
}

type InMemoryURLRepository struct {
	mu   sync.RWMutex
	data map[string]*model.URL
}

func NewInMemoryURLRepository() *InMemoryURLRepository {
	return &InMemoryURLRepository{
		data: make(map[string]*model.URL),
	}
}

func (r *InMemoryURLRepository) Create(url *model.URL) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data[url.ID] = url
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
	for _, url := range r.data {
		if url.Original == originalURL {
			return url, nil
		}
	}
	return nil, nil
}
