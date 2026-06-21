package main

import "fmt"

type PDFReport struct{}

func (p *PDFReport) Name() string { return "PDF sales" }

func (p *PDFReport) LoadData() ([]Order, error) {
	fmt.Println("Loading paid orders from the reporting database")
	return []Order{{ID: "ORD-100", Amount: 450_000, Paid: true}}, nil
}

func (p *PDFReport) FormatData(orders []Order) (string, error) {
	fmt.Println("Formatting orders as a branded PDF with totals")
	return fmt.Sprintf("PDF: %d orders", len(orders)), nil
}

func (p *PDFReport) Export(content string) (string, error) {
	fmt.Printf("Uploading %q to document storage\n", content)
	return "s3://reports/sales.pdf", nil
}

func (p *PDFReport) SendNotification(location string) {
	fmt.Printf("Emailing finance a link to %s\n", location)
}
