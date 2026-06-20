package main

import "fmt"

type Light struct {
	name string
}

func (l *Light) TurnOn() {
	fmt.Println(l.name, "turned on")
}

func (l *Light) TurnOff() {
	fmt.Println(l.name, "turned off")
}

type TurnOnCommand struct {
	light *Light
}

func (c *TurnOnCommand) Execute() {
	c.light.TurnOn()
}

type TurnOffCommand struct {
	light *Light
}

func (c *TurnOffCommand) Execute() {
	c.light.TurnOff()
}
