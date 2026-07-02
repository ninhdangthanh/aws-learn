package main

import (
	"fmt"
	"sync"
	"time"
)

type leakyBucketState struct {
	queueSize int
	lastLeak  time.Time
}

type LeakyBucketLimiter struct {
	mu          sync.Mutex
	capacity    int
	leakPerSec  float64
	stateByKey  map[string]leakyBucketState
	leakRemaind map[string]float64
}

func NewLeakyBucketLimiter(capacity int, leakPerSecond float64) *LeakyBucketLimiter {
	return &LeakyBucketLimiter{
		capacity:    capacity,
		leakPerSec:  leakPerSecond,
		stateByKey:  make(map[string]leakyBucketState),
		leakRemaind: make(map[string]float64),
	}
}

func (l *LeakyBucketLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	st := l.stateByKey[key]
	if st.lastLeak.IsZero() {
		st.lastLeak = now
	}

	elapsed := now.Sub(st.lastLeak).Seconds()
	leakedFloat := elapsed*l.leakPerSec + l.leakRemaind[key]
	leaked := int(leakedFloat)
	l.leakRemaind[key] = leakedFloat - float64(leaked)

	if leaked > 0 {
		st.queueSize -= leaked
		if st.queueSize < 0 {
			st.queueSize = 0
		}
		st.lastLeak = now
	}

	if st.queueSize >= l.capacity {
		l.stateByKey[key] = st
		return false
	}

	st.queueSize++
	l.stateByKey[key] = st
	return true
}

func main() {
	limiter := NewLeakyBucketLimiter(3, 1)
	key := "worker:image-resize"

	for i := 1; i <= 5; i++ {
		fmt.Printf("job %d accepted=%v\n", i, limiter.Allow(key))
	}
}
