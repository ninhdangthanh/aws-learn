package main

import "errors"

var (
	ErrOutOfStock        = errors.New("out of stock")
	ErrPaymentDeclined   = errors.New("payment declined")
	ErrNoCourier         = errors.New("no courier available")
	ErrRefundUnavailable = errors.New("refund service unavailable")
)

// Each service below stands in for a separate microservice with its own
// database. That is the whole reason a saga is needed: there is no single
// transaction spanning all three, so consistency has to be built by hand.

type InventoryService struct {
	failFor  map[string]bool
	reserved map[string]bool
}

func NewInventoryService(failFor map[string]bool) *InventoryService {
	return &InventoryService{failFor: failFor, reserved: map[string]bool{}}
}

func (s *InventoryService) Reserve(orderID string) error {
	if s.failFor[orderID] {
		return ErrOutOfStock
	}
	s.reserved[orderID] = true
	return nil
}

func (s *InventoryService) Release(orderID string) error {
	delete(s.reserved, orderID)
	return nil
}

func (s *InventoryService) IsReserved(orderID string) bool { return s.reserved[orderID] }

type PaymentService struct {
	failFor       map[string]bool
	refundFailFor map[string]bool
	charged       map[string]bool
}

func NewPaymentService(failFor, refundFailFor map[string]bool) *PaymentService {
	return &PaymentService{
		failFor:       failFor,
		refundFailFor: refundFailFor,
		charged:       map[string]bool{},
	}
}

func (s *PaymentService) Charge(orderID string) error {
	if s.failFor[orderID] {
		return ErrPaymentDeclined
	}
	s.charged[orderID] = true
	return nil
}

// Refund is a compensating action that can itself fail - the case people
// forget when they first meet sagas.
func (s *PaymentService) Refund(orderID string) error {
	if s.refundFailFor[orderID] {
		return ErrRefundUnavailable
	}
	delete(s.charged, orderID)
	return nil
}

func (s *PaymentService) IsCharged(orderID string) bool { return s.charged[orderID] }

type ShippingService struct {
	failFor   map[string]bool
	shipments map[string]bool
}

func NewShippingService(failFor map[string]bool) *ShippingService {
	return &ShippingService{failFor: failFor, shipments: map[string]bool{}}
}

func (s *ShippingService) CreateShipment(orderID string) error {
	if s.failFor[orderID] {
		return ErrNoCourier
	}
	s.shipments[orderID] = true
	return nil
}

func (s *ShippingService) CancelShipment(orderID string) error {
	delete(s.shipments, orderID)
	return nil
}

func (s *ShippingService) HasShipment(orderID string) bool { return s.shipments[orderID] }
