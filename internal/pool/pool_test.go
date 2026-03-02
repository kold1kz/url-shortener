package pool

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type testObj struct {
	n          int
	resetCalls int
}

func (t *testObj) Reset() {
	t.resetCalls++
	t.n = 0
}

func TestPool_ResetCalledOnPut(t *testing.T) {
	p := New(func() *testObj { return &testObj{} })

	x := p.Get()
	x.n = 42
	require.Equal(t, 0, x.resetCalls)

	p.Put(x)

	// Reset точно вызвали при Put
	require.Equal(t, 1, x.resetCalls)
	require.Equal(t, 0, x.n)

	// Достаём снова — объект должен быть "чистый"
	y := p.Get()
	require.Equal(t, 0, y.n)
	require.GreaterOrEqual(t, y.resetCalls, 1)
}

func TestPool_NewPanicsOnNilFactory(t *testing.T) {
	require.Panics(t, func() {
		_ = New[*testObj](nil)
	})
}

func TestPool_NewPanicsIfFactoryReturnsNil(t *testing.T) {
	p := New(func() *testObj { return nil })
	require.Panics(t, func() {
		_ = p.Get()
	})
}

func TestPool_NilReceiverIsSafe(t *testing.T) {
	var p *Pool[*testObj]

	// nil receiver Get() должен вернуть нулевое значение T, для *testObj это nil
	require.Nil(t, p.Get())

	require.NotPanics(t, func() { p.Put(nil) })
	require.NotPanics(t, func() { p.Put(&testObj{n: 1}) })
}
