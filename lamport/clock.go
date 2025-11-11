package lamport

import "sync"

type Clock struct {
	mu sync.Mutex
	ts int64
}

func (c *Clock) Tick() int64 {
	c.mu.Lock()
	c.ts++
	v := c.ts
	c.mu.Unlock()
	return v
}

func (c *Clock) Observe(remote int64) int64 {
	c.mu.Lock()
	if remote > c.ts {
		c.ts = remote
	}
	c.ts++
	v := c.ts
	c.mu.Unlock()
	return v
}

func (c *Clock) Read() int64 {
	c.mu.Lock()
	v := c.ts
	c.mu.Unlock()
	return v
}
