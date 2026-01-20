package audit

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// FileSink пишет аудит событие в файл.
type FileSink struct {
	mu sync.Mutex
	f  *os.File
	w  *bufio.Writer
}

// NewFileSink oткрывет\создает файл по пути и доваляет аудит событие.
func NewFileSink(path string) (*FileSink, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("audit file sink: open file %q: %w", path, err)
	}
	return &FileSink{f: f, w: bufio.NewWriterSize(f, 64*1024)}, nil
}

func (s *FileSink) Consume(ctx context.Context, e Event) error {
	b, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("audit file sink: marshal event: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.w.Write(append(b, '\n')); err != nil {
		return fmt.Errorf("audit file sink: write: %w", err)
	}
	if err := s.w.Flush(); err != nil {
		return fmt.Errorf("audit file sink: flush: %w", err)
	}
	return nil
}

func (s *FileSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.w != nil {
		_ = s.w.Flush()
	}
	if s.f != nil {
		return s.f.Close()
	}
	return nil
}
