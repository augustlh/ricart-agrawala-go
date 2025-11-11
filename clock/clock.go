package clock;

import "sync"

type AtomicLamportClock struct {
	time uint32;
	mu sync.Mutex
}

func NewClock() AtomicLamportClock {
	return AtomicLamportClock {time: 0}
}

func (this *AtomicLamportClock) HappenedBefore(that *AtomicLamportClock) bool {
	return this.time < that.time
}

func (this *AtomicLamportClock) MergeWith(that *AtomicLamportClock) {
	this.mu.Lock()
	this.time = max(this.time, that.time) + 1
	this.mu.Unlock()
}

func (this *AtomicLamportClock) MergeWithRawTimestamp(that uint32) {
	this.mu.Lock()
	this.time = max(this.time, that) + 1
	this.mu.Unlock()
}

func (this *AtomicLamportClock) CurrentTime() uint32 {
	return this.time
}

func (this *AtomicLamportClock) Advance() {
	this.mu.Lock()
	this.time += 1
	this.mu.Unlock()
}

