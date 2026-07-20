package services_test

import (
	"errors"
	"testing"

	"order-saga-demo/internal/services"
)

func TestInventoryService_ReserveSucceedsByDefault(t *testing.T) {
	s := services.NewInventoryService(nil, nil)
	if err := s.Reserve("order-1"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestInventoryService_ReserveFailsWhenConfigured(t *testing.T) {
	s := services.NewInventoryService(func(orderID string) bool { return orderID == "order-2" }, nil)
	if err := s.Reserve("order-1"); err != nil {
		t.Fatalf("expected order-1 to succeed, got %v", err)
	}
	err := s.Reserve("order-2")
	if !errors.Is(err, services.ErrInventoryUnavailable) {
		t.Fatalf("expected ErrInventoryUnavailable, got %v", err)
	}
}

func TestInventoryService_ReleaseInvokesHook(t *testing.T) {
	var released string
	s := services.NewInventoryService(nil, func(orderID string) { released = orderID })
	s.Release("order-3")
	if released != "order-3" {
		t.Fatalf("expected hook to receive order-3, got %q", released)
	}
}

func TestPaymentService_ChargeFailsWhenConfigured(t *testing.T) {
	s := services.NewPaymentService(func(orderID string) bool { return orderID == "order-4" })
	if err := s.Charge("order-1", 10); err != nil {
		t.Fatalf("expected order-1 to succeed, got %v", err)
	}
	err := s.Charge("order-4", 10)
	if !errors.Is(err, services.ErrPaymentDeclined) {
		t.Fatalf("expected ErrPaymentDeclined, got %v", err)
	}
}
