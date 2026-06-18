package main

import "fmt"

type IPaymentStrategy interface {
	Pay(amount int) error
}

type CardPayment struct{}

func (c *CardPayment) Pay(
	amount int,
) error {

	fmt.Println(
		"Pay by CARD:",
		amount,
	)

	return nil
}

type MoMoPayment struct{}

func (c *MoMoPayment) Pay(
	amount int,
) error {

	fmt.Println(
		"Pay by MOMO:",
		amount,
	)

	return nil
}

type CashPayment struct{}

func (c *CashPayment) Pay(
	amount int,
) error {

	fmt.Println(
		"Pay by CASH:",
		amount,
	)

	return nil
}
