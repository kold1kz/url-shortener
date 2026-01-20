package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	retryablehttp "github.com/hashicorp/go-retryablehttp"
)

// HTTPSink отправляет audit-события HTTP POST запросом на указанный endpoint.
type HTTPSink struct {
	url     string
	client  *retryablehttp.Client
	marshal func(v any) ([]byte, error)
}

// HTTPSinkOption настраивает HTTPSink.
type HTTPSinkOption func(*HTTPSink)

// WithHTTPClient позволяет заменить http.Client (например, для тестов/таймаутов).
func WithHTTPClient(c *http.Client) HTTPSinkOption {
	return func(s *HTTPSink) {
		if c != nil {
			s.client.HTTPClient = c
		}
	}
}

// WithMarshal позволяет подменить JSON-маршалер (нужно только для тестирования ошибок marshal).
func WithMarshal(m func(v any) ([]byte, error)) HTTPSinkOption {
	return func(s *HTTPSink) {
		if m != nil {
			s.marshal = m
		}
	}
}

// NewHTTPSink создаёт HTTP sink для аудита.
func NewHTTPSink(url string, opts ...HTTPSinkOption) *HTTPSink {
	client := retryablehttp.NewClient()
	client.Logger = nil
	client.RetryMax = 3
	client.RetryWaitMin = 100 * time.Millisecond
	client.RetryWaitMax = 1 * time.Second
	client.HTTPClient.Timeout = 3 * time.Second

	s := &HTTPSink{
		url:     url,
		client:  client,
		marshal: json.Marshal,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Consume отправляет audit-событие на удалённый endpoint.
func (s *HTTPSink) Consume(ctx context.Context, e Event) error {
	body, err := s.marshal(e)
	if err != nil {
		return fmt.Errorf("audit http sink marshal event: %w", err)
	}

	req, err := retryablehttp.NewRequestWithContext(ctx, http.MethodPost, s.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("audit http sink build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("audit http sink request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &httpError{StatusCode: resp.StatusCode}
	}

	return nil
}

// Close реализует Sink. Для HTTPSink закрывать нечего.
func (s *HTTPSink) Close() error { return nil }
