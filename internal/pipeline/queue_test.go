package pipeline

import (
	"slices"
	"testing"
	"time"
)

func TestQueueHandsItemsOutInTheOrderTheyCame(t *testing.T) {
	q := newQueue[int](0)
	for i := range 5 {
		q.push(i)
	}
	q.close()

	var got []int
	for {
		item, skipped, ok := q.pop()
		if len(skipped) > 0 {
			t.Errorf("a queue without a limit skipped %v", skipped)
		}

		if !ok {
			break
		}

		got = append(got, item)
	}

	if !slices.Equal(got, []int{0, 1, 2, 3, 4}) {
		t.Errorf("got %v, want 0 to 4 in order and then the end", got)
	}
}

type popping struct {
	item    int
	skipped []int
}

func popped(t *testing.T, q *queue[int], want popping) {
	t.Helper()

	item, skipped, ok := q.pop()
	if !ok || item != want.item || !slices.Equal(skipped, want.skipped) {
		t.Fatalf("pop gave %d skipping %v (ok %v), want %d skipping %v", item, skipped, ok, want.item, want.skipped)
	}
}

func TestQueueSkipsTheOldestWhenFullAndSaysSoWithTheNextItem(t *testing.T) {
	q := newQueue[int](2)
	q.push(0, 1)
	q.push(2)
	popped(t, q, popping{item: 1, skipped: []int{0}})

	q.push(3, 4, 5)
	popped(t, q, popping{item: 4, skipped: []int{2, 3}})
	popped(t, q, popping{item: 5})
}

func TestQueueWakesAPopWaitingForAnItem(t *testing.T) {
	q := newQueue[int](1)
	got := make(chan int)
	go func() {
		item, _, _ := q.pop()
		got <- item
	}()

	time.Sleep(10 * time.Millisecond)
	q.push(7)

	select {
	case item := <-got:
		if item != 7 {
			t.Errorf("the waiting pop got %d, want 7", item)
		}
	case <-time.After(time.Second):
		t.Fatal("a pop waiting on an empty queue never saw the item pushed")
	}
}

func TestQueueReleasesAWaitingPopOnceClosed(t *testing.T) {
	q := newQueue[int](1)
	got := make(chan bool)
	go func() {
		_, _, ok := q.pop()
		got <- ok
	}()

	time.Sleep(10 * time.Millisecond)
	q.close()

	select {
	case ok := <-got:
		if ok {
			t.Error("a pop on a closed empty queue claims an item")
		}
	case <-time.After(time.Second):
		t.Fatal("a pop waiting on an empty queue was never released by closing it")
	}
}
