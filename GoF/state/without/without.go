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

func (o *Order) Deliver() {
	if o.state == "shipped" {
		o.state = "delivered"
	}
}

func (o *Order) Cancel() {
	if o.state == "pending" || o.state == "paid" {
		o.state = "cancelled"
	}
}

// More actions and states make these conditions spread across the Order type.
