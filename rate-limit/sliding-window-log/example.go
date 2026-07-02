package main

import (
	"fmt"
	"sync"
	"time"
)

type SlidingWindowLogLimiter struct {
	mu         sync.Mutex
	limit      int
	windowSize time.Duration
	requests   map[string][]time.Time
}

func NewSlidingWindowLogLimiter(limit int, windowSize time.Duration) *SlidingWindowLogLimiter {
	return &SlidingWindowLogLimiter{
		limit:      limit,
		windowSize: windowSize,
		requests:   make(map[string][]time.Time),
	}
}

func (l *SlidingWindowLogLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-l.windowSize)
	logs := l.requests[key]

	keepFrom := 0
	for keepFrom < len(logs) && logs[keepFrom].Before(cutoff) {
		keepFrom++
	}
	logs = logs[keepFrom:]

	if len(logs) >= l.limit {
		l.requests[key] = logs
		return false
	}

	logs = append(logs, now)
	l.requests[key] = logs
	return true
}

func main() {
	limiter := NewSlidingWindowLogLimiter(3, 10*time.Second)
	key := "login:account:user_123"

	for i := 1; i <= 5; i++ {
		fmt.Printf("login attempt %d allowed=%v\n", i, limiter.Allow(key))
	}
}
