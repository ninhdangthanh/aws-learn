package messaging

import (
	"context"
	"encoding/json"
	"time"

	"github.com/go-template/models"
	amqp "github.com/rabbitmq/amqp091-go"
)

// PublishOrderCreatedEvent publishes an order creation event via RabbitMQ
func PublishOrderCreatedEvent(order *models.Order) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	body, err := json.Marshal(order)
	if err != nil {
		return err
	}

	err = GetChannel().PublishWithContext(ctx,
		"orders_exchange", // exchange
		"order.created",   // routing key
		false,             // mandatory
		false,             // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		})
	return err
}
