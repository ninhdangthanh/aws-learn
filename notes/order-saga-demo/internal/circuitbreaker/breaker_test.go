// internal/circuitbreaker/breaker_test.go
package circuitbreaker_test

import (
	"errors"
	"testing"
	"time"

	"order-saga-demo/internal/circuitbreaker"
)

var errBoom = errors.New("boom")

func TestBreaker_ClosedStaysClosedBelowThreshold(t *testing.T) {
	b := circuitbreaker.New(3, 50*time.Millisecond)
	for i := 0; i < 2; i++ {
		err := b.Execute(func() error { return errBoom })
		if !errors.Is(err, errBoom) {
			t.Fatalf("expected errBoom, got %v", err)
		}
	}
	if b.State() != circuitbreaker.Closed {
		t.Fatalf("expected Closed, got %v", b.State())
	}
}

func TestBreaker_OpensAfterThresholdAndFailsFast(t *testing.T) {
	b := circuitbreaker.New(3, 50*time.Millisecond)
	for i := 0; i < 3; i++ {
		_ = b.Execute(func() error { return errBoom })
	}
	if b.State() != circuitbreaker.Open {
		t.Fatalf("expected Open, got %v", b.State())
	}

	called := false
	err := b.Execute(func() error { called = true; return nil })
	if !errors.Is(err, circuitbreaker.ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}
	if called {
		t.Fatalf("fn must not be called while circuit is open")
	}
}

func TestBreaker_HalfOpenTrialSuccessCloses(t *testing.T) {
	b := circuitbreaker.New(2, 20*time.Millisecond)
	for i := 0; i < 2; i++ {
		_ = b.Execute(func() error { return errBoom })
	}
	time.Sleep(30 * time.Millisecond)
	if b.State() != circuitbreaker.HalfOpen {
		t.Fatalf("expected HalfOpen after timeout, got %v", b.State())
	}

	if err := b.Execute(func() error { return nil }); err != nil {
		t.Fatalf("expected trial success, got %v", err)
	}
	if b.State() != circuitbreaker.Closed {
		t.Fatalf("expected Closed after successful trial, got %v", b.State())
	}
}

func TestBreaker_HalfOpenTrialFailureReopens(t *testing.T) {
	b := circuitbreaker.New(2, 20*time.Millisecond)
	for i := 0; i < 2; i++ {
		_ = b.Execute(func() error { return errBoom })
	}
	time.Sleep(30 * time.Millisecond)
	if b.State() != circuitbreaker.HalfOpen {
		t.Fatalf("expected HalfOpen after timeout, got %v", b.State())
	}

	err := b.Execute(func() error { return errBoom })
	if !errors.Is(err, errBoom) {
		t.Fatalf("expected errBoom from trial, got %v", err)
	}
	if b.State() != circuitbreaker.Open {
		t.Fatalf("expected Open after failed trial, got %v", b.State())
	}
}
