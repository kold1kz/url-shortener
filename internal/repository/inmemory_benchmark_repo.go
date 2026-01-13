package repository

import (
	"testing"
	"url-shortener/internal/model"
)

func BenchmarkInMemory_Create(b *testing.B) {
	repo := NewInMemoryURLRepository()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		u := &model.URL{
			ID:       itoa(i),
			Original: "https://example.com/" + itoa(i),
			Short:    "http://localhost/" + itoa(i),
			UserID:   "u1",
		}
		_ = repo.Create(u)
	}
}

func BenchmarkInMemory_FindByID(b *testing.B) {
	repo := NewInMemoryURLRepository()
	// prefill
	for i := 0; i < 10000; i++ {
		u := &model.URL{
			ID:       itoa(i),
			Original: "https://example.com/" + itoa(i),
			Short:    "http://localhost/" + itoa(i),
			UserID:   "u1",
		}
		_ = repo.Create(u)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = repo.FindByID("5000")
	}
}

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
