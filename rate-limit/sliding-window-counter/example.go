package main

import (
	"fmt"
	"sync"
	"time"
)

type slidingCounterState struct {
	currentWindow  time.Time
	currentCount   int
	previousWindow time.Time
	previousCount  int
}

type SlidingWindowCounterLimiter struct {
	mu         sync.Mutex
	limit      float64
	windowSize time.Duration
	state      map[string]slidingCounterState
}

func NewSlidingWindowCounterLimiter(limit int, windowSize time.Duration) *SlidingWindowCounterLimiter {
	return &SlidingWindowCounterLimiter{
		limit:      float64(limit),
		windowSize: windowSize,
		state:      make(map[string]slidingCounterState),
	}
}

func (l *SlidingWindowCounterLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	window := now.Truncate(l.windowSize)
	st := l.state[key]

	if st.currentWindow.IsZero() {
		st.currentWindow = window
	}

	if !st.currentWindow.Equal(window) {
		st.previousWindow = st.currentWindow
		st.previousCount = st.currentCount
		st.currentWindow = window
		st.currentCount = 0
	}

	elapsed := now.Sub(st.currentWindow)
	overlapRatio := 1 - float64(elapsed)/float64(l.windowSize)
	if !st.previousWindow.Equal(st.currentWindow.Add(-l.windowSize)) {
		st.previousCount = 0
	}

	estimated := float64(st.currentCount) + float64(st.previousCount)*overlapRatio
	if estimated >= l.limit {
		l.state[key] = st
		return false
	}

	st.currentCount++
	l.state[key] = st
	return true
}

func main() {
	limiter := NewSlidingWindowCounterLimiter(3, time.Minute)
	key := "api-key:free_123:/v1/products"

	for i := 1; i <= 5; i++ {
		fmt.Printf("request %d allowed=%v\n", i, limiter.Allow(key))
	}
}
