package audit

import (
	"context"
	"sync"
)

type Sink interface {
	Consume(ctx context.Context, e Event) error
	Close() error
}

type Publisher struct {
	mu    sync.RWMutex
	sinks []Sink
}

func NewPublisher() *Publisher { return &Publisher{} }

func (p *Publisher) AddSink(s Sink) {
	if s == nil {
		return
	}
	p.mu.Lock()
	p.sinks = append(p.sinks, s)
	p.mu.Unlock()
}

func (p *Publisher) Publish(ctx context.Context, e Event) {
	p.mu.RLock()
	sinks := append([]Sink(nil), p.sinks...)
	p.mu.RUnlock()

	for _, s := range sinks {
		go func(s Sink) { _ = s.Consume(context.Background(), e) }(s)
	}
}

func (p *Publisher) Close() error {
	p.mu.RLock()
	sinks := append([]Sink(nil), p.sinks...)
	p.mu.RUnlock()

	var firstErr error
	for _, s := range sinks {
		if err := s.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
