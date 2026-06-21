package main

import "fmt"

type ReportGenerator interface {
	Name() string
	LoadData() ([]Order, error)
	FormatData(orders []Order) (string, error)
	Export(content string) (string, error)
	SendNotification(location string)
}

type BaseReport struct {
	Generator ReportGenerator
}

type Order struct {
	ID     string
	Amount int
	Paid   bool
}

// Generate is the template method: every report follows this fixed workflow.
func (b *BaseReport) Generate() error {
	fmt.Printf("Generating %s report\n", b.Generator.Name())

	orders, err := b.Generator.LoadData()
	if err != nil {
		return fmt.Errorf("load data: %w", err)
	}

	if err := validateOrders(orders); err != nil {
		return err
	}

	content, err := b.Generator.FormatData(orders)
	if err != nil {
		return fmt.Errorf("format data: %w", err)
	}

	location, err := b.Generator.Export(content)
	if err != nil {
		return fmt.Errorf("export report: %w", err)
	}

	b.Generator.SendNotification(location)
	return nil
}

func validateOrders(orders []Order) error {
	if len(orders) == 0 {
		return fmt.Errorf("report has no orders")
	}

	for _, order := range orders {
		if !order.Paid || order.Amount <= 0 {
			return fmt.Errorf("order %s is not eligible for the report", order.ID)
		}
	}

	fmt.Printf("Validated %d paid orders\n", len(orders))
	return nil
}
