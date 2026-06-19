package main

func main() {
	order := &Order{
		id: "ORD-001",
	}

	email := &EmailService{}
	logger := &Logger{}

	order.Attach(email)
	order.Attach(logger)

	order.ChangeStatus("PAID")
	order.ChangeStatus("FINISHEDß")
	order.ChangeStatus("REFUND")

	email.CountSentEmail()
	logger.PrintLogs()
}
