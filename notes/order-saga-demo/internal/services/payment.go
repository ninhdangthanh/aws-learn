package services

import "errors"

var ErrPaymentDeclined = errors.New("payment declined")

type PaymentService struct {
	fail func(orderID string) bool
}

func NewPaymentService(fail func(orderID string) bool) *PaymentService {
	if fail == nil {
		fail = func(string) bool { return false }
	}
	return &PaymentService{fail: fail}
}

func (s *PaymentService) Charge(orderID string, amount float64) error {
	if s.fail(orderID) {
		return ErrPaymentDeclined
	}
	return nil
}
