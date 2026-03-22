package grpcserver

import (
	"context"
	"testing"

	"url-shortener/internal/auth"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestAuthInterceptor_PublicMethods(t *testing.T) {
	methods := []string{
		"/shortener.ShortenerService/ExpandURL",
		"/shortener.ShortenerService/Login",
	}

	for _, m := range methods {
		_, err := AuthInterceptor(
			context.Background(),
			nil,
			&grpc.UnaryServerInfo{FullMethod: m},
			func(ctx context.Context, req any) (any, error) {
				return "ok", nil
			},
		)

		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestAuthInterceptor_NoMetadata(t *testing.T) {
	_, err := AuthInterceptor(
		context.Background(),
		nil,
		&grpc.UnaryServerInfo{FullMethod: "X"},
		func(ctx context.Context, req any) (any, error) { return nil, nil },
	)

	if status.Code(err) != codes.Unauthenticated {
		t.Fatal("expected Unauthenticated")
	}
}

func TestAuthInterceptor_InvalidToken(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(),
		metadata.Pairs("authorization", "bad"),
	)

	_, err := AuthInterceptor(
		ctx,
		nil,
		&grpc.UnaryServerInfo{FullMethod: "X"},
		func(ctx context.Context, req any) (any, error) { return nil, nil },
	)

	if status.Code(err) != codes.Unauthenticated {
		t.Fatal("expected Unauthenticated")
	}
}

func TestAuthInterceptor_Success(t *testing.T) {
	token, _ := auth.NewTokenForUserID("42")

	ctx := metadata.NewIncomingContext(context.Background(),
		metadata.Pairs("authorization", token),
	)

	called := false

	_, err := AuthInterceptor(
		ctx,
		nil,
		&grpc.UnaryServerInfo{FullMethod: "X"},
		func(ctx context.Context, req any) (any, error) {
			called = true

			userID, ok := userIDFromContext(ctx)
			if !ok || userID != "42" {
				t.Fatal("wrong userID in context")
			}

			return nil, nil
		},
	)

	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("handler not called")
	}
}
