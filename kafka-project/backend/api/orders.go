package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/kafka-commerce/backend/kafka"
	"github.com/kafka-commerce/backend/models"
)

type OrderAPI struct {
	db       *sql.DB
	producer *kafka.Producer
}

func NewOrderAPI(db *sql.DB, producer *kafka.Producer) *OrderAPI {
	return &OrderAPI{db: db, producer: producer}
}

type CreateOrderRequest struct {
	UserID    string  `json:"user_id"`
	ProductID string  `json:"product_id"`
	Quantity  int     `json:"quantity"`
	Price     float64 `json:"price"`
}

type CreateOrderResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

func (api *OrderAPI) CreateOrder(w http.ResponseWriter, r *http.Request) {
	var req CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	orderID := uuid.New().String()

	// Save order to database
	_, err := api.db.Exec(`INSERT INTO orders (id, user_id, product_id, quantity, price, status)
		VALUES ($1, $2, $3, $4, $5, 'created')`,
		orderID, req.UserID, req.ProductID, req.Quantity, req.Price,
	)
	if err != nil {
		http.Error(w, "Failed to create order", http.StatusInternalServerError)
		return
	}

	// Publish event to Kafka
	event := models.Event{
		EventID:   uuid.New().String(),
		EventType: "order.created",
		Data:      orderID + ":" + req.UserID + ":" + req.ProductID,
		Version:   1,
	}

	eventJSON, _ := json.Marshal(event)
	ctx := context.Background()
	if err := api.producer.PublishEvent(ctx, "orders", []byte(orderID), eventJSON); err != nil {
		log.Printf("Failed to publish event: %v", err)
	}

	resp := CreateOrderResponse{
		ID:     orderID,
		Status: "created",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (api *OrderAPI) GetOrders(w http.ResponseWriter, r *http.Request) {
	rows, err := api.db.Query(`SELECT id, user_id, product_id, quantity, price, status, created_at, updated_at FROM orders`)
	if err != nil {
		http.Error(w, "Failed to fetch orders", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var orders []models.Order
	for rows.Next() {
		var order models.Order
		if err := rows.Scan(&order.ID, &order.UserID, &order.ProductID, &order.Quantity, &order.Price, &order.Status, &order.CreatedAt, &order.UpdatedAt); err != nil {
			log.Printf("Error scanning order: %v", err)
			continue
		}
		orders = append(orders, order)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(orders)
}

func (api *OrderAPI) RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/api/orders", api.CreateOrder).Methods("POST")
	r.HandleFunc("/api/orders", api.GetOrders).Methods("GET")
}
