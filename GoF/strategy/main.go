package main

func main() {
	orders := []struct {
		method string
		amount int
	}{
		{method: PaymentMethodCard, amount: 100},
		{method: PaymentMethodMoMo, amount: 200},
		{method: PaymentMethodCash, amount: 50},
	}

	for _, order := range orders {
		payment, err := NewPaymentStrategy(order.method)
		if err != nil {
			panic(err)
		}

		checkout := NewCheckout(payment)
		checkout.Checkout(order.amount)
	}
}
