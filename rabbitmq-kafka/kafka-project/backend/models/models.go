package models

import (
	"time"
)

type Order struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	ProductID string    `json:"product_id"`
	Quantity  int       `json:"quantity"`
	Price     float64   `json:"price"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Event struct {
	EventID   string    `json:"event_id"`
	EventType string    `json:"event_type"`
	Data      string    `json:"data"`
	Timestamp time.Time `json:"timestamp"`
	Version   int       `json:"version"`
}

type ConsumerOffset struct {
	ConsumerGroup string `json:"consumer_group"`
	Topic         string `json:"topic"`
	Partition     int32  `json:"partition"`
	CurrentOffset int64  `json:"current_offset"`
	HighWaterMark int64  `json:"high_water_mark"`
	Lag           int64  `json:"lag"`
}

type ConsumerStatus struct {
	ConsumerGroup string           `json:"consumer_group"`
	Members       []string         `json:"members"`
	Offsets       []ConsumerOffset `json:"offsets"`
	UpdatedAt     time.Time        `json:"updated_at"`
}
