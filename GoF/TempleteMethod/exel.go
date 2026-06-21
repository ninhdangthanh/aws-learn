package main

import "fmt"

type ExcelReport struct{}

func (e *ExcelReport) Name() string { return "Excel sales" }

func (e *ExcelReport) LoadData() ([]Order, error) {
	fmt.Println("Loading paid orders from the reporting database")
	return []Order{
		{ID: "ORD-100", Amount: 450_000, Paid: true},
		{ID: "ORD-101", Amount: 120_000, Paid: true},
	}, nil
}

func (e *ExcelReport) FormatData(orders []Order) (string, error) {
	fmt.Println("Formatting orders as an Excel workbook with a pivot table")
	return fmt.Sprintf("XLSX: %d orders", len(orders)), nil
}

func (e *ExcelReport) Export(content string) (string, error) {
	fmt.Printf("Uploading %q to document storage\n", content)
	return "s3://reports/sales.xlsx", nil
}

func (e *ExcelReport) SendNotification(location string) {
	fmt.Printf("Sending operations a Slack message with %s\n", location)
}
