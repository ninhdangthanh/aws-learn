package main

import "fmt"

func main() {
	bus := NewBus()
	writeStore := NewWriteStore()
	readStore := NewReadStore()
	Project(bus, readStore) // the read side subscribes to the write side's events

	cmds := NewCommandBus(writeStore, bus)
	queries := NewQueryBus(readStore)

	showRead := func(orderID string) {
		v, _ := queries.GetSummary(orderID)
		fmt.Printf("  read  : status=%-5s items=%d total=%.0f\n", v.Status, v.ItemCount, v.Total)
	}
	showWrite := func(orderID string) {
		o, _ := writeStore.Get(orderID)
		fmt.Printf("  write : paid=%-5v lines=%v\n", o.Paid, o.Lines)
	}

	fmt.Println("Scenario 1: a command updates the write model, an event updates the read model")
	_ = cmds.PlaceOrder("ord-1")
	_ = cmds.AddItem("ord-1", "keyboard", 120)
	_ = cmds.AddItem("ord-1", "mouse", 40)
	showWrite("ord-1")
	showRead("ord-1")
	fmt.Println("  ^ same order, two shapes: the write side keeps lines, the read side keeps totals")
	fmt.Println()

	fmt.Println("Scenario 2: commands are validated against the write model and can be rejected")
	fmt.Printf("  AddItem to a missing order : %v\n", cmds.AddItem("nope", "x", 10))
	_ = cmds.PlaceOrder("ord-empty")
	fmt.Printf("  PayOrder on an empty order : %v\n", cmds.PayOrder("ord-empty"))
	fmt.Println()

	fmt.Println("Scenario 3: a rejected command publishes nothing, so the read model never moves")
	_ = cmds.PayOrder("ord-1")
	before, _ := queries.GetSummary("ord-1")
	err := cmds.AddItem("ord-1", "late-item", 999)
	after, _ := queries.GetSummary("ord-1")
	fmt.Printf("  AddItem to a paid order    : %v\n", err)
	fmt.Printf("  read model total: %.0f -> %.0f (unchanged)\n", before.Total, after.Total)
	fmt.Println()

	fmt.Println("Scenario 4: bypass the command bus and the read model never hears about it")
	sneaky, _ := writeStore.Get("ord-1")
	sneaky.Lines = append(sneaky.Lines, OrderLine{Item: "smuggled", Price: 500})
	writeStore.Save(sneaky) // saved directly - no event published
	showWrite("ord-1")
	showRead("ord-1")
	fmt.Println("  the two now disagree, and that is the proof they are separate stores.")
	fmt.Println("  the event is the only bridge - remove it and nothing crosses.")
	fmt.Println()

	fmt.Println("All summaries (read side only):")
	for _, v := range queries.ListSummaries() {
		fmt.Printf("  %-10s %-5s items=%d total=%.0f\n", v.OrderID, v.Status, v.ItemCount, v.Total)
	}
}
