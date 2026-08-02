// Package pool provides reusable storage for resettable objects.
package pool

import "sync"

// Resetter describes an object whose state can be reset before reuse.
type Resetter interface {
	Reset()
}

// Pool stores reusable objects of one resettable type.
type Pool[T Resetter] struct {
	pool sync.Pool
}

// New creates a pool that uses newValue when no stored object is available.
// If newValue is nil, Get returns the zero value of T for an empty pool.
func New[T Resetter](newValue func() T) *Pool[T] {
	values := &Pool[T]{}
	if newValue != nil {
		values.pool.New = func() any {
			return newValue()
		}
	}
	return values
}

// Get returns an object from the pool.
func (p *Pool[T]) Get() T {
	value := p.pool.Get()
	if value == nil {
		var zero T
		return zero
	}
	return value.(T)
}

// Put resets an object and returns it to the pool.
func (p *Pool[T]) Put(value T) {
	value.Reset()
	p.pool.Put(value)
}
