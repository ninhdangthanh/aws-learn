package main

import (
	"fmt"
	"strings"
)

type EmailService struct {
	orderUuids []string
}

func (e *EmailService) Update(
	orderID string,
	status string,
) {
	fmt.Println(
		"send email:",
		orderID,
		status,
	)

	e.orderUuids = append(e.orderUuids, orderID)
}

func (e *EmailService) CountSentEmail() {
	fmt.Println(
		"==> sent email number: ",
		len(e.orderUuids),
	)
}

type Logger struct {
	rows []string
}

func (l *Logger) Update(
	orderID string,
	status string,
) {
	fmt.Println(
		"log:",
		orderID,
		status,
	)

	l.rows = append(l.rows, orderID+"---"+status)
}

func (l *Logger) PrintLogs() {
	var builder strings.Builder
	for i, row := range l.rows {
		builder.WriteString(fmt.Sprintf("-- %d. %s\n", i+1, row))
	}
	fmt.Print(builder.String())
}
