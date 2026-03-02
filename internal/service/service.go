package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"hash/fnv"
	"log"
	"sync"
	"time"
	"url-shortener/internal/model"
	"url-shortener/internal/repository"
)

var (
	// ErrURLAlreadyExists возвращается, если исходный URL уже сохранён в системе.
	// В этом случае сервис может вернуть существующую короткую ссылку.
	ErrURLAlreadyExists = errors.New("url already exists")

	// ErrURLDeleted возвращается при попытке получить исходный URL,
	// который помечен как удалённый.
	ErrURLDeleted = errors.New("url deleted")
)

// URLService описывает бизнес-операции сервиса сокращения URL.
//
// Реализация должна быть потокобезопасной.
type URLService interface {
	// ShortenURL создаёт сокращённый URL для original и связывает его с userID.
	//
	// Если URL уже существует, возвращает существующую запись и ErrURLAlreadyExists.
	ShortenURL(ctx context.Context, original string, userID string) (*model.URL, error)

	// GetOriginalURL возвращает исходный URL по короткому идентификатору.
	//
	// Возвращает ("", nil), если запись не найдена.
	// Возвращает ErrURLDeleted, если URL помечен как удалённый.
	GetOriginalURL(ctx context.Context, id string) (string, error)

	// ShortenURLBatch сокращает список URL одним запросом.
	//
	// Корреляция запроса и ответа выполняется через CorrelationID.
	ShortenURLBatch(ctx context.Context, batch []model.BatchRequest, userID string) ([]model.BatchResponse, error)

	// FindByUserID возвращает список URL, созданных пользователем.
	FindByUserID(ctx context.Context, userID string) ([]*model.URL, error)

	// DeleteUserURLs принимает список идентификаторов URL для удаления.
	//
	// Реализация выполняет удаление асинхронно (batched mark-as-deleted).
	DeleteUserURLs(ctx context.Context, userID string, ids []string) error

	// Close завершает фоновые процессы сервиса и освобождает ресурсы.
	Close() error
}

type deleteRequest struct {
	userID string
	ids    []string
}

type urlService struct {
	repo    repository.URLRepository
	baseURL string

	inputChs []chan deleteRequest
	bufferCh <-chan deleteRequest
	stopCh   chan struct{}
}

func fanInDeleteRequests(done <-chan struct{}, chans ...<-chan deleteRequest) <-chan deleteRequest {
	out := make(chan deleteRequest)

	var wg sync.WaitGroup
	for _, ch := range chans {
		c := ch
		wg.Add(1)
		go func() {
			defer wg.Done()
			for req := range c {
				select {
				case <-done:
					return
				case out <- req:
				}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}

func fnv32(s string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return h.Sum32()
}

// NewURLService создаёт реализацию URLService.
//
// baseURL используется для формирования коротких ссылок (Short).
// Для обработки запросов удаления запускается фоновый worker, который
// агрегирует удаления и периодически вызывает repository.MarkAsDeleted.
func NewURLService(repo repository.URLRepository, baseURL string) URLService {
	const workers = 4

	s := &urlService{
		repo:     repo,
		baseURL:  baseURL,
		inputChs: make([]chan deleteRequest, workers),
		stopCh:   make(chan struct{}),
	}

	inputs := make([]<-chan deleteRequest, 0, workers)
	for i := 0; i < workers; i++ {
		ch := make(chan deleteRequest, 64)
		s.inputChs[i] = ch
		inputs = append(inputs, ch)
	}

	s.bufferCh = fanInDeleteRequests(s.stopCh, inputs...)

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
	var u *model.URL
	for {
		id = generateID(10)
		u, err = s.repo.FindByID(id)
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

	idx := fnv32(userID) % uint32(len(s.inputChs))

	select {
	case s.inputChs[idx] <- req:
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
		case req, ok := <-s.bufferCh:
			if !ok {
				if len(batches) > 0 {
					flush()
				}
				return
			}
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
	for _, ch := range s.inputChs {
		close(ch)
	}
	close(s.stopCh)
	return nil
}
