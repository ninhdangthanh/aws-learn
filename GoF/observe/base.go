package main

import "fmt"

type Observer interface {
	Update(orderID string, status string)
}

type Order struct {
	id        string
	status    string
	observers []Observer
}

func (o *Order) Attach(observer Observer) {
	o.observers = append(o.observers, observer)
}

func (o *Order) Notify() {
	for _, observer := range o.observers {
		observer.Update(
			o.id,
			o.status,
		)
	}
}

func (o *Order) ChangeStatus(status string) {
	o.status = status
	fmt.Println("changed status.....")

	o.Notify()
}
