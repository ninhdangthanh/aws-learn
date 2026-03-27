package messaging

import (
	"context"
	"encoding/json"
	"time"

	"github.com/go-template/models"
	amqp "github.com/rabbitmq/amqp091-go"
)

func PublishUserCreatedEvent(user *models.User) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	body, err := json.Marshal(user)
	if err != nil {
		return err
	}

	return GetChannel().PublishWithContext(ctx,
		"users_exchange",
		"user.created",
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		})
}
