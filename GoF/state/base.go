package main

type State interface {
	Pay(order *Order)
	Ship(order *Order)
}

type Order struct {
	state State
}

func (o *Order) SetState(state State) {
	o.state = state
}

func (o *Order) Pay() {
	o.state.Pay(o)
}

func (o *Order) Ship() {
	o.state.Ship(o)
}
