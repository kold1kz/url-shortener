package repository

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"url-shortener/internal/model"
)

func makeReadOnlyDir(t *testing.T) (dir string, filePath string) {
	t.Helper()

	base := t.TempDir()
	locked := filepath.Join(base, "locked")
	if err := os.MkdirAll(locked, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	// убираем write-права на каталог
	if err := os.Chmod(locked, 0555); err != nil {
		t.Fatalf("Chmod: %v", err)
	}

	return locked, filepath.Join(locked, "data.json")
}

func makeBrokenFilePath(t *testing.T) (goodPath string, brokenPath string) {
	t.Helper()

	base := t.TempDir()
	goodPath = filepath.Join(base, "data.json") // валидный файл

	// brokenPath = директория (WriteFile туда всегда падает)
	brokenDir := filepath.Join(base, "broken")
	if err := os.MkdirAll(brokenDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	return goodPath, brokenDir
}

func TestInMemoryRepository_BasicFlow(t *testing.T) {
	repo := NewInMemoryURLRepository()

	u1 := &model.URL{ID: "1", Original: "https://a.com", Short: "http://s/1", UserID: "u1"}
	u2 := &model.URL{ID: "2", Original: "https://b.com", Short: "http://s/2", UserID: "u1"}
	u3 := &model.URL{ID: "3", Original: "https://c.com", Short: "http://s/3", UserID: ""}

	// Create
	if err := repo.Create(u1); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := repo.Create(u2); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := repo.Create(u3); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// duplicate Original should error
	if err := repo.Create(&model.URL{ID: "x", Original: u1.Original, Short: "http://s/x", UserID: "u1"}); err == nil {
		t.Fatalf("expected error on duplicate original URL")
	}

	// FindByID
	got, err := repo.FindByID("1")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got == nil || got.Original != u1.Original {
		t.Fatalf("FindByID got=%v", got)
	}

	// FindByID not found -> nil,nil
	got, err = repo.FindByID("nope")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil for missing id")
	}

	// FindByOriginalURL
	got, err = repo.FindByOriginalURL("https://b.com")
	if err != nil {
		t.Fatalf("FindByOriginalURL: %v", err)
	}
	if got == nil || got.ID != "2" {
		t.Fatalf("FindByOriginalURL got=%v", got)
	}

	// FindByOriginalURL not found -> nil,nil
	got, err = repo.FindByOriginalURL("https://missing.com")
	if err != nil {
		t.Fatalf("FindByOriginalURL: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil for missing original")
	}

	// FindByUserID returns copy
	list, err := repo.FindByUserID("u1")
	if err != nil {
		t.Fatalf("FindByUserID: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 urls for user, got %d", len(list))
	}

	orig, _ := repo.FindByUserID("u1")
	if len(orig) != 2 {
		t.Fatalf("expected 2 urls for user, got %d", len(orig))
	}

	// Меняем слайс локально: добавляем элемент.
	// Если репо вернул шареный слайс, это могло бы “протечь” внутрь.
	orig = append(orig, &model.URL{ID: "x", Original: "https://x.com", Short: "http://s/x", UserID: "u1"})
	orig2 := append(orig, &model.URL{ID: "x"})
	_ = orig2
	again, _ := repo.FindByUserID("u1")
	if len(again) != 2 {
		t.Fatalf("expected FindByUserID to return independent slice; got len=%d", len(again))
	}

	// MarkAsDeleted: only matching user
	if err := repo.MarkAsDeleted(context.Background(), "u1", []string{"1", "3"}); err != nil {
		t.Fatalf("MarkAsDeleted: %v", err)
	}

	u, _ := repo.FindByID("1")
	if u == nil || !u.IsDeleted {
		t.Fatalf("expected url 1 deleted")
	}

	u, _ = repo.FindByID("3")
	if u == nil || u.IsDeleted {
		t.Fatalf("expected url 3 NOT deleted because user mismatch/empty")
	}

	// MarkAsDeleted with empty ids -> nil
	if err := repo.MarkAsDeleted(context.Background(), "u1", nil); err != nil {
		t.Fatalf("MarkAsDeleted empty: %v", err)
	}
}

func TestInMemoryRepository_CreateBatch(t *testing.T) {
	repo := NewInMemoryURLRepository()

	seed := &model.URL{ID: "1", Original: "https://a.com", Short: "http://s/1", UserID: "u1"}
	if err := repo.Create(seed); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Batch: one duplicate original (should be skipped), one new, one empty UserID
	batch := []*model.URL{
		{ID: "dup", Original: "https://a.com", Short: "http://s/dup", UserID: "u1"},
		{ID: "2", Original: "https://b.com", Short: "http://s/2", UserID: "u1"},
		{ID: "3", Original: "https://c.com", Short: "http://s/3", UserID: ""},
	}

	if err := repo.CreateBatch(batch); err != nil {
		t.Fatalf("CreateBatch: %v", err)
	}

	u, _ := repo.FindByOriginalURL("https://a.com")
	if u == nil || u.ID != "1" {
		t.Fatalf("expected original https://a.com to stay with ID=1, got=%v", u)
	}

	u, _ = repo.FindByID("2")
	if u == nil || u.Original != "https://b.com" {
		t.Fatalf("expected url 2 created")
	}

	u, _ = repo.FindByID("3")
	if u == nil || u.UserID != "" {
		t.Fatalf("expected url 3 created with empty user")
	}

	uu, _ := repo.FindByUserID("u1")
	if len(uu) != 2 {
		t.Fatalf("expected 2 urls for user u1, got %d", len(uu))
	}
}

func TestFileRepository_LoadFromFile_MissingAndEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.json")

	// file missing -> ok
	repo, err := NewFileURLRepository(path)
	if err != nil {
		t.Fatalf("NewFileURLRepository(missing): %v", err)
	}

	// empty file -> ok
	if err := os.WriteFile(path, []byte{}, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	repo2, err := NewFileURLRepository(path)
	if err != nil {
		t.Fatalf("NewFileURLRepository(empty): %v", err)
	}

	_ = repo.Close()
	_ = repo2.Close()
}

func TestFileRepository_LoadFromFile_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.json")

	if err := os.WriteFile(path, []byte("{not-json"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := NewFileURLRepository(path); err == nil {
		t.Fatalf("expected error on invalid JSON")
	}
}

func TestFileRepository_PersistAndQueries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.json")

	repo, err := NewFileURLRepository(path)
	if err != nil {
		t.Fatalf("NewFileURLRepository: %v", err)
	}

	u1 := &model.URL{ID: "1", Original: "https://a.com", Short: "http://s/1", UserID: "u1"}
	u2 := &model.URL{ID: "2", Original: "https://b.com", Short: "http://s/2", UserID: "u1"}
	if err := repo.Create(u1); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := repo.Create(u2); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// duplicate original
	if err := repo.Create(&model.URL{ID: "x", Original: u1.Original, Short: "http://s/x", UserID: "u1"}); err == nil {
		t.Fatalf("expected error on duplicate original")
	}

	// FindByID / FindByOriginalURL / FindByUserID
	got, _ := repo.FindByID("1")
	if got == nil || got.Original != "https://a.com" {
		t.Fatalf("FindByID got=%v", got)
	}

	got, _ = repo.FindByOriginalURL("https://b.com")
	if got == nil || got.ID != "2" {
		t.Fatalf("FindByOriginalURL got=%v", got)
	}

	list, _ := repo.FindByUserID("u1")
	if len(list) != 2 {
		t.Fatalf("expected 2 urls, got %d", len(list))
	}

	// MarkAsDeleted should persist
	if err := repo.MarkAsDeleted(context.Background(), "u1", []string{"1"}); err != nil {
		t.Fatalf("MarkAsDeleted: %v", err)
	}

	_ = repo.Close()

	// re-open and ensure loaded state includes deleted flag
	repo2, err := NewFileURLRepository(path)
	if err != nil {
		t.Fatalf("NewFileURLRepository reopen: %v", err)
	}
	defer repo2.Close()

	u, _ := repo2.FindByID("1")
	if u == nil || !u.IsDeleted {
		t.Fatalf("expected deleted flag persisted, got=%v", u)
	}
}

func TestFileRepository_CreateBatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.json")

	repo, err := NewFileURLRepository(path)
	if err != nil {
		t.Fatalf("NewFileURLRepository: %v", err)
	}
	defer repo.Close()

	// seed
	if err := repo.Create(&model.URL{ID: "1", Original: "https://a.com", Short: "http://s/1", UserID: "u1"}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	batch := []*model.URL{
		{ID: "dup", Original: "https://a.com", Short: "http://s/dup", UserID: "u1"}, // duplicate original -> skip
		{ID: "2", Original: "https://b.com", Short: "http://s/2", UserID: "u1"},
	}
	if err := repo.CreateBatch(batch); err != nil {
		t.Fatalf("CreateBatch: %v", err)
	}

	u, _ := repo.FindByOriginalURL("https://a.com")
	if u == nil || u.ID != "1" {
		t.Fatalf("expected https://a.com stay ID=1, got=%v", u)
	}

	u, _ = repo.FindByID("2")
	if u == nil || u.Original != "https://b.com" {
		t.Fatalf("expected id=2 exists")
	}

	// file should be valid JSON array
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var arr []model.URL
	if err := json.Unmarshal(raw, &arr); err != nil {
		t.Fatalf("expected saved JSON array, got err=%v, data=%s", err, string(raw))
	}
}

func TestFileRepository_Create_RollbackOnSaveError(t *testing.T) {
	goodPath, brokenPath := makeBrokenFilePath(t)

	repo, err := NewFileURLRepository(goodPath)
	if err != nil {
		t.Fatalf("NewFileURLRepository: %v", err)
	}

	// ломаем сохранение
	repo.filePath = brokenPath

	u := &model.URL{ID: "1", Original: "https://a.com", Short: "http://s/1", UserID: "u1"}
	if err := repo.Create(u); err == nil {
		t.Fatalf("expected save error")
	}

	// rollback: ничего не должно остаться в памяти
	got, _ := repo.FindByID("1")
	if got != nil {
		t.Fatalf("expected rollback removed ID=1, got=%v", got)
	}
	got, _ = repo.FindByOriginalURL("https://a.com")
	if got != nil {
		t.Fatalf("expected rollback removed original mapping, got=%v", got)
	}
}
