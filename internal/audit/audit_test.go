package audit

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
	"sync/atomic"
	"testing"
	"time"
)

func TestPublisher_AddSink_NilIgnored(t *testing.T) {
	p := NewPublisher()
	p.AddSink(nil)

	// publish should not panic
	p.Publish(context.Background(), Event{TS: 1, Action: ActionShorten, URL: "x"})
	_ = p.Close()
}

type mockSink struct {
	consumeCalls int64
	closeCalls   int64

	wg     *sync.WaitGroup
	errC   error
	errCls error
}

func (m *mockSink) Consume(ctx context.Context, e Event) error {
	atomic.AddInt64(&m.consumeCalls, 1)
	if m.wg != nil {
		m.wg.Done()
	}
	return m.errC
}

func (m *mockSink) Close() error {
	atomic.AddInt64(&m.closeCalls, 1)
	return m.errCls
}

func TestPublisher_Publish_FanoutToAllSinks(t *testing.T) {
	p := NewPublisher()

	var wg sync.WaitGroup
	wg.Add(2)

	s1 := &mockSink{wg: &wg}
	s2 := &mockSink{wg: &wg}

	p.AddSink(s1)
	p.AddSink(s2)

	p.Publish(context.Background(), Event{TS: 123, Action: ActionShorten, UserID: "u1", URL: "https://x"})

	// Publish spawns goroutines -> wait bounded time
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

	if atomic.LoadInt64(&s1.consumeCalls) != 1 || atomic.LoadInt64(&s2.consumeCalls) != 1 {
		t.Fatalf("expected both sinks consumed exactly once")
	}

	_ = p.Close()
}

func TestPublisher_Close_ReturnsFirstError(t *testing.T) {
	p := NewPublisher()

	s1 := &mockSink{errCls: errors.New("close1")}
	s2 := &mockSink{errCls: errors.New("close2")}

	p.AddSink(s1)
	p.AddSink(s2)

	err := p.Close()
	if err == nil || err.Error() != "close1" {
		t.Fatalf("expected first close error, got: %v", err)
	}
}

func TestFileSink_NewFileSink_BadPath(t *testing.T) {
	// invalid path: create dir as file path prefix or impossible location
	_, err := NewFileSink(string([]byte{0})) // NUL is invalid path on most OS
	if err == nil {
		t.Fatalf("expected error for invalid path")
	}
}

func TestFileSink_Consume_WritesJSONLine_AndClose(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")

	s, err := NewFileSink(path)
	if err != nil {
		t.Fatalf("NewFileSink error: %v", err)
	}

	ev := Event{TS: 111, Action: ActionFollow, UserID: "u2", URL: "https://example.com"}
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

	// must end with '\n'
	if !strings.HasSuffix(got, "\n") {
		t.Fatalf("expected newline at end, got: %q", got)
	}

	// check that key names match json tags and contains values
	if !strings.Contains(got, `"ts":111`) ||
		!strings.Contains(got, `"action":"follow"`) ||
		!strings.Contains(got, `"user_id":"u2"`) ||
		!strings.Contains(got, `"url":"https://example.com"`) {
		t.Fatalf("unexpected json line: %s", got)
	}
}

func TestFileSink_Consume_MarshalError(t *testing.T) {
	// stub jsonMarshal to return error
	old := jsonMarshal
	jsonMarshal = func(v any) ([]byte, error) { return nil, errors.New("marshal") }
	t.Cleanup(func() { jsonMarshal = old })

	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")

	s, err := NewFileSink(path)
	if err != nil {
		t.Fatalf("NewFileSink error: %v", err)
	}
	defer s.Close()

	err = s.Consume(context.Background(), Event{TS: 1, Action: ActionShorten, URL: "x"})
	if err == nil || err.Error() != "marshal" {
		t.Fatalf("expected marshal error, got: %v", err)
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

	s := NewHTTPSink(ts.URL)
	err := s.Consume(context.Background(), Event{TS: 10, Action: ActionShorten, UserID: "u1", URL: "https://x"})
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

	s := NewHTTPSink(ts.URL)
	err := s.Consume(context.Background(), Event{TS: 1, Action: ActionShorten, URL: "x"})
	if err == nil {
		t.Fatalf("expected error")
	}
	if _, ok := err.(*httpError); !ok {
		t.Fatalf("expected *httpError, got %T", err)
	}
	if err.Error() != "audit http sink status not ok" {
		t.Fatalf("unexpected error string: %v", err)
	}
}

func TestHTTPSink_Consume_NewRequestError(t *testing.T) {
	// invalid URL -> NewRequestWithContext should fail
	s := NewHTTPSink("http://[::1") // malformed
	err := s.Consume(context.Background(), Event{TS: 1, Action: ActionShorten, URL: "x"})
	if err == nil {
		t.Fatalf("expected error for invalid request url")
	}
}

func TestHTTPSink_Consume_DoError(t *testing.T) {
	// use an URL that will fail at client.Do (invalid scheme)
	s := NewHTTPSink("http://127.0.0.1:0") // port 0 should fail connect
	err := s.Consume(context.Background(), Event{TS: 1, Action: ActionShorten, URL: "x"})
	if err == nil {
		t.Fatalf("expected do error")
	}
}

func TestHTTPSink_Consume_MarshalError(t *testing.T) {
	old := jsonMarshal
	jsonMarshal = func(v any) ([]byte, error) { return nil, errors.New("marshal") }
	t.Cleanup(func() { jsonMarshal = old })

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	s := NewHTTPSink(ts.URL)
	err := s.Consume(context.Background(), Event{TS: 1, Action: ActionShorten, URL: "x"})
	if err == nil || err.Error() != "marshal" {
		t.Fatalf("expected marshal error, got %v", err)
	}
}
