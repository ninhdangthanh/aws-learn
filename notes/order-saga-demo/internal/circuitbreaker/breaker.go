package circuitbreaker

import (
	"errors"
	"sync"
	"time"
)

var ErrCircuitOpen = errors.New("circuit breaker is open")

type State int

const (
	Closed State = iota
	Open
	HalfOpen
)

func (s State) String() string {
	switch s {
	case Closed:
		return "Closed"
	case Open:
		return "Open"
	case HalfOpen:
		return "HalfOpen"
	default:
		return "Unknown"
	}
}

type Breaker struct {
	mu               sync.Mutex
	state            State
	failureThreshold int
	resetTimeout     time.Duration
	consecutiveFails int
	openedAt         time.Time
}

func New(failureThreshold int, resetTimeout time.Duration) *Breaker {
	return &Breaker{
		state:            Closed,
		failureThreshold: failureThreshold,
		resetTimeout:     resetTimeout,
	}
}

func (b *Breaker) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.currentState()
}

// currentState must be called with b.mu held. It lazily promotes Open to
// HalfOpen once resetTimeout has elapsed since the circuit opened.
func (b *Breaker) currentState() State {
	if b.state == Open && time.Since(b.openedAt) >= b.resetTimeout {
		b.state = HalfOpen
	}
	return b.state
}

func (b *Breaker) Execute(fn func() error) error {
	b.mu.Lock()
	if b.currentState() == Open {
		b.mu.Unlock()
		return ErrCircuitOpen
	}
	b.mu.Unlock()

	err := fn()

	b.mu.Lock()
	defer b.mu.Unlock()
	if err != nil {
		b.consecutiveFails++
		if b.state == HalfOpen || b.consecutiveFails >= b.failureThreshold {
			b.state = Open
			b.openedAt = time.Now()
		}
		return err
	}

	b.consecutiveFails = 0
	b.state = Closed
	return nil
}
