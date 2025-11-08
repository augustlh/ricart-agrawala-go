package queue

import "sync"

type Cons[T any] struct {
	value T
	next *Cons[T]
}


type AtomicQueue[T any] struct {
	mu sync.Mutex
	head *Cons[T]
	tail *Cons[T]
	size int
}

func NewAtomicQueue[T any]() AtomicQueue[T] {
	return AtomicQueue[T]{mu: sync.Mutex{}, head: nil, size: 0}
}

func (queue *AtomicQueue[T]) Pop() T {
	if queue.size == 0 {
		panic("Queue is empty")
	}
	value := queue.head.value

	queue.head = queue.head.next
	queue.size -= 1;

	return value
}

func (queue *AtomicQueue[T]) Push(value T) {
	queue.mu.Lock()

	element := new(Cons[T])
	element.value = value

	if queue.size == 0 {
		queue.head = element
		queue.tail = element
	} else {
		queue.tail.next = element
		queue.tail = element
	}
	queue.size += 1

	queue.mu.Unlock()
}

func (queue *AtomicQueue[T]) Size() int {
	return queue.size
}

func (queue *AtomicQueue[T]) Peek() T {
	queue.mu.Lock()

	if queue.size == 0 {
		panic("Queue is empty")
	}

	value := queue.head.value
	queue.mu.Unlock()

	return value
}


