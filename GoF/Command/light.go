package main

import "fmt"

type Light struct {
	name string
	isOn bool
}

func (l *Light) TurnOn() {
	l.isOn = true
	fmt.Println(l.name, "is ON")
}

func (l *Light) TurnOff() {
	l.isOn = false
	fmt.Println(l.name, "is OFF")
}

type LightOnCommand struct {
	light *Light
}

func (c *LightOnCommand) Execute() {
	c.light.TurnOn()
}

func (c *LightOnCommand) Undo() {
	c.light.TurnOff()
}

type LightOffCommand struct {
	light *Light
}

func (c *LightOffCommand) Execute() {
	c.light.TurnOff()
}

func (c *LightOffCommand) Undo() {
	c.light.TurnOn()
}
