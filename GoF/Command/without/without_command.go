package main

import "fmt"

type Light struct{}

func (l *Light) TurnOn() {
	fmt.Println("Light ON")
}

func (l *Light) TurnOff() {
	fmt.Println("Light OFF")
}

func execute() {
	light := &Light{}

	light.TurnOn()
	light.TurnOff()
}
