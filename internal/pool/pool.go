// Package pool provides a typed wrapper around sync.Pool for reusable objects
// that can reset their internal state before being returned to the pool.
package pool

import (
	"reflect"
	"sync"
)

// Resettable — ограничение для типов, которые умеют сбрасывать своё состояние.
type Resettable interface {
	Reset()
}

// Pool — типизированная обёртка над sync.Pool для объектов одного типа.
type Pool[T Resettable] struct {
	p *sync.Pool
}

// New создаёт пул.
func New[T Resettable](newFn func() T) *Pool[T] {
	if newFn == nil {
		panic("pool.New: newFn is nil")
	}

	sp := &sync.Pool{
		New: func() any {
			v := newFn()
			if isNil(v) {
				panic("pool.New: newFn returned nil")
			}
			return v
		},
	}

	return &Pool[T]{p: sp}
}

// Get возвращает объект из пула (или создаёт новый через newFn).
func (pp *Pool[T]) Get() T {
	var zero T
	if pp == nil || pp.p == nil {
		return zero
	}
	return pp.p.Get().(T)
}

// Put сбрасывает состояние объекта и возвращает его в пул.
func (pp *Pool[T]) Put(v T) {
	if pp == nil || pp.p == nil || isNil(v) {
		return
	}
	v.Reset()
	pp.p.Put(v)
}

// isNil корректно проверяет nil для “nil-able” типов.
func isNil[T any](v T) bool {
	iv := any(v)
	if iv == nil {
		return true
	}
	rv := reflect.ValueOf(iv)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}
