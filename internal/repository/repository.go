package repository

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
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

type FileURLRepository struct {
	mu           sync.RWMutex
	data         map[string]*model.URL
	originalURLs map[string]string
	filePath     string
}

func NewFileURLRepository(filePath string) (*FileURLRepository, error) {
	repo := &FileURLRepository{
		data:         make(map[string]*model.URL),
		originalURLs: make(map[string]string),
		filePath:     filePath,
	}

	// Загружаем данные из файла при инициализации
	if err := repo.loadFromFile(); err != nil {
		return nil, fmt.Errorf("failed to load data from file: %w", err)
	}
	return repo, nil
}

func (r *FileURLRepository) loadFromFile() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, err := os.Stat(r.filePath); os.IsNotExist(err) {
		log.Printf("file %s does not exist", r.filePath)
		return nil
	}

	data, err := os.ReadFile(r.filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	if len(data) == 0 {
		return nil
	}

	var urls []model.URL
	if err := json.Unmarshal(data, &urls); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	for i := range urls {
		url := &urls[i]
		r.data[url.ID] = url
		r.originalURLs[url.Original] = url.ID
	}

	return nil
}

func (r *FileURLRepository) saveToFile() error {
	urls := make([]model.URL, 0, len(r.data))
	for _, url := range r.data {
		urls = append(urls, *url)
	}

	data, err := json.MarshalIndent(urls, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	dir := filepath.Dir(r.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(r.filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

func (r *FileURLRepository) Create(url *model.URL) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.originalURLs[url.Original]; exists {
		return fmt.Errorf("URL already exists")
	}

	r.data[url.ID] = url
	r.originalURLs[url.Original] = url.ID

	if err := r.saveToFile(); err != nil {
		// Откатываем изменения если сохранение не удалось
		delete(r.data, url.ID)
		delete(r.originalURLs, url.Original)
		return fmt.Errorf("failed to save URL to file: %w", err)
	}

	return nil
}

func (r *FileURLRepository) FindByID(id string) (*model.URL, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	url, exists := r.data[id]
	if !exists {
		return nil, nil
	}

	return url, nil
}

func (r *FileURLRepository) FindByOriginalURL(originalURL string) (*model.URL, error) {
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

func (r *FileURLRepository) Close() error {
	return r.saveToFile()
}
