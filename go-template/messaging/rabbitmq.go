package messaging

import (
	"log"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

var (
	rmqConnInstance    *amqp.Connection
	rmqChannelInstance *amqp.Channel
	rmqOnce            sync.Once
)

func InitRabbitMQ(url string) {
	rmqOnce.Do(func() {
		var err error
		for i := 0; i < 5; i++ {
			rmqConnInstance, err = amqp.Dial(url)
			if err == nil {
				rmqChannelInstance, err = rmqConnInstance.Channel()
				if err == nil {
					err = rmqChannelInstance.ExchangeDeclare(
						"orders_exchange", // name
						"topic",           // type
						true,              // durable
						false,             // auto-deleted
						false,             // internal
						false,             // no-wait
						nil,               // arguments
					)
					if err == nil {
						log.Println("Connected to RabbitMQ successfully")
						return
					}
				}
			}
			log.Printf("Waiting for RabbitMQ (attempt %d/5)... error: %v", i+1, err)
			time.Sleep(5 * time.Second)
		}
		log.Fatalf("Could not connect to RabbitMQ after 5 attempts: %v", err)
	})
}

func GetChannel() *amqp.Channel {
	if rmqChannelInstance == nil {
		log.Println("RabbitMQ channel is nil. Did you call InitRabbitMQ?")
	}
	return rmqChannelInstance
}

func CloseRabbitMQ() {
	if rmqChannelInstance != nil {
		rmqChannelInstance.Close()
	}
	if rmqConnInstance != nil {
		rmqConnInstance.Close()
	}
}
