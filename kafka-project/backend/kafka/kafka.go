package kafka

import (
	"context"
	"log"
	"net"
	"strconv"
	"time"

	"github.com/segmentio/kafka-go"
)

type Producer struct {
	writer *kafka.Writer
}

type Consumer struct {
	reader *kafka.Reader
}

func NewProducer(brokers []string) *Producer {
	writer := kafka.NewWriter(kafka.WriterConfig{
		Brokers:      brokers,
		RequiredAcks: int(kafka.RequireAll),
	})

	return &Producer{writer: writer}
}

func (p *Producer) PublishEvent(ctx context.Context, topic string, key, value []byte) error {
	err := p.writer.WriteMessages(ctx, kafka.Message{
		Topic: topic,
		Key:   key,
		Value: value,
	})

	if err != nil {
		log.Printf("Error publishing to topic %s: %v", topic, err)
		return err
	}

	return nil
}

func (p *Producer) Close() error {
	return p.writer.Close()
}

func NewConsumer(brokers []string, topic, groupID string, startOffset int64) *Consumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokers,
		Topic:          topic,
		GroupID:        groupID,
		CommitInterval: time.Second,
	})

	return &Consumer{reader: reader}
}

func (c *Consumer) ReadMessage(ctx context.Context) (kafka.Message, error) {
	return c.reader.FetchMessage(ctx)
}

func (c *Consumer) CommitMessage(ctx context.Context, msg kafka.Message) error {
	return c.reader.CommitMessages(ctx, msg)
}

func (c *Consumer) Close() error {
	return c.reader.Close()
}

func EnsureTopics(brokers []string, topics []string, numPartitions int) error {
	conn, err := kafka.Dial("tcp", brokers[0])
	if err != nil {
		return err
	}
	defer conn.Close()

	controller, err := conn.Controller()
	if err != nil {
		return err
	}

	connController, err := kafka.Dial("tcp", net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port)))
	if err != nil {
		return err
	}
	defer connController.Close()

	for _, topic := range topics {
		topicConfigs := []kafka.TopicConfig{
			{
				Topic:             topic,
				NumPartitions:     numPartitions,
				ReplicationFactor: 1,
			},
		}

		err := connController.CreateTopics(topicConfigs...)
		if err != nil {
			log.Printf("Error creating topic %s (may already exist): %v", topic, err)
		}
	}

	return nil
}
