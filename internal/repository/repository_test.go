package repository

import (
	"testing"

	"url-shortener/internal/model"

	"github.com/stretchr/testify/require"
)

func TestInMemory_CreateBatchUpdatesUserURLs(t *testing.T) {
	repo := NewInMemoryURLRepository()

	urls := []*model.URL{
		{ID: "1", Original: "https://ex.com/1", Short: "s1", UserID: "user1"},
		{ID: "2", Original: "https://ex.com/2", Short: "s2", UserID: "user1"},
	}

	err := repo.CreateBatch(urls)
	require.NoError(t, err)

	got, err := repo.FindByUserID("user1")
	require.NoError(t, err)
	require.Len(t, got, 2)
}
