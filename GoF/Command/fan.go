package main

import "fmt"

type Fan struct {
	speed int
}

func (f *Fan) Start() {
	fmt.Println("Fan start")
}

func (f *Fan) Stop() {
	fmt.Println("Fan stop")
}

type FanStartCommand struct {
	fan *Fan
}

func (c *FanStartCommand) Execute() {
	c.fan.Start()
}

func (c *FanStartCommand) Undo() {
	c.fan.Stop()
}
