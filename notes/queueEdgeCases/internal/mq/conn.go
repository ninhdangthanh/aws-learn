// Package mq chứa topology và helper dùng chung cho producer/consumer/dlqwatcher.
package mq

import (
	"os"

	amqp "github.com/rabbitmq/amqp091-go"
)

func amqpURL() string {
	if v := os.Getenv("AMQP_URL"); v != "" {
		return v
	}
	return "amqp://guest:guest@localhost:5682/"
}

// Dial mở connection + channel, và đảm bảo topology (queue/TTL/dead-letter)
// đã tồn tại trước khi publish/consume.
func Dial() (*amqp.Connection, *amqp.Channel, error) {
	conn, err := amqp.Dial(amqpURL())
	if err != nil {
		return nil, nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, nil, err
	}

	if err := declareTopology(ch); err != nil {
		ch.Close()
		conn.Close()
		return nil, nil, err
	}

	return conn, ch, nil
}
