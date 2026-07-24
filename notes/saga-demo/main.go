package main

import "fmt"

// buildSaga wires the three services into the fixed order-fulfilment sequence.
// The orchestrator is the single place that knows the order of the steps and
// what undoes each one.
func buildSaga(inv *InventoryService, pay *PaymentService, ship *ShippingService, log func(string)) *Saga {
	return NewSaga(log,
		Step{
			Name:       "ReserveStock",
			Do:         inv.Reserve,
			Compensate: inv.Release,
		},
		Step{
			Name:       "ChargePayment",
			Do:         pay.Charge,
			Compensate: pay.Refund,
		},
		Step{
			Name:       "CreateShipment",
			Do:         ship.CreateShipment,
			Compensate: ship.CancelShipment,
		},
	)
}

func main() {
	log := func(s string) { fmt.Println(s) }

	inv := NewInventoryService(map[string]bool{"order-no-stock": true})
	pay := NewPaymentService(
		map[string]bool{"order-card-declined": true},
		map[string]bool{"order-refund-broken": true},
	)
	ship := NewShippingService(map[string]bool{
		"order-no-courier":    true,
		"order-refund-broken": true,
	})

	saga := buildSaga(inv, pay, ship, log)

	run := func(title, orderID string) {
		fmt.Printf("%s\n", title)
		res := saga.Execute(orderID)
		if res.OK() {
			fmt.Printf("  => %s CONFIRMED (completed: %v)\n\n", orderID, res.Completed)
			return
		}
		fmt.Printf("  => %s FAILED at %s (%v); compensated: %v\n\n",
			orderID, res.FailedStep, res.FailureCause, res.Compensated)
	}

	run("Scenario 1: every step succeeds", "order-happy")

	run("Scenario 2: first step fails -> nothing to compensate", "order-no-stock")

	run("Scenario 3: middle step fails -> compensate the one step already done", "order-card-declined")

	run("Scenario 4: last step fails -> compensate two steps, in reverse order", "order-no-courier")

	run("Scenario 5: the compensation itself fails -> stuck, needs a human", "order-refund-broken")

	fmt.Println("Final state held by each service:")
	for _, id := range []string{"order-happy", "order-no-stock", "order-card-declined", "order-no-courier", "order-refund-broken"} {
		fmt.Printf("  %-20s stock_reserved=%-5v charged=%-5v shipment=%v\n",
			id, inv.IsReserved(id), pay.IsCharged(id), ship.HasShipment(id))
	}
	fmt.Println("\n  note order-refund-broken: stock was released but the money is still taken.")
	fmt.Println("  that row is the whole reason compensation failures need alerting.")
}
