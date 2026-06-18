package main

import "fmt"

type ICoffee interface {
	MakeCoffee()
}

type Coffee struct {
}

func (c *Coffee) MakeCoffee() {
	fmt.Println("Make origin coffee...")
}
