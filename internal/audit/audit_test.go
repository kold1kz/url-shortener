package audit_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"url-shortener/internal/audit"
	"url-shortener/internal/audit/mocks"

	"github.com/stretchr/testify/mock"
)

func TestPublisher_AddSink_NilIgnored(t *testing.T) {
	p := audit.NewPublisher()
	p.AddSink(nil)

	// publish should not panic
	p.Publish(context.Background(), audit.Event{TS: 1, Action: audit.ActionShorten, URL: "x"})
	_ = p.Close()
}

func TestPublisher_Publish_FanoutToAllSinks(t *testing.T) {
	p := audit.NewPublisher()

	var wg sync.WaitGroup
	wg.Add(2)

	s1 := mocks.NewSink(t)
	s2 := mocks.NewSink(t)

	s1.On("Consume", mock.Anything, mock.AnythingOfType("audit.Event")).
		Return(nil).
		Run(func(args mock.Arguments) { wg.Done() })

	s2.On("Consume", mock.Anything, mock.AnythingOfType("audit.Event")).
		Return(nil).
		Run(func(args mock.Arguments) { wg.Done() })

	// Close тоже будет вызван — надо описать ожидание
	s1.On("Close").Return(nil)
	s2.On("Close").Return(nil)

	p.AddSink(s1)
	p.AddSink(s2)

	p.Publish(context.Background(), audit.Event{
		TS:     123,
		Action: audit.ActionShorten,
		UserID: "u1",
		URL:    "https://x",
	})

	// дождаться асинхронных Consume
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting Consume calls")
	}

	_ = p.Close()
}

func TestPublisher_Close_ReturnsFirstError(t *testing.T) {
	p := audit.NewPublisher()

	s1 := mocks.NewSink(t)
	s2 := mocks.NewSink(t)

	s1.On("Close").Return(errors.New("close1"))
	s2.On("Close").Return(errors.New("close2"))

	p.AddSink(s1)
	p.AddSink(s2)

	err := p.Close()
	if err == nil || err.Error() != "close1" {
		t.Fatalf("expected first close error, got: %v", err)
	}
}

func TestFileSink_NewFileSink_BadPath(t *testing.T) {
	// NUL is invalid path on most OS
	_, err := audit.NewFileSink(string([]byte{0}))
	if err == nil {
		t.Fatalf("expected error for invalid path")
	}
}

func TestFileSink_Consume_WritesJSONLine_AndClose(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")

	s, err := audit.NewFileSink(path)
	if err != nil {
		t.Fatalf("NewFileSink error: %v", err)
	}

	ev := audit.Event{TS: 111, Action: audit.ActionFollow, UserID: "u2", URL: "https://example.com"}
	if err := s.Consume(context.Background(), ev); err != nil {
		t.Fatalf("Consume error: %v", err)
	}

	if err := s.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	got := string(b)

	if !strings.HasSuffix(got, "\n") {
		t.Fatalf("expected newline at end, got: %q", got)
	}

	if !strings.Contains(got, `"ts":111`) ||
		!strings.Contains(got, `"action":"follow"`) ||
		!strings.Contains(got, `"user_id":"u2"`) ||
		!strings.Contains(got, `"url":"https://example.com"`) {
		t.Fatalf("unexpected json line: %s", got)
	}
}

func TestHTTPSink_Consume_Success(t *testing.T) {
	var gotBody []byte

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Fatalf("expected content-type application/json, got %q", ct)
		}
		b, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()
		gotBody = b
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	s := audit.NewHTTPSink(ts.URL)
	err := s.Consume(context.Background(), audit.Event{TS: 10, Action: audit.ActionShorten, UserID: "u1", URL: "https://x"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !strings.Contains(string(gotBody), `"ts":10`) {
		t.Fatalf("unexpected body: %s", string(gotBody))
	}
	_ = s.Close()
}

func TestHTTPSink_Consume_StatusNotOK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot) // 418
	}))
	defer ts.Close()

	s := audit.NewHTTPSink(ts.URL)
	err := s.Consume(context.Background(), audit.Event{TS: 1, Action: audit.ActionShorten, URL: "x"})
	if err == nil {
		t.Fatalf("expected error")
	}
	if err.Error() != "audit http sink status not ok: status=418" {
		t.Fatalf("unexpected error string: %v", err)
	}
}

func TestHTTPSink_Consume_NewRequestError(t *testing.T) {
	s := audit.NewHTTPSink("http://[::1") // malformed
	err := s.Consume(context.Background(), audit.Event{TS: 1, Action: audit.ActionShorten, URL: "x"})
	if err == nil {
		t.Fatalf("expected error for invalid request url")
	}
}

func TestHTTPSink_Consume_DoError(t *testing.T) {
	s := audit.NewHTTPSink("http://127.0.0.1:0") // connect should fail
	err := s.Consume(context.Background(), audit.Event{TS: 1, Action: audit.ActionShorten, URL: "x"})
	if err == nil {
		t.Fatalf("expected do error")
	}
}

func TestHTTPSink_Consume_MarshalError(t *testing.T) {
	s := audit.NewHTTPSink("http://example.com", audit.WithMarshal(func(v any) ([]byte, error) {
		return nil, errors.New("marshal")
	}))

	err := s.Consume(context.Background(), audit.Event{TS: 1, Action: audit.ActionShorten, URL: "x"})
	require.Error(t, err)
	require.EqualError(t, err, "marshal")
}
