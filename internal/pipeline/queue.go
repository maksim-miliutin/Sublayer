package pipeline

import (
	"slices"
	"sync"
)

type queue[T any] struct {
	mu      sync.Mutex
	ready   *sync.Cond
	items   []T
	skipped []T
	limit   int // items kept waiting; zero keeps every one
	closed  bool
}

func newQueue[T any](limit int) *queue[T] {
	q := &queue[T]{limit: limit}
	q.ready = sync.NewCond(&q.mu)

	return q
}

func (q *queue[T]) push(items ...T) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.items = append(q.items, items...)
	if over := len(q.items) - q.limit; q.limit > 0 && over > 0 {
		q.skipped = append(q.skipped, q.items[:over]...)
		q.items = slices.Delete(q.items, 0, over)
	}

	q.ready.Signal()
}

func (q *queue[T]) pop() (item T, skipped []T, ok bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	for len(q.items) == 0 && !q.closed {
		q.ready.Wait()
	}

	skipped, q.skipped = q.skipped, nil
	if len(q.items) == 0 {
		return item, skipped, false
	}

	item = q.items[0]
	q.items = slices.Delete(q.items, 0, 1)

	return item, skipped, true
}

func (q *queue[T]) close() {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.closed = true
	q.ready.Broadcast()
}
