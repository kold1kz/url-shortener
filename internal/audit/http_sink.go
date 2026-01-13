package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"
)

var jsonMarshal = json.Marshal

type HTTPSink struct {
	url    string
	client *http.Client
}

func NewHTTPSink(url string) *HTTPSink {
	return &HTTPSink{
		url: url,
		client: &http.Client{
			Timeout: 3 * time.Second,
		},
	}
}

func (s *HTTPSink) Consume(ctx context.Context, e Event) error {
	b, err := jsonMarshal(e)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &httpError{StatusCode: resp.StatusCode}
	}
	return nil
}

func (s *HTTPSink) Close() error { return nil }

type httpError struct{ StatusCode int }

func (e *httpError) Error() string { return "audit http sink status not ok" }
