package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

// Config hard-code cho demo, khớp với idempotency/docker-compose.yml.
const (
	postgresDSN    = "postgres://postgres:postgres@127.0.0.1:5432/idempotency_lab?sslmode=disable"
	redisAddr      = "127.0.0.1:6379"
	httpAddr       = ":8020"
	catalogTTL     = 24 * time.Hour // safety net, không phải cơ chế đồng bộ
	resyncInterval = 5 * time.Minute
)

type App struct {
	repo    *CatalogRepo
	cache   *CatalogCache
	catalog *CatalogService
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	repo, err := NewCatalogRepo(postgresDSN)
	if err != nil {
		log.Fatal("connect postgres: ", err)
	}
	defer repo.Close()

	if err := repo.Migrate(ctx); err != nil {
		log.Fatal("migrate postgres: ", err)
	}

	cache, err := NewCatalogCache(redisAddr, catalogTTL)
	if err != nil {
		log.Fatal("connect redis: ", err)
	}
	defer cache.Close()

	service := NewCatalogService(repo, cache)
	if err := service.WarmUp(ctx); err != nil {
		log.Println("warm-up:", err)
	}
	service.StartResync(ctx, resyncInterval)

	app := &App{repo: repo, cache: cache, catalog: service}

	router := gin.Default()
	router.GET("/health", app.health)
	router.GET("/clients/:clientID/catalog", app.getCatalog)
	router.POST("/clients/:clientID/products", app.createProduct)
	router.PUT("/clients/:clientID/products/:productID", app.updateProduct)
	router.DELETE("/clients/:clientID/products/:productID", app.deleteProduct)
	router.PUT("/clients/:clientID/sizes/:sizeID", app.updateSize)
	router.POST("/clients/:clientID/orders", app.createOrder)

	server := &http.Server{Addr: httpAddr, Handler: router}
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()
	log.Printf("cache-products api listening on %s", httpAddr)

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Println("shutdown:", err)
	}
}

func (a *App) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// getCatalog đọc read model từ Redis. Miss thì rebuild ngay từ Postgres rồi SET
// lại — cache tự lành, không phải chờ tới chu kỳ resync. Redis down mới là 503.
func (a *App) getCatalog(c *gin.Context) {
	ctx := c.Request.Context()
	clientID := c.Param("clientID")
	cacheKey := a.cache.Key(clientID)

	catalog, hit, err := a.cache.Get(ctx, clientID)
	if err != nil {
		log.Println("redis get:", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "cache_unavailable"})
		return
	}
	if hit {
		c.JSON(http.StatusOK, gin.H{
			"source":    "redis",
			"cache_key": cacheKey,
			"ttl":       a.cache.TTL().String(),
			"catalog":   catalog,
		})
		return
	}

	catalog, err = a.catalog.Rebuild(ctx, clientID)
	if errors.Is(err, ErrClientNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "client_not_found"})
		return
	}
	if errors.Is(err, ErrCacheUnavailable) {
		log.Println("rebuild catalog:", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "cache_unavailable"})
		return
	}
	if err != nil {
		log.Println("rebuild catalog:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "load_catalog_failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"source":    "rebuilt",
		"cache_key": cacheKey,
		"ttl":       a.cache.TTL().String(),
		"catalog":   catalog,
	})
}

func (a *App) createProduct(c *gin.Context) {
	clientID := c.Param("clientID")

	var input ProductInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
		return
	}

	product, err := a.repo.CreateProduct(c.Request.Context(), clientID, input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	a.catalog.RebuildAfterWrite(c.Request.Context(), clientID)
	c.JSON(http.StatusCreated, product)
}

func (a *App) updateProduct(c *gin.Context) {
	clientID := c.Param("clientID")
	productID, ok := parseIDParam(c, "productID")
	if !ok {
		return
	}

	var input ProductInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
		return
	}

	product, err := a.repo.UpdateProduct(c.Request.Context(), clientID, productID, input)
	if errors.Is(err, ErrProductNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	a.catalog.RebuildAfterWrite(c.Request.Context(), clientID)
	c.JSON(http.StatusOK, product)
}

func (a *App) deleteProduct(c *gin.Context) {
	clientID := c.Param("clientID")
	productID, ok := parseIDParam(c, "productID")
	if !ok {
		return
	}

	err := a.repo.DeleteProduct(c.Request.Context(), clientID, productID)
	if errors.Is(err, ErrProductNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		log.Println("delete product:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "delete_product_failed"})
		return
	}

	a.catalog.RebuildAfterWrite(c.Request.Context(), clientID)
	c.Status(http.StatusNoContent)
}

// updateSize đổi tên/giá một size mà giữ nguyên size id — dùng cho thao tác đổi
// giá thường ngày, thay vì PUT product (thay toàn bộ sizes nên id bị đổi).
func (a *App) updateSize(c *gin.Context) {
	clientID := c.Param("clientID")
	sizeID, ok := parseIDParam(c, "sizeID")
	if !ok {
		return
	}

	var input SizeInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
		return
	}

	size, err := a.repo.UpdateSize(c.Request.Context(), clientID, sizeID, input)
	if errors.Is(err, ErrSizeNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	a.catalog.RebuildAfterWrite(c.Request.Context(), clientID)
	c.JSON(http.StatusOK, size)
}

// createOrder cố tình không đọc Redis: giá tiền luôn lấy trực tiếp từ Postgres.
// Đây là ranh giới giữa query path (có cache) và command path (không cache).
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
		if item.SizeID <= 0 || item.Quantity <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "size_id and quantity must be positive"})
			return
		}

		size, err := a.repo.GetSizeForOrder(c.Request.Context(), clientID, item.SizeID)
		if errors.Is(err, ErrSizeNotFound) || (err == nil && !size.ProductActive) {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("size %d is not available", item.SizeID)})
			return
		}
		if err != nil {
			log.Println("get size:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "get_size_failed"})
			return
		}

		lineAmount := size.Price * int64(item.Quantity)
		total += lineAmount
		lines = append(lines, OrderLine{
			SizeID:     size.SizeID,
			ProductID:  size.ProductID,
			Name:       fmt.Sprintf("%s (%s)", size.ProductName, size.SizeName),
			Quantity:   item.Quantity,
			UnitPrice:  size.Price,
			LineAmount: lineAmount,
		})
	}

	c.JSON(http.StatusCreated, gin.H{
		"price_source": "db",
		"lines":        lines,
		"total":        total,
	})
}

func parseIDParam(c *gin.Context, name string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid " + name})
		return 0, false
	}
	return id, true
}
