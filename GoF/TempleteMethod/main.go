package main

import "log"

func main() {
	for _, generator := range []ReportGenerator{&PDFReport{}, &ExcelReport{}} {
		base := BaseReport{Generator: generator}
		if err := base.Generate(); err != nil {
			log.Printf("could not generate report: %v", err)
		}
	}
}
