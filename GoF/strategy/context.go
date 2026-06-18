package main

type Checkout struct {
	payment IPaymentStrategy
}

func NewCheckout(
	payment IPaymentStrategy,
) *Checkout {
	return &Checkout{
		payment: payment,
	}

}

func (c *Checkout) Checkout(
	amount int,
) error {

	return c.payment.Pay(amount)

}
