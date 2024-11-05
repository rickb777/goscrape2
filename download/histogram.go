package download

import (
	"sync"
)

type SyncCounter struct {
	m  map[int]int
	mu sync.Mutex
}

func NewHistogram() *SyncCounter {
	return &SyncCounter{m: make(map[int]int)}
}

// Increment adds one to the bucket indicated by code.
func (c *SyncCounter) Increment(code int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[code]++
}

// Map accesses the histogram. This is not safe for concurrent use.
func (c *SyncCounter) Map() map[int]int {
	return c.m
}
