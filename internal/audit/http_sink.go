package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// HTTPSink отправляет audit-события HTTP POST запросом на указанный endpoint.
type HTTPSink struct {
	url     string
	client  *http.Client
	marshal func(v any) ([]byte, error)
}

// HTTPSinkOption настраивает HTTPSink.
type HTTPSinkOption func(*HTTPSink)

// WithHTTPClient позволяет заменить http.Client (например, для тестов/таймаутов).
func WithHTTPClient(c *http.Client) HTTPSinkOption {
	return func(s *HTTPSink) {
		if c != nil {
			s.client = c
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
	s := &HTTPSink{
		url: url,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
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
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("audit http sink request failed: %w", err)
	}
	_ = resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &httpError{StatusCode: resp.StatusCode}
	}

	return nil
}

// Close реализует Sink. Для HTTPSink закрывать нечего.
func (s *HTTPSink) Close() error {
	return nil
}
