package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/kafka-commerce/backend/api"
	"github.com/kafka-commerce/backend/config"
	"github.com/kafka-commerce/backend/database"
	"github.com/kafka-commerce/backend/kafka"
	"github.com/kafka-commerce/backend/service"
)

// corsMiddleware adds CORS headers for frontend communication
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func main() {
	// Load environment variables
	godotenv.Load()

	// Load config
	cfg := config.Load()

	// Initialize database
	db, err := database.InitDB(cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Initialize Kafka topics with 3 partitions for Phase 2 demonstration
	numPartitions := 3
	if partStr := os.Getenv("KAFKA_PARTITIONS"); partStr != "" {
		if p, err := strconv.Atoi(partStr); err == nil {
			numPartitions = p
		}
	}

	if err := kafka.EnsureTopics(cfg.KafkaBrokers, []string{"orders"}, numPartitions); err != nil {
		log.Printf("Warning: Failed to create topics: %v", err)
	}

	// Initialize Kafka producer
	producer := kafka.NewProducer(cfg.KafkaBrokers)
	defer producer.Close()

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup API server
	router := mux.NewRouter()
	router.Use(corsMiddleware)

	orderAPI := api.NewOrderAPI(db, producer)
	orderAPI.RegisterRoutes(router)

	// Phase 2: Consumer status API
	consumerStatusAPI := api.NewConsumerStatusAPI()
	consumerStatusAPI.RegisterRoutes(router)

	// Health check endpoint
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Get number of consumer instances from environment (Phase 2 feature)
	numInstances := 3
	if instStr := os.Getenv("CONSUMER_INSTANCES"); instStr != "" {
		if n, err := strconv.Atoi(instStr); err == nil && n > 0 {
			numInstances = n
		}
	}

	log.Printf("Starting %d consumer instances per service (Phase 2 - Consumer Groups)", numInstances)
	log.Printf("Using %d partitions for 'orders' topic", numPartitions)

	// Start consumer services in separate goroutines
	// Each service can have multiple instances for horizontal scaling
	for i := 1; i <= numInstances; i++ {
		go func(instanceNum int) {
			instanceID := fmt.Sprintf("payment-%d", instanceNum)
			svc := service.NewPaymentService(cfg.KafkaBrokers, instanceID)
			svc.Start(ctx)
		}(i)
	}

	for i := 1; i <= numInstances; i++ {
		go func(instanceNum int) {
			instanceID := fmt.Sprintf("inventory-%d", instanceNum)
			svc := service.NewInventoryService(cfg.KafkaBrokers, instanceID)
			svc.Start(ctx)
		}(i)
	}

	for i := 1; i <= numInstances; i++ {
		go func(instanceNum int) {
			instanceID := fmt.Sprintf("analytics-%d", instanceNum)
			svc := service.NewAnalyticsService(cfg.KafkaBrokers, instanceID)
			svc.Start(ctx)
		}(i)
	}

	for i := 1; i <= numInstances; i++ {
		go func(instanceNum int) {
			instanceID := fmt.Sprintf("notification-%d", instanceNum)
			svc := service.NewNotificationService(cfg.KafkaBrokers, instanceID)
			svc.Start(ctx)
		}(i)
	}

	// Start HTTP server
	server := &http.Server{
		Addr:    ":" + cfg.ServerPort,
		Handler: router,
	}

	log.Printf("Server starting on port %s", cfg.ServerPort)

	// Start server in a goroutine
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down server...")
	cancel()

	// Give services time to shutdown gracefully
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("Server stopped")
}
