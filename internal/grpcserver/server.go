package grpcserver

import (
	"context"
	"errors"

	"url-shortener/internal/service"
	pb "url-shortener/proto"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

type Server struct {
	pb.UnimplementedShortenerServiceServer
	urlService   service.URLService
	loginService service.LoginService
}

func NewServer(urlSvc service.URLService, loginSvc service.LoginService) *Server {
	return &Server{
		urlService:   urlSvc,
		loginService: loginSvc,
	}
}

func (s *Server) ShortenURL(ctx context.Context, req *pb.URLShortenRequest) (*pb.URLShortenResponse, error) {
	userID, ok := userIDFromContext(ctx)
	if !ok || userID == "" {
		return nil, status.Error(codes.Unauthenticated, "missing user in context")
	}

	if req.GetUrl() == "" {
		return nil, status.Error(codes.InvalidArgument, "url is empty")
	}

	u, err := s.urlService.ShortenURL(ctx, req.GetUrl(), userID)
	if err != nil {
		if errors.Is(err, service.ErrURLAlreadyExists) && u != nil {
			return pb.URLShortenResponse_builder{
				Result: proto.String(u.Short),
			}.Build(), nil
		}
		return nil, status.Errorf(codes.Internal, "shorten url: %v", err)
	}

	return pb.URLShortenResponse_builder{
		Result: proto.String(u.Short),
	}.Build(), nil
}

func (s *Server) ExpandURL(ctx context.Context, req *pb.URLExpandRequest) (*pb.URLExpandResponse, error) {
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id is empty")
	}

	originalURL, err := s.urlService.GetOriginalURL(ctx, req.GetId())
	if err != nil {
		if errors.Is(err, service.ErrURLDeleted) {
			return nil, status.Error(codes.FailedPrecondition, "url is deleted")
		}
		return nil, status.Errorf(codes.Internal, "expand url: %v", err)
	}

	if originalURL == "" {
		return nil, status.Error(codes.NotFound, "url not found")
	}

	return pb.URLExpandResponse_builder{
		Result: proto.String(originalURL),
	}.Build(), nil
}

func (s *Server) ListUserURLs(ctx context.Context, _ *emptypb.Empty) (*pb.UserURLsResponse, error) {
	userID, ok := userIDFromContext(ctx)
	if !ok || userID == "" {
		return nil, status.Error(codes.Unauthenticated, "missing user in context")
	}

	urls, err := s.urlService.FindByUserID(ctx, userID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "find user urls: %v", err)
	}

	items := make([]*pb.URLData, 0, len(urls))
	for _, item := range urls {
		items = append(items, pb.URLData_builder{
			ShortUrl:    proto.String(item.Short),
			OriginalUrl: proto.String(item.Original),
		}.Build())
	}

	return pb.UserURLsResponse_builder{
		Url: items,
	}.Build(), nil
}

func (s *Server) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	if req.GetUsername() == "" || req.GetPassword() == "" {
		return nil, status.Error(codes.InvalidArgument, "empty credentials")
	}

	token, err := s.loginService.Login(ctx, req.GetUsername(), req.GetPassword())
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid credentials")
	}

	return pb.LoginResponse_builder{
		Token: proto.String(token),
	}.Build(), nil
}
