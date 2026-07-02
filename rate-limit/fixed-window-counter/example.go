package main

import (
	"fmt"
	"sync"
	"time"
)

type fixedWindowState struct {
	windowStart time.Time
	count       int
}

type FixedWindowLimiter struct {
	mu         sync.Mutex
	limit      int
	windowSize time.Duration
	state      map[string]fixedWindowState
}

func NewFixedWindowLimiter(limit int, windowSize time.Duration) *FixedWindowLimiter {
	return &FixedWindowLimiter{
		limit:      limit,
		windowSize: windowSize,
		state:      make(map[string]fixedWindowState),
	}
}

func (l *FixedWindowLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	currentWindow := now.Truncate(l.windowSize)
	st := l.state[key]

	if st.windowStart.IsZero() || !st.windowStart.Equal(currentWindow) {
		st = fixedWindowState{windowStart: currentWindow}
	}

	if st.count >= l.limit {
		l.state[key] = st
		return false
	}

	st.count++
	l.state[key] = st
	return true
}

func main() {
	limiter := NewFixedWindowLimiter(3, time.Minute)
	key := "tenant:t1:/v1/orders"

	for i := 1; i <= 5; i++ {
		fmt.Printf("request %d allowed=%v\n", i, limiter.Allow(key))
	}
}
