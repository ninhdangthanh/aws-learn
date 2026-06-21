package main

import "fmt"

type State interface {
	Name() string
	Pay(order *Order)
	Ship(order *Order)
	Deliver(order *Order)
	Cancel(order *Order)
}

type Order struct {
	ID    string
	Total int
	state State
}

func (o *Order) SetState(state State) {
	if o.state != nil {
		fmt.Printf("Order %s: %s -> %s\n", o.ID, o.state.Name(), state.Name())
	}
	o.state = state
}

func (o *Order) Pay() {
	o.state.Pay(o)
}

func (o *Order) Ship() {
	o.state.Ship(o)
}

func (o *Order) Deliver() {
	o.state.Deliver(o)
}

func (o *Order) Cancel() {
	o.state.Cancel(o)
}
