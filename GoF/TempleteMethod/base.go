package main

import "fmt"

type ReportGenerator interface {
	LoadData()
	FormatData()
	Export()
}

type BaseReport struct {
	Generator ReportGenerator
}

func (b *BaseReport) Generate() {
	b.Generator.LoadData()
	b.Generator.FormatData()
	b.Generator.Export()

	fmt.Println("Send notification")
}
