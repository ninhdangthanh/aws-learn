package events

type Event struct {
	Name    string
	OrderID string
	Reason  string
}

const (
	OrderCreated           = "OrderCreated"
	InventoryReserved      = "InventoryReserved"
	InventoryReserveFailed = "InventoryReserveFailed"
	PaymentCharged         = "PaymentCharged"
	PaymentFailed          = "PaymentFailed"
	OrderConfirmed         = "OrderConfirmed"
	OrderFailed            = "OrderFailed"
)
