package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"url-shortener/internal/model"
	"url-shortener/internal/repository"
)

// controlledRepo wraps a real repo and lets us inject behaviors/errors.
// Embedding guarantees we implement repository.URLRepository even if it changes (e.g. CreateBatch added).
type controlledRepo struct {
	repository.URLRepository

	mu sync.Mutex

	findByOriginalErr error
	findByIDErr       error
	createErr         error
	findByUserErr     error

	// observe MarkAsDeleted calls
	markCalls int64
	lastMark  struct {
		userID string
		ids    []string
	}
	markErr error
}

func (r *controlledRepo) FindByOriginalURL(original string) (*model.URL, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.findByOriginalErr != nil {
		return nil, r.findByOriginalErr
	}
	return r.URLRepository.FindByOriginalURL(original)
}

func (r *controlledRepo) FindByID(id string) (*model.URL, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.findByIDErr != nil {
		return nil, r.findByIDErr
	}
	return r.URLRepository.FindByID(id)
}

func (r *controlledRepo) Create(u *model.URL) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.createErr != nil {
		return r.createErr
	}
	return r.URLRepository.Create(u)
}

func (r *controlledRepo) FindByUserID(userID string) ([]*model.URL, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.findByUserErr != nil {
		return nil, r.findByUserErr
	}
	return r.URLRepository.FindByUserID(userID)
}

func (r *controlledRepo) MarkAsDeleted(ctx context.Context, userID string, ids []string) error {
	atomic.AddInt64(&r.markCalls, 1)
	r.mu.Lock()
	r.lastMark.userID = userID
	r.lastMark.ids = append([]string(nil), ids...)
	err := r.markErr
	r.mu.Unlock()
	return err
}

func waitUntil(t *testing.T, d time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timeout waiting condition")
}

func TestShortenURL_Existing_ReturnsConflict(t *testing.T) {
	base := repository.NewInMemoryURLRepository()
	repo := &controlledRepo{URLRepository: base}

	// pre-create existing
	_ = base.Create(&model.URL{
		ID:       "x",
		Original: "https://example.com",
		Short:    "http://b/x",
		UserID:   "u1",
	})

	svc := NewURLService(repo, "http://b")

	u, err := svc.ShortenURL(context.Background(), "https://example.com", "u2")
	if !errors.Is(err, ErrURLAlreadyExists) {
		t.Fatalf("expected ErrURLAlreadyExists, got %v", err)
	}
	if u == nil || u.ID != "x" {
		t.Fatalf("expected existing url, got %+v", u)
	}
}

func TestShortenURL_FindByOriginalError(t *testing.T) {
	base := repository.NewInMemoryURLRepository()
	repo := &controlledRepo{URLRepository: base, findByOriginalErr: errors.New("db")}

	svc := NewURLService(repo, "http://b")

	_, err := svc.ShortenURL(context.Background(), "https://x", "u1")
	if err == nil || err.Error() != "db" {
		t.Fatalf("expected db error, got %v", err)
	}
}

func TestShortenURL_FindByIDError(t *testing.T) {
	base := repository.NewInMemoryURLRepository()
	repo := &controlledRepo{URLRepository: base, findByIDErr: errors.New("findid")}

	svc := NewURLService(repo, "http://b")

	_, err := svc.ShortenURL(context.Background(), "https://x", "u")
	if err == nil || err.Error() != "findid" {
		t.Fatalf("expected findid error, got %v", err)
	}
}

func TestShortenURL_CreateConflict_ReturnsErrURLAlreadyExists(t *testing.T) {
	base := repository.NewInMemoryURLRepository()
	repo := &controlledRepo{URLRepository: base}

	// repository.IsURLConflictError(err) must be true for this type
	repo.createErr = &repository.URLConflictError{
		ExistingURL: &model.URL{
			ID:       "old",
			Original: "https://x",
			Short:    "http://b/old",
			UserID:   "u1",
		},
	}

	svc := NewURLService(repo, "http://b")

	u, err := svc.ShortenURL(context.Background(), "https://x", "u2")
	if !errors.Is(err, ErrURLAlreadyExists) {
		t.Fatalf("expected ErrURLAlreadyExists, got %v", err)
	}
	if u == nil || u.ID != "old" {
		t.Fatalf("expected returned ExistingURL, got %+v", u)
	}
}

func TestShortenURL_CreateGenericError_Wrapped(t *testing.T) {
	base := repository.NewInMemoryURLRepository()
	repo := &controlledRepo{URLRepository: base, createErr: errors.New("boom")}

	svc := NewURLService(repo, "http://b")

	_, err := svc.ShortenURL(context.Background(), "https://x", "u")
	if err == nil || !strings.Contains(err.Error(), "failed to create URL") {
		t.Fatalf("expected wrapped error, got %v", err)
	}
}

func TestFindByUserID_DelegatesAndError(t *testing.T) {
	base := repository.NewInMemoryURLRepository()
	repo := &controlledRepo{URLRepository: base}

	_ = base.Create(&model.URL{ID: "1", Original: "o1", Short: "s1", UserID: "u"})
	svc := NewURLService(repo, "http://b")

	urls, err := svc.FindByUserID(context.Background(), "u")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(urls) != 1 || urls[0].ID != "1" {
		t.Fatalf("unexpected urls: %+v", urls)
	}

	repo.findByUserErr = errors.New("x")
	_, err = svc.FindByUserID(context.Background(), "u")
	if err == nil || err.Error() != "x" {
		t.Fatalf("expected error, got %v", err)
	}
}

func TestGetOriginalURL_Variants(t *testing.T) {
	base := repository.NewInMemoryURLRepository()
	repo := &controlledRepo{URLRepository: base}
	svc := NewURLService(repo, "http://b")

	// nil -> "", nil
	orig, err := svc.GetOriginalURL(context.Background(), "nope")
	if err != nil || orig != "" {
		t.Fatalf("expected empty+nil, got orig=%q err=%v", orig, err)
	}

	// saved -> original
	_ = base.Create(&model.URL{ID: "a", Original: "https://a", Short: "http://b/a", UserID: "u"})
	orig, err = svc.GetOriginalURL(context.Background(), "a")
	if err != nil || orig != "https://a" {
		t.Fatalf("expected https://a, got orig=%q err=%v", orig, err)
	}

	// deleted -> ErrURLDeleted
	u, _ := base.FindByID("a")
	u.IsDeleted = true
	orig, err = svc.GetOriginalURL(context.Background(), "a")
	if !errors.Is(err, ErrURLDeleted) || orig != "" {
		t.Fatalf("expected ErrURLDeleted, got orig=%q err=%v", orig, err)
	}

	// repo error
	repo.findByIDErr = errors.New("db")
	orig, err = svc.GetOriginalURL(context.Background(), "a")
	if err == nil || err.Error() != "db" {
		t.Fatalf("expected db err, got orig=%q err=%v", orig, err)
	}
}

func TestShortenURLBatch_Empty(t *testing.T) {
	base := repository.NewInMemoryURLRepository()
	repo := &controlledRepo{URLRepository: base}
	svc := NewURLService(repo, "http://b")

	_, err := svc.ShortenURLBatch(context.Background(), nil, "u")
	if err == nil || err.Error() != "empty batch" {
		t.Fatalf("expected empty batch error, got %v", err)
	}
}

func TestShortenURLBatch_Success(t *testing.T) {
	base := repository.NewInMemoryURLRepository()
	repo := &controlledRepo{URLRepository: base}
	svc := NewURLService(repo, "http://b")

	batch := []model.BatchRequest{
		{CorrelationID: "1", OriginalURL: "https://1"},
		{CorrelationID: "2", OriginalURL: "https://2"},
	}

	resp, err := svc.ShortenURLBatch(context.Background(), batch, "u1")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(resp) != 2 {
		t.Fatalf("unexpected resp len: %d", len(resp))
	}
	if resp[0].CorrelationID != "1" || !strings.HasPrefix(resp[0].ShortURL, "http://b/") {
		t.Fatalf("unexpected resp[0]: %+v", resp[0])
	}
}

func TestShortenURLBatch_ErrorWrappedWithCorrelation(t *testing.T) {
	base := repository.NewInMemoryURLRepository()
	repo := &controlledRepo{URLRepository: base}
	svc := NewURLService(repo, "http://b")

	repo.findByOriginalErr = errors.New("x") // makes ShortenURL fail

	_, err := svc.ShortenURLBatch(context.Background(), []model.BatchRequest{
		{CorrelationID: "c1", OriginalURL: "https://1"},
	}, "u")
	if err == nil || !strings.Contains(err.Error(), "correlation_id c1") {
		t.Fatalf("expected correlation wrapper, got %v", err)
	}
}

func TestDeleteUserURLs_EmptyIDs_NoOp(t *testing.T) {
	base := repository.NewInMemoryURLRepository()
	repo := &controlledRepo{URLRepository: base}
	svc := NewURLService(repo, "http://b")

	if err := svc.DeleteUserURLs(context.Background(), "u", nil); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestDeleteUserURLs_ContextCanceled_WhenShardChannelBlocked(t *testing.T) {
	repo := repository.NewInMemoryURLRepository()
	s := NewURLService(repo, "http://b").(*urlService)

	// Выбираем shard
	userID := "some-user"
	idx := fnv32(userID) % uint32(len(s.inputChs))

	// Подменяем канал шарда на unbuffered, на который НИКТО не читает -> send будет блокироваться
	blockingCh := make(chan deleteRequest)
	s.inputChs[idx] = blockingCh

	// Контекст сразу отменяем (или можно cancel сразу после старта)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := s.DeleteUserURLs(ctx, userID, []string{"id1"})
	if err == nil {
		t.Fatalf("expected ctx error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}

	_ = s.Close()
}

func TestDeleteWorker_FlushOnStop(t *testing.T) {
	base := repository.NewInMemoryURLRepository()
	repo := &controlledRepo{URLRepository: base}

	s := NewURLService(repo, "http://b").(*urlService)

	// enqueue delete request
	if err := s.DeleteUserURLs(context.Background(), "u1", []string{"a", "b"}); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	// ДАЁМ ВОРКЕРУ ВРЕМЯ ПОЛУЧИТЬ req В batches (иначе Close может выиграть гонку)
	time.Sleep(150 * time.Millisecond)

	// Триггерим flush-on-stop детерминированно
	close(s.stopCh)

	// Закрываем inputChs, чтобы fan-in корректно завершился
	for _, ch := range s.inputChs {
		// безопасно закрыть: если вдруг уже закрыто — ловим панику
		func() {
			defer func() { _ = recover() }()
			close(ch)
		}()
	}

	waitUntil(t, 5*time.Second, func() bool {
		return atomic.LoadInt64(&repo.markCalls) > 0
	})

	repo.mu.Lock()
	defer repo.mu.Unlock()
	if repo.lastMark.userID != "u1" {
		t.Fatalf("unexpected userID: %s", repo.lastMark.userID)
	}
	if len(repo.lastMark.ids) != 2 {
		t.Fatalf("unexpected ids: %v", repo.lastMark.ids)
	}
}

func TestDeleteWorker_MarkAsDeletedError_DoesNotHang(t *testing.T) {
	base := repository.NewInMemoryURLRepository()
	repo := &controlledRepo{URLRepository: base, markErr: errors.New("fail-mark")}

	s := NewURLService(repo, "http://b").(*urlService)

	_ = s.DeleteUserURLs(context.Background(), "u2", []string{"x"})

	// Ждём чуть больше flushInterval (1s), чтобы тикер точно сработал
	waitUntil(t, 5*time.Second, func() bool {
		return atomic.LoadInt64(&repo.markCalls) > 0
	})

	// cleanup
	_ = s.Close()
}

func TestFanInDeleteRequests_StopsOnDone(t *testing.T) {
	done := make(chan struct{})
	ch := make(chan deleteRequest, 1)

	out := fanInDeleteRequests(done, ch)

	close(done)
	close(ch)

	// out must close eventually
	select {
	case <-out:
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting out close")
	}
}
