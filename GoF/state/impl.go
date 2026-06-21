package main

import "fmt"

type PendingState struct{}

func (s *PendingState) Pay(order *Order) {
	fmt.Println("Payment success")
	order.SetState(&PaidState{})
}

func (s *PendingState) Ship(order *Order) {
	fmt.Println("Cannot ship unpaid order")
}

type PaidState struct{}

func (s *PaidState) Pay(order *Order) {
	fmt.Println("Already paid")
}

func (s *PaidState) Ship(order *Order) {
	fmt.Println("Order shipped")
}
