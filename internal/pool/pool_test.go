package pool

import (
	"sync"
	"sync/atomic"
	"testing"
)

type testValue struct {
	text       string
	numbers    []int
	resetCount int
}

func (v *testValue) Reset() {
	v.text = ""
	v.numbers = v.numbers[:0]
	v.resetCount++
}

func TestPoolGet(t *testing.T) {
	var created atomic.Int64
	values := New(func() *testValue {
		created.Add(1)
		return &testValue{}
	})

	value := values.Get()

	if value == nil {
		t.Fatal("Get() returned nil")
	}
	if created.Load() != 1 {
		t.Fatalf("factory calls = %d, want 1", created.Load())
	}
}

func TestPoolGetWithoutFactory(t *testing.T) {
	values := New[*testValue](nil)

	if value := values.Get(); value != nil {
		t.Fatalf("Get() = %#v, want nil", value)
	}
}

func TestPoolPutResetsValue(t *testing.T) {
	values := New(func() *testValue {
		return &testValue{}
	})
	value := &testValue{
		text:    "value",
		numbers: []int{1, 2, 3},
	}
	numbersCapacity := cap(value.numbers)

	values.Put(value)

	if value.text != "" {
		t.Errorf("text = %q, want empty string", value.text)
	}
	if len(value.numbers) != 0 {
		t.Errorf("numbers length = %d, want 0", len(value.numbers))
	}
	if cap(value.numbers) != numbersCapacity {
		t.Errorf("numbers capacity = %d, want %d", cap(value.numbers), numbersCapacity)
	}
	if value.resetCount != 1 {
		t.Errorf("Reset() calls = %d, want 1", value.resetCount)
	}
}

func TestPoolConcurrentUse(t *testing.T) {
	values := New(func() *testValue {
		return &testValue{}
	})

	var workers sync.WaitGroup
	for range 100 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			value := values.Get()
			value.text = "value"
			value.numbers = append(value.numbers, 1, 2, 3)
			values.Put(value)
		}()
	}
	workers.Wait()
}
