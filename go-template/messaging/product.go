package messaging

import (
	"context"
	"encoding/json"
	"time"

	"github.com/go-template/models"
	amqp "github.com/rabbitmq/amqp091-go"
)

func PublishProductCreatedEvent(product *models.Product) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	body, err := json.Marshal(product)
	if err != nil {
		return err
	}

	return GetChannel().PublishWithContext(ctx,
		"products_exchange",
		"product.created",
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		})
}
