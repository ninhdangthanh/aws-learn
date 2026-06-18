package main

import "fmt"

func main() {
	var coffee ICoffee = &Coffee{}
	coffee.MakeCoffee()
	fmt.Println("----------------------------------------------------")

	coffee = &Americano{
		cf: coffee,
	}

	coffee.MakeCoffee()
	fmt.Println("----------------------------------------------------")

	coffee = &SugerCoffee{
		cf: coffee,
	}

	coffee.MakeCoffee()
	fmt.Println("----------------------------------------------------")
}
