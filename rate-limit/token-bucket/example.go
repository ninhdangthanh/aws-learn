package main

import (
	"fmt"
	"sync"
	"time"
)

type tokenBucketState struct {
	tokens     float64
	lastRefill time.Time
}

type TokenBucketLimiter struct {
	mu         sync.Mutex
	rate       float64
	capacity   float64
	stateByKey map[string]tokenBucketState
}

func NewTokenBucketLimiter(ratePerSecond float64, capacity int) *TokenBucketLimiter {
	return &TokenBucketLimiter{
		rate:       ratePerSecond,
		capacity:   float64(capacity),
		stateByKey: make(map[string]tokenBucketState),
	}
}

func (l *TokenBucketLimiter) Allow(key string, cost float64) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	st := l.stateByKey[key]
	if st.lastRefill.IsZero() {
		st.tokens = l.capacity
		st.lastRefill = now
	}

	elapsed := now.Sub(st.lastRefill).Seconds()
	st.tokens += elapsed * l.rate
	if st.tokens > l.capacity {
		st.tokens = l.capacity
	}
	st.lastRefill = now

	if st.tokens < cost {
		l.stateByKey[key] = st
		return false
	}

	st.tokens -= cost
	l.stateByKey[key] = st
	return true
}

func main() {
	limiter := NewTokenBucketLimiter(1, 3)
	key := "tenant:t1:openai"

	for i := 1; i <= 5; i++ {
		fmt.Printf("ai call %d allowed=%v\n", i, limiter.Allow(key, 1))
	}
}
