// internal/events/bus_test.go
package events_test

import (
	"testing"

	"order-saga-demo/internal/events"
)

func TestBus_PublishInvokesSubscribedHandlersInOrder(t *testing.T) {
	bus := events.NewBus()
	var calls []string

	bus.Subscribe("Foo", func(e events.Event) { calls = append(calls, "first:"+e.OrderID) })
	bus.Subscribe("Foo", func(e events.Event) { calls = append(calls, "second:"+e.OrderID) })
	bus.Subscribe("Bar", func(e events.Event) { calls = append(calls, "bar") })

	bus.Publish(events.Event{Name: "Foo", OrderID: "order-1"})

	want := []string{"first:order-1", "second:order-1"}
	if len(calls) != len(want) {
		t.Fatalf("expected %v, got %v", want, calls)
	}
	for i := range want {
		if calls[i] != want[i] {
			t.Fatalf("expected %v, got %v", want, calls)
		}
	}
}
