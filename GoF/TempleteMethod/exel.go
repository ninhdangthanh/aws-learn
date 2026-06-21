package main

import "fmt"

type ExcelReport struct{}

func (e *ExcelReport) LoadData() {
	fmt.Println("Load data")
}

func (e *ExcelReport) FormatData() {
	fmt.Println("Format Excel")
}

func (e *ExcelReport) Export() {
	fmt.Println("Export Excel")
}
