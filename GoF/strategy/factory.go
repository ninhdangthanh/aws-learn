package main

import "fmt"

const (
	PaymentMethodCard = "CARD"
	PaymentMethodMoMo = "MOMO"
	PaymentMethodCash = "CASH"
)

func NewPaymentStrategy(method string) (IPaymentStrategy, error) {
	switch method {
	case PaymentMethodCard:
		return &CardPayment{}, nil
	case PaymentMethodMoMo:
		return &MoMoPayment{}, nil
	case PaymentMethodCash:
		return &CashPayment{}, nil
	default:
		return nil, fmt.Errorf("unsupported payment method: %s", method)
	}
}
