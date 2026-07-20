package main

import (
	"fmt"
	"time"

	"order-saga-demo/internal/circuitbreaker"
	"order-saga-demo/internal/cqrs"
	"order-saga-demo/internal/events"
	"order-saga-demo/internal/readmodel"
	"order-saga-demo/internal/saga"
	"order-saga-demo/internal/services"
	"order-saga-demo/internal/writemodel"
)

func main() {
	bus := events.NewBus()
	writeStore := writemodel.NewStore()
	readStore := readmodel.NewStore()
	readmodel.Project(bus, readStore)

	cmdBus := cqrs.NewCommandBus(writeStore, bus)
	queryBus := cqrs.NewQueryBus(readStore)

	inventoryFailFor := map[string]bool{"order-fail-inventory": true}
	inventory := services.NewInventoryService(
		func(orderID string) bool { return inventoryFailFor[orderID] },
		func(orderID string) { fmt.Printf("  [compensate] inventory released for %s\n", orderID) },
	)

	paymentCallCount := 0
	flakyPaymentOrders := map[string]bool{"order-payment-flaky-1": true, "order-payment-flaky-2": true}
	payment := services.NewPaymentService(func(orderID string) bool {
		paymentCallCount++
		return flakyPaymentOrders[orderID]
	})

	breaker := circuitbreaker.New(2, 200*time.Millisecond)
	saga.NewOrderSaga(bus, writeStore, inventory, payment, breaker)

	printResult := func(orderID string) {
		view, _ := queryBus.GetOrder(cqrs.GetOrderQuery{OrderID: orderID})
		fmt.Printf("  => %s status=%s history=%v\n\n", orderID, view.Status, view.StepHistory)
	}

	fmt.Println("Scenario 1: happy path")
	cmdBus.CreateOrder(cqrs.CreateOrderCommand{OrderID: "order-happy", Items: []string{"widget"}, Amount: 100})
	printResult("order-happy")

	fmt.Println("Scenario 2: inventory failure -> compensation")
	cmdBus.CreateOrder(cqrs.CreateOrderCommand{OrderID: "order-fail-inventory", Items: []string{"widget"}, Amount: 50})
	printResult("order-fail-inventory")

	fmt.Println("Scenario 3: payment fails repeatedly -> circuit breaker opens -> fail-fast")
	cmdBus.CreateOrder(cqrs.CreateOrderCommand{OrderID: "order-payment-flaky-1", Items: []string{"widget"}, Amount: 75})
	printResult("order-payment-flaky-1")
	fmt.Printf("  breaker state: %v\n\n", breaker.State())

	cmdBus.CreateOrder(cqrs.CreateOrderCommand{OrderID: "order-payment-flaky-2", Items: []string{"widget"}, Amount: 75})
	printResult("order-payment-flaky-2")
	fmt.Printf("  breaker state: %v\n\n", breaker.State())

	callsBeforeOpenOrder := paymentCallCount
	fmt.Println("  order-breaker-open: submitted while breaker is Open, payment service must NOT be called")
	cmdBus.CreateOrder(cqrs.CreateOrderCommand{OrderID: "order-breaker-open", Items: []string{"widget"}, Amount: 75})
	printResult("order-breaker-open")
	fmt.Printf("  payment service calls: %d before -> %d after (unchanged means fail-fast worked)\n\n",
		callsBeforeOpenOrder, paymentCallCount)

	fmt.Println("Scenario 4: wait for reset timeout -> breaker half-open -> recovers")
	time.Sleep(250 * time.Millisecond)
	fmt.Printf("  breaker state before retry: %v\n", breaker.State())
	cmdBus.CreateOrder(cqrs.CreateOrderCommand{OrderID: "order-recovered", Items: []string{"widget"}, Amount: 75})
	printResult("order-recovered")
	fmt.Printf("  breaker state after retry: %v\n\n", breaker.State())

	fmt.Println("Final read-model query for all orders:")
	for _, v := range readStore.All() {
		fmt.Printf("  %s: %s %v\n", v.OrderID, v.Status, v.StepHistory)
	}
}
