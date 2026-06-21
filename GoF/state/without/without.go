package without

type Order struct {
	state string
}

func (o *Order) Pay() {
	if o.state == "pending" {
		o.state = "paid"
	}
}

func (o *Order) Ship() {
	if o.state == "paid" {
		o.state = "shipped"
	}
}

func (o *Order) Cancel() {
	if o.state == "pending" {
		o.state = "cancelled"
	}
}

// too complicated with if/else
