package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const cacheTTL = time.Hour

type Product struct {
	ID       int64  `json:"id"`
	ClientID string `json:"client_id"`
	Name     string `json:"name"`
	Price    int64  `json:"price"`
	Active   bool   `json:"active"`
}

type ProductInput struct {
	Name   string `json:"name"`
	Price  int64  `json:"price"`
	Active *bool  `json:"active,omitempty"`
}

type OrderItemInput struct {
	ProductID int64 `json:"product_id"`
	Quantity  int   `json:"quantity"`
}

type CreateOrderRequest struct {
	Items []OrderItemInput `json:"items"`
}

type OrderLine struct {
	ProductID  int64  `json:"product_id"`
	Name       string `json:"name"`
	Quantity   int    `json:"quantity"`
	UnitPrice  int64  `json:"unit_price"`
	LineAmount int64  `json:"line_amount"`
}

type Store struct {
	mu       sync.RWMutex
	nextID   int64
	products map[string]map[int64]Product
}

func NewStore() *Store {
	s := &Store{
		nextID:   1,
		products: make(map[string]map[int64]Product),
	}

	s.mustSeed("1001", "Pizza", 100000)
	s.mustSeed("1001", "Burger", 75000)
	s.mustSeed("1002", "Coffee", 45000)
	return s
}

func (s *Store) mustSeed(clientID, name string, price int64) {
	active := true
	_, err := s.CreateProduct(clientID, ProductInput{Name: name, Price: price, Active: &active})
	if err != nil {
		panic(err)
	}
}

func (s *Store) ListProducts(clientID string) []Product {
	s.mu.RLock()
	defer s.mu.RUnlock()

	productsByID := s.products[clientID]
	products := make([]Product, 0, len(productsByID))
	for _, product := range productsByID {
		products = append(products, product)
	}
	return products
}

func (s *Store) GetProduct(clientID string, productID int64) (Product, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	product, ok := s.products[clientID][productID]
	return product, ok
}

func (s *Store) CreateProduct(clientID string, input ProductInput) (Product, error) {
	if strings.TrimSpace(input.Name) == "" || input.Price <= 0 {
		return Product{}, errors.New("name is required and price must be positive")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.products[clientID] == nil {
		s.products[clientID] = make(map[int64]Product)
	}

	active := true
	if input.Active != nil {
		active = *input.Active
	}

	product := Product{
		ID:       s.nextID,
		ClientID: clientID,
		Name:     strings.TrimSpace(input.Name),
		Price:    input.Price,
		Active:   active,
	}
	s.nextID++
	s.products[clientID][product.ID] = product
	return product, nil
}

func (s *Store) UpdateProduct(clientID string, productID int64, input ProductInput) (Product, error) {
	if strings.TrimSpace(input.Name) == "" || input.Price <= 0 {
		return Product{}, errors.New("name is required and price must be positive")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.products[clientID][productID]
	if !ok {
		return Product{}, errors.New("product not found")
	}

	active := true
	if input.Active != nil {
		active = *input.Active
	}

	product := Product{
		ID:       productID,
		ClientID: clientID,
		Name:     strings.TrimSpace(input.Name),
		Price:    input.Price,
		Active:   active,
	}
	s.products[clientID][productID] = product
	return product, nil
}

func (s *Store) DeleteProduct(clientID string, productID int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.products[clientID][productID]; !ok {
		return false
	}
	delete(s.products[clientID], productID)
	return true
}

type App struct {
	store *Store
	rdb   *redis.Client
}

func main() {
	ctx := context.Background()

	rdb := redis.NewClient(&redis.Options{
		Addr: getenv("REDIS_ADDR", "127.0.0.1:6379"),
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatal("connect redis: ", err)
	}
	defer rdb.Close()

	app := &App{
		store: NewStore(),
		rdb:   rdb,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", app.health)
	mux.HandleFunc("GET /clients/{clientID}/products", app.listProducts)
	mux.HandleFunc("POST /clients/{clientID}/products", app.createProduct)
	mux.HandleFunc("PUT /clients/{clientID}/products/{productID}", app.updateProduct)
	mux.HandleFunc("DELETE /clients/{clientID}/products/{productID}", app.deleteProduct)
	mux.HandleFunc("POST /clients/{clientID}/orders", app.createOrder)

	addr := getenv("HTTP_ADDR", ":8020")
	log.Printf("cache-products api listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func (a *App) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *App) listProducts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	clientID := r.PathValue("clientID")
	cacheKey := productCacheKey(clientID)

	cached, err := a.rdb.Get(ctx, cacheKey).Bytes()
	if err == nil {
		var products []Product
		if err := json.Unmarshal(cached, &products); err == nil {
			writeJSON(w, http.StatusOK, map[string]any{
				"source":    "cache",
				"cache_key": cacheKey,
				"products":  products,
			})
			return
		}
		_ = a.rdb.Del(ctx, cacheKey).Err()
	}
	if err != nil && !errors.Is(err, redis.Nil) {
		log.Println("redis get:", err)
	}

	products := a.store.ListProducts(clientID)
	body, err := json.Marshal(products)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "encode_products_failed"})
		return
	}
	if err := a.rdb.Set(ctx, cacheKey, body, cacheTTL).Err(); err != nil {
		log.Println("redis set:", err)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"source":    "db",
		"cache_key": cacheKey,
		"ttl":       cacheTTL.String(),
		"products":  products,
	})
}

func (a *App) createProduct(w http.ResponseWriter, r *http.Request) {
	clientID := r.PathValue("clientID")

	var input ProductInput
	if !decodeJSON(w, r, &input) {
		return
	}

	product, err := a.store.CreateProduct(clientID, input)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	a.invalidateProducts(r.Context(), clientID)
	writeJSON(w, http.StatusCreated, product)
}

func (a *App) updateProduct(w http.ResponseWriter, r *http.Request) {
	clientID := r.PathValue("clientID")
	productID, ok := parseProductID(w, r)
	if !ok {
		return
	}

	var input ProductInput
	if !decodeJSON(w, r, &input) {
		return
	}

	product, err := a.store.UpdateProduct(clientID, productID, input)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	a.invalidateProducts(r.Context(), clientID)
	writeJSON(w, http.StatusOK, product)
}

func (a *App) deleteProduct(w http.ResponseWriter, r *http.Request) {
	clientID := r.PathValue("clientID")
	productID, ok := parseProductID(w, r)
	if !ok {
		return
	}

	if !a.store.DeleteProduct(clientID, productID) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "product not found"})
		return
	}

	a.invalidateProducts(r.Context(), clientID)
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) createOrder(w http.ResponseWriter, r *http.Request) {
	clientID := r.PathValue("clientID")

	var req CreateOrderRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if len(req.Items) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "items are required"})
		return
	}

	lines := make([]OrderLine, 0, len(req.Items))
	var total int64
	for _, item := range req.Items {
		if item.ProductID <= 0 || item.Quantity <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "product_id and quantity must be positive"})
			return
		}

		product, ok := a.store.GetProduct(clientID, item.ProductID)
		if !ok || !product.Active {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("product %d is not available", item.ProductID)})
			return
		}

		lineAmount := product.Price * int64(item.Quantity)
		total += lineAmount
		lines = append(lines, OrderLine{
			ProductID:  product.ID,
			Name:       product.Name,
			Quantity:   item.Quantity,
			UnitPrice:  product.Price,
			LineAmount: lineAmount,
		})
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"price_source": "db",
		"lines":        lines,
		"total":        total,
	})
}

func (a *App) invalidateProducts(ctx context.Context, clientID string) {
	cacheKey := productCacheKey(clientID)
	if err := a.rdb.Del(ctx, cacheKey).Err(); err != nil {
		log.Println("redis del:", err)
	}
}

func productCacheKey(clientID string) string {
	return "client:" + clientID + ":products"
}

func parseProductID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	productID, err := strconv.ParseInt(r.PathValue("productID"), 10, 64)
	if err != nil || productID <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid product_id"})
		return 0, false
	}
	return productID, true
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if status == http.StatusNoContent {
		return
	}
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Println("write json:", err)
	}
}

func getenv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}
