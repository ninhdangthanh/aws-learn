package main

import "fmt"

type PDFReport struct{}

func (p *PDFReport) LoadData() {
	fmt.Println("Load data")
}

func (p *PDFReport) FormatData() {
	fmt.Println("Format PDF")
}

func (p *PDFReport) Export() {
	fmt.Println("Export PDF")
}
