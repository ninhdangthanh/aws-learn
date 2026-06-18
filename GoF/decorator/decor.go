package main

import "fmt"

type Americano struct {
	cf ICoffee
}

func (a *Americano) MakeCoffee() {
	a.cf.MakeCoffee()
	fmt.Println("Make Americano....")
}

type SugerCoffee struct {
	cf ICoffee
}

func (s *SugerCoffee) MakeCoffee() {
	s.cf.MakeCoffee()
	fmt.Println("Make Sugar coffee....")
}
