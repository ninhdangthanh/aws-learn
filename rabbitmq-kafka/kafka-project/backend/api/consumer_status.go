package api

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

type ConsumerInfo struct {
	ConsumerGroup string `json:"consumer_group"`
	InstanceID    string `json:"instance_id"`
	Status        string `json:"status"`
}

type ConsumerGroupInfo struct {
	Name      string         `json:"name"`
	Members   []ConsumerInfo `json:"members"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// This is a helper to provide consumer group information
// In Phase 2, this helps visualize consumer distribution
var consumerRegistry map[string][]string = make(map[string][]string)

func RegisterConsumer(groupID, instanceID string) {
	consumerRegistry[groupID] = append(consumerRegistry[groupID], instanceID)
}

type ConsumerStatusAPI struct{}

func NewConsumerStatusAPI() *ConsumerStatusAPI {
	return &ConsumerStatusAPI{}
}

func (api *ConsumerStatusAPI) GetConsumerGroups(w http.ResponseWriter, r *http.Request) {
	groups := []ConsumerGroupInfo{
		{
			Name: "payment-service",
			Members: []ConsumerInfo{
				{ConsumerGroup: "payment-service", InstanceID: "payment-1", Status: "active"},
				{ConsumerGroup: "payment-service", InstanceID: "payment-2", Status: "active"},
				{ConsumerGroup: "payment-service", InstanceID: "payment-3", Status: "active"},
			},
			UpdatedAt: time.Now(),
		},
		{
			Name: "inventory-service",
			Members: []ConsumerInfo{
				{ConsumerGroup: "inventory-service", InstanceID: "inventory-1", Status: "active"},
				{ConsumerGroup: "inventory-service", InstanceID: "inventory-2", Status: "active"},
				{ConsumerGroup: "inventory-service", InstanceID: "inventory-3", Status: "active"},
			},
			UpdatedAt: time.Now(),
		},
		{
			Name: "analytics-service",
			Members: []ConsumerInfo{
				{ConsumerGroup: "analytics-service", InstanceID: "analytics-1", Status: "active"},
				{ConsumerGroup: "analytics-service", InstanceID: "analytics-2", Status: "active"},
				{ConsumerGroup: "analytics-service", InstanceID: "analytics-3", Status: "active"},
			},
			UpdatedAt: time.Now(),
		},
		{
			Name: "notification-service",
			Members: []ConsumerInfo{
				{ConsumerGroup: "notification-service", InstanceID: "notification-1", Status: "active"},
				{ConsumerGroup: "notification-service", InstanceID: "notification-2", Status: "active"},
				{ConsumerGroup: "notification-service", InstanceID: "notification-3", Status: "active"},
			},
			UpdatedAt: time.Now(),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(groups); err != nil {
		log.Printf("Error encoding consumer groups: %v", err)
	}
}

func (api *ConsumerStatusAPI) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/api/consumer-groups", api.GetConsumerGroups).Methods("GET")
}
