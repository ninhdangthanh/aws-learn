package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	kafkaClient "github.com/kafka-commerce/backend/kafka"
)

type ConsumerStatusAPI struct{}

func NewConsumerStatusAPI() *ConsumerStatusAPI {
	return &ConsumerStatusAPI{}
}

// GetConsumerGroups returns real-time status of all consumer groups from the registry
func (api *ConsumerStatusAPI) GetConsumerGroups(w http.ResponseWriter, r *http.Request) {
	registry := kafkaClient.GetRegistry()
	groups := registry.GetAllGroupStatuses()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(groups); err != nil {
		log.Printf("Error encoding consumer groups: %v", err)
	}
}

// GetRebalanceEvents returns recent rebalance events
func (api *ConsumerStatusAPI) GetRebalanceEvents(w http.ResponseWriter, r *http.Request) {
	registry := kafkaClient.GetRegistry()

	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	events := registry.GetRebalanceEvents(limit)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(events); err != nil {
		log.Printf("Error encoding rebalance events: %v", err)
	}
}

func (api *ConsumerStatusAPI) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/api/consumer-groups", api.GetConsumerGroups).Methods("GET")
	router.HandleFunc("/api/rebalance-events", api.GetRebalanceEvents).Methods("GET")
}
