package domain

type OrderStatus string

const (
	OrderStatusPending   OrderStatus = "pending"
	OrderStatusConfirmed OrderStatus = "confirmed"
	OrderStatusFailed    OrderStatus = "failed"
)

type Order struct {
	ID     string
	Items  []string
	Amount float64
	Status OrderStatus
}
