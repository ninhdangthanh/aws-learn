package main

import "fmt"

type PaymentService struct{}

func (p *PaymentService) Pay(
	method string,
	amount int,
) {

	if method == "CARD" {

		fmt.Println(
			"Pay with credit card",
		)

	}

	if method == "CASH" {

		fmt.Println(
			"Pay with cash",
		)

	}

	if method == "PAYPAL" {

		fmt.Println(
			"Pay with paypal",
		)

	}
}
