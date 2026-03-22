package grpcserver

import (
	"context"
	"testing"
)

func TestContext_UserID(t *testing.T) {
	ctx := context.Background()

	ctx = withUserID(ctx, "123")

	userID, ok := userIDFromContext(ctx)
	if !ok {
		t.Fatal("expected userID")
	}
	if userID != "123" {
		t.Fatalf("expected 123, got %s", userID)
	}
}
