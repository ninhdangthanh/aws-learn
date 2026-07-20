package services

import "errors"

var ErrInventoryUnavailable = errors.New("inventory unavailable")

type InventoryService struct {
	fail      func(orderID string) bool
	onRelease func(orderID string)
}

func NewInventoryService(fail func(orderID string) bool, onRelease func(orderID string)) *InventoryService {
	if fail == nil {
		fail = func(string) bool { return false }
	}
	if onRelease == nil {
		onRelease = func(string) {}
	}
	return &InventoryService{fail: fail, onRelease: onRelease}
}

func (s *InventoryService) Reserve(orderID string) error {
	if s.fail(orderID) {
		return ErrInventoryUnavailable
	}
	return nil
}

// Release is the saga's compensating action for a successful Reserve.
func (s *InventoryService) Release(orderID string) {
	s.onRelease(orderID)
}
