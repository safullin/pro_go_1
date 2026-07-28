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
func New[T Resetter](newValue func() T) *Pool[T] {
	return &Pool[T]{
		pool: sync.Pool{
			New: func() any {
				return newValue()
			},
		},
	}
}

// Get returns an object from the pool.
func (p *Pool[T]) Get() T {
	return p.pool.Get().(T)
}

// Put resets an object and returns it to the pool.
func (p *Pool[T]) Put(value T) {
	value.Reset()
	p.pool.Put(value)
}
