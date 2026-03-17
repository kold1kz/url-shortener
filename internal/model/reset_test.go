package model

import "testing"

func TestBatchRequest_Reset(t *testing.T) {
	obj := &BatchRequest{
		CorrelationID: "123",
		OriginalURL:   "https://example.com",
	}

	obj.Reset()

	if obj.CorrelationID != "" || obj.OriginalURL != "" {
		t.Fatal("fields not reset")
	}
}

func TestBatchResponse_Reset(t *testing.T) {
	obj := &BatchResponse{
		CorrelationID: "123",
		ShortURL:      "short",
	}

	obj.Reset()

	if obj.CorrelationID != "" || obj.ShortURL != "" {
		t.Fatal("fields not reset")
	}
}

func TestShortenRequest_Reset(t *testing.T) {
	obj := &ShortenRequest{
		URL: "https://example.com",
	}

	obj.Reset()

	if obj.URL != "" {
		t.Fatal("URL not reset")
	}
}

func TestShortenResponse_Reset(t *testing.T) {
	obj := &ShortenResponse{
		Result: "short",
	}

	obj.Reset()

	if obj.Result != "" {
		t.Fatal("Result not reset")
	}
}

func TestURL_Reset(t *testing.T) {
	obj := &URL{
		ID:        "id",
		Original:  "orig",
		Short:     "short",
		UserID:    "user",
		IsDeleted: true,
	}

	obj.Reset()

	if obj.ID != "" ||
		obj.Original != "" ||
		obj.Short != "" ||
		obj.UserID != "" ||
		obj.IsDeleted != false {
		t.Fatal("fields not reset correctly")
	}
}

func TestUserURLResponse_Reset(t *testing.T) {
	obj := &UserURLResponse{
		ShortURL:    "short",
		OriginalURL: "orig",
	}

	obj.Reset()

	if obj.ShortURL != "" || obj.OriginalURL != "" {
		t.Fatal("fields not reset")
	}
}
