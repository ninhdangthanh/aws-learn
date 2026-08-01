package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Config hard-code cho demo, khớp với idempotency/docker-compose.yml.
const (
	postgresDSN = "postgres://postgres:postgres@127.0.0.1:5432/idempotency_lab?sslmode=disable"
	redisAddr   = "127.0.0.1:6379"
	httpAddr    = ":8020"
	cacheTTL    = time.Hour
)

type App struct {
	repo  *ProductRepo
	cache *ProductCache
}

func main() {
	ctx := context.Background()

	repo, err := NewProductRepo(postgresDSN)
	if err != nil {
		log.Fatal("connect postgres: ", err)
	}
	defer repo.Close()

	if err := repo.Migrate(ctx); err != nil {
		log.Fatal("migrate postgres: ", err)
	}

	cache, err := NewProductCache(redisAddr, cacheTTL)
	if err != nil {
		log.Fatal("connect redis: ", err)
	}
	defer cache.Close()

	app := &App{repo: repo, cache: cache}

	router := gin.Default()
	router.GET("/health", app.health)
	router.GET("/clients/:clientID/products", app.listProducts)
	router.POST("/clients/:clientID/products", app.createProduct)
	router.PUT("/clients/:clientID/products/:productID", app.updateProduct)
	router.DELETE("/clients/:clientID/products/:productID", app.deleteProduct)
	router.POST("/clients/:clientID/orders", app.createOrder)

	log.Printf("cache-products api listening on %s", httpAddr)
	if err := router.Run(httpAddr); err != nil {
		log.Fatal(err)
	}
}

func (a *App) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// listProducts dùng cache-aside: đọc Redis trước, miss thì đọc Postgres rồi ghi lại cache.
func (a *App) listProducts(c *gin.Context) {
	clientID := c.Param("clientID")
	cacheKey := a.cache.Key(clientID)

	if products, hit := a.cache.Get(c.Request.Context(), clientID); hit {
		c.JSON(http.StatusOK, gin.H{
			"source":    "cache",
			"cache_key": cacheKey,
			"products":  products,
		})
		return
	}

	products, err := a.repo.List(c.Request.Context(), clientID)
	if err != nil {
		log.Println("list products:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list_products_failed"})
		return
	}

	a.cache.Set(c.Request.Context(), clientID, products)

	c.JSON(http.StatusOK, gin.H{
		"source":    "db",
		"cache_key": cacheKey,
		"ttl":       a.cache.TTL().String(),
		"products":  products,
	})
}

func (a *App) createProduct(c *gin.Context) {
	clientID := c.Param("clientID")

	var input ProductInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
		return
	}

	product, err := a.repo.Create(c.Request.Context(), clientID, input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	a.cache.Del(c.Request.Context(), clientID)
	c.JSON(http.StatusCreated, product)
}

func (a *App) updateProduct(c *gin.Context) {
	clientID := c.Param("clientID")
	productID, ok := parseProductID(c)
	if !ok {
		return
	}

	var input ProductInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
		return
	}

	product, err := a.repo.Update(c.Request.Context(), clientID, productID, input)
	if errors.Is(err, ErrProductNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	a.cache.Del(c.Request.Context(), clientID)
	c.JSON(http.StatusOK, product)
}

func (a *App) deleteProduct(c *gin.Context) {
	clientID := c.Param("clientID")
	productID, ok := parseProductID(c)
	if !ok {
		return
	}

	err := a.repo.Delete(c.Request.Context(), clientID, productID)
	if errors.Is(err, ErrProductNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		log.Println("delete product:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "delete_product_failed"})
		return
	}

	a.cache.Del(c.Request.Context(), clientID)
	c.Status(http.StatusNoContent)
}

// createOrder cố tình không đọc cache: giá tiền luôn phải lấy trực tiếp từ Postgres.
func (a *App) createOrder(c *gin.Context) {
	clientID := c.Param("clientID")

	var req CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
		return
	}
	if len(req.Items) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "items are required"})
		return
	}

	lines := make([]OrderLine, 0, len(req.Items))
	var total int64
	for _, item := range req.Items {
		if item.ProductID <= 0 || item.Quantity <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "product_id and quantity must be positive"})
			return
		}

		product, err := a.repo.Get(c.Request.Context(), clientID, item.ProductID)
		if errors.Is(err, ErrProductNotFound) || (err == nil && !product.Active) {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("product %d is not available", item.ProductID)})
			return
		}
		if err != nil {
			log.Println("get product:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "get_product_failed"})
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

	c.JSON(http.StatusCreated, gin.H{
		"price_source": "db",
		"lines":        lines,
		"total":        total,
	})
}

func parseProductID(c *gin.Context) (int64, bool) {
	productID, err := strconv.ParseInt(c.Param("productID"), 10, 64)
	if err != nil || productID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product_id"})
		return 0, false
	}
	return productID, true
}
