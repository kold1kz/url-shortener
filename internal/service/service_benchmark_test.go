package service

import (
	"context"
	"sync"
	"testing"
	"url-shortener/internal/model"
	"url-shortener/internal/repository"
)

// repo для бенчмарка (in-memory, без диска)
type benchRepo struct {
	mu       sync.RWMutex
	byID     map[string]*model.URL
	byOrig   map[string]*model.URL
	byUserID map[string][]*model.URL
}

func newBenchRepo() *benchRepo {
	return &benchRepo{
		byID:     map[string]*model.URL{},
		byOrig:   map[string]*model.URL{},
		byUserID: map[string][]*model.URL{},
	}
}

func (r *benchRepo) Create(u *model.URL) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	// emulate uniqueness by original
	if _, ok := r.byOrig[u.Original]; ok {
		return &repository.URLConflictError{ExistingURL: r.byOrig[u.Original]}
	}
	r.byID[u.ID] = u
	r.byOrig[u.Original] = u
	if u.UserID != "" {
		r.byUserID[u.UserID] = append(r.byUserID[u.UserID], u)
	}
	return nil
}

func (r *benchRepo) FindByID(id string) (*model.URL, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.byID[id], nil
}

func (r *benchRepo) FindByOriginalURL(original string) (*model.URL, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.byOrig[original], nil
}

func (r *benchRepo) CreateBatch(urls []*model.URL) error { // для интерфейса
	for _, u := range urls {
		_ = r.Create(u)
	}
	return nil
}

func (r *benchRepo) FindByUserID(userID string) ([]*model.URL, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	src := r.byUserID[userID]
	dst := make([]*model.URL, len(src))
	copy(dst, src)
	return dst, nil
}

func (r *benchRepo) MarkAsDeleted(ctx context.Context, userID string, ids []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, id := range ids {
		if u, ok := r.byID[id]; ok && u.UserID == userID {
			u.IsDeleted = true
		}
	}
	return nil
}

func BenchmarkService_ShortenURL_Unique(b *testing.B) {
	repo := newBenchRepo()
	svc := NewURLService(repo, "http://localhost:8080")
	defer svc.Close()

	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		orig := "https://example.com/path/" + itoa(i)
		_, err := svc.ShortenURL(ctx, orig, "u1")
		if err != nil {
			b.Fatal(err)
		}
	}
}

// маленький itoa без fmt (для бенча)
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
