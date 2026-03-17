package grpcserver

import (
	"context"
	"errors"
	"testing"

	"url-shortener/internal/model"
	pb "url-shortener/proto"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type stubURLService struct {
	shortenFn    func(ctx context.Context, original string, userID string) (*model.URL, error)
	expandFn     func(ctx context.Context, id string) (string, error)
	findByUserFn func(ctx context.Context, userID string) ([]*model.URL, error)
}

func (s *stubURLService) ShortenURL(ctx context.Context, original string, userID string) (*model.URL, error) {
	return s.shortenFn(ctx, original, userID)
}
func (s *stubURLService) GetOriginalURL(ctx context.Context, id string) (string, error) {
	return s.expandFn(ctx, id)
}
func (s *stubURLService) ShortenURLBatch(ctx context.Context, _ []model.BatchRequest, _ string) ([]model.BatchResponse, error) {
	return nil, nil
}
func (s *stubURLService) FindByUserID(ctx context.Context, userID string) ([]*model.URL, error) {
	return s.findByUserFn(ctx, userID)
}
func (s *stubURLService) DeleteUserURLs(context.Context, string, []string) error { return nil }
func (s *stubURLService) GetStats(context.Context) (*model.StatsResponse, error) {
	return nil, nil
}
func (s *stubURLService) Close() error { return nil }

type stubLoginService struct {
	loginFn func(ctx context.Context, username, password string) (string, error)
}

func (s *stubLoginService) Login(ctx context.Context, username, password string) (string, error) {
	return s.loginFn(ctx, username, password)
}

func TestServer_ShortenURL_Success(t *testing.T) {
	srv := NewServer(&stubURLService{
		shortenFn: func(ctx context.Context, original string, userID string) (*model.URL, error) {
			return &model.URL{Short: "short"}, nil
		},
	}, &stubLoginService{})

	ctx := withUserID(context.Background(), "u1")

	resp, err := srv.ShortenURL(ctx, pb.URLShortenRequest_builder{
		Url: protoString("url"),
	}.Build())
	if err != nil {
		t.Fatal(err)
	}

	if resp.GetResult() != "short" {
		t.Fatalf("unexpected result: %v", resp.GetResult())
	}
}

func TestServer_ShortenURL_NoUser(t *testing.T) {
	srv := NewServer(&stubURLService{}, &stubLoginService{})

	_, err := srv.ShortenURL(context.Background(), pb.URLShortenRequest_builder{
		Url: protoString("url"),
	}.Build())
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", err)
	}
}

func TestServer_ExpandURL_NotFound(t *testing.T) {
	srv := NewServer(&stubURLService{
		expandFn: func(ctx context.Context, id string) (string, error) {
			return "", nil
		},
	}, &stubLoginService{})

	_, err := srv.ExpandURL(context.Background(), pb.URLExpandRequest_builder{
		Id: protoString("1"),
	}.Build())
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v", err)
	}
}

func TestServer_ListUserURLs_Success(t *testing.T) {
	srv := NewServer(&stubURLService{
		findByUserFn: func(ctx context.Context, userID string) ([]*model.URL, error) {
			return []*model.URL{{Short: "s", Original: "o"}}, nil
		},
	}, &stubLoginService{})

	ctx := withUserID(context.Background(), "u1")

	resp, err := srv.ListUserURLs(ctx, &emptypb.Empty{})
	if err != nil {
		t.Fatal(err)
	}

	if len(resp.GetUrl()) != 1 {
		t.Fatal("expected 1 url")
	}
}

func TestServer_Login_Success(t *testing.T) {
	srv := NewServer(&stubURLService{}, &stubLoginService{
		loginFn: func(ctx context.Context, username, password string) (string, error) {
			return "token", nil
		},
	})

	resp, err := srv.Login(context.Background(), pb.LoginRequest_builder{
		Username: protoString("u"),
		Password: protoString("p"),
	}.Build())
	if err != nil {
		t.Fatal(err)
	}

	if resp.GetToken() != "token" {
		t.Fatal("unexpected token")
	}
}

func TestServer_Login_Invalid(t *testing.T) {
	srv := NewServer(&stubURLService{}, &stubLoginService{
		loginFn: func(ctx context.Context, username, password string) (string, error) {
			return "", errors.New("bad")
		},
	})

	_, err := srv.Login(context.Background(), pb.LoginRequest_builder{
		Username: protoString("u"),
		Password: protoString("p"),
	}.Build())

	if status.Code(err) != codes.Unauthenticated {
		t.Fatal("expected Unauthenticated")
	}
}

func protoString(v string) *string {
	return &v
}
