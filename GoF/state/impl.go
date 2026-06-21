package main

import "fmt"

type PendingState struct{}

func (s *PendingState) Name() string { return "pending" }

func (s *PendingState) Pay(order *Order) {
	fmt.Printf("Charging %d for order %s\n", order.Total, order.ID)
	order.SetState(&PaidState{})
}

func (s *PendingState) Ship(order *Order) {
	fmt.Println("Cannot ship an unpaid order")
}

func (s *PendingState) Deliver(order *Order) {
	fmt.Println("Cannot deliver an order that has not been shipped")
}

func (s *PendingState) Cancel(order *Order) {
	fmt.Println("Cancelling unpaid order")
	order.SetState(&CancelledState{})
}

type PaidState struct{}

func (s *PaidState) Name() string { return "paid" }

func (s *PaidState) Pay(order *Order) {
	fmt.Println("Already paid")
}

func (s *PaidState) Ship(order *Order) {
	fmt.Println("Reserving inventory and handing order to carrier")
	order.SetState(&ShippedState{})
}

func (s *PaidState) Deliver(order *Order) {
	fmt.Println("Cannot deliver an order that has not been shipped")
}

func (s *PaidState) Cancel(order *Order) {
	fmt.Println("Refunding paid order")
	order.SetState(&CancelledState{})
}

type ShippedState struct{}

func (s *ShippedState) Name() string { return "shipped" }

func (s *ShippedState) Pay(order *Order) {
	fmt.Println("Order has already been paid and shipped")
}

func (s *ShippedState) Ship(order *Order) {
	fmt.Println("Order is already with the carrier")
}

func (s *ShippedState) Deliver(order *Order) {
	fmt.Println("Confirming delivery with customer")
	order.SetState(&DeliveredState{})
}

func (s *ShippedState) Cancel(order *Order) {
	fmt.Println("Cannot cancel: order is already in transit")
}

type DeliveredState struct{}

func (s *DeliveredState) Name() string { return "delivered" }

func (s *DeliveredState) Pay(order *Order)     { fmt.Println("Order is already delivered") }
func (s *DeliveredState) Ship(order *Order)    { fmt.Println("Order is already delivered") }
func (s *DeliveredState) Deliver(order *Order) { fmt.Println("Order is already delivered") }
func (s *DeliveredState) Cancel(order *Order)  { fmt.Println("Cannot cancel a delivered order") }

type CancelledState struct{}

func (s *CancelledState) Name() string { return "cancelled" }

func (s *CancelledState) Pay(order *Order)     { fmt.Println("Cannot pay a cancelled order") }
func (s *CancelledState) Ship(order *Order)    { fmt.Println("Cannot ship a cancelled order") }
func (s *CancelledState) Deliver(order *Order) { fmt.Println("Cannot deliver a cancelled order") }
func (s *CancelledState) Cancel(order *Order)  { fmt.Println("Order is already cancelled") }
