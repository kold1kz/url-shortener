package grpcserver

import (
	"context"

	"url-shortener/internal/auth"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func AuthInterceptor(
	ctx context.Context,
	req any,
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (any, error) {
	// ExpandURL можно оставить публичным, как и HTTP GET /<id>
	if info.FullMethod == "/shortener.ShortenerService/ExpandURL" ||
		info.FullMethod == "/shortener.ShortenerService/Login" {
		return handler(ctx, req)
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing authorization metadata")
	}

	values := md.Get("authorization")
	if len(values) == 0 || values[0] == "" {
		return nil, status.Error(codes.Unauthenticated, "missing authorization token")
	}

	userID, err := auth.ParseToken(values[0])
	if err != nil || userID == "" {
		return nil, status.Error(codes.Unauthenticated, "invalid authorization token")
	}

	return handler(withUserID(ctx, userID), req)
}
