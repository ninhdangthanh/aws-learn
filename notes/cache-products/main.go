package main

import (
	"context"
	"errors"
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
	// cacheTTL chỉ là cơ chế dọn dẹp. Cache chủ yếu được invalidate sau mỗi write.
	cacheTTL = time.Hour
)

type App struct {
	repo  *CatalogRepo
	cache *CatalogCache
}

func main() {
	ctx := context.Background()

	repo, err := NewCatalogRepo(postgresDSN)
	if err != nil {
		log.Fatal("connect postgres: ", err)
	}
	defer repo.Close()

	if err := repo.Migrate(ctx); err != nil {
		log.Fatal("migrate postgres: ", err)
	}

	cache, err := NewCatalogCache(redisAddr, cacheTTL)
	if err != nil {
		log.Fatal("connect redis: ", err)
	}
	defer cache.Close()

	// Không warm-up: Redis rỗng lúc start là chuyện bình thường của cache-aside,
	// request đọc đầu tiên sẽ tự dựng cache.
	app := &App{repo: repo, cache: cache}

	router := gin.Default()
	router.GET("/health", app.health)
	router.GET("/clients/:clientID/catalog", app.getCatalog)
	router.POST("/clients/:clientID/products", app.createProduct)
	router.PUT("/clients/:clientID/products/:productID", app.updateProduct)
	router.DELETE("/clients/:clientID/products/:productID", app.deleteProduct)
	router.PUT("/clients/:clientID/sizes/:sizeID", app.updateSize)
	router.POST("/clients/:clientID/orders", app.createOrder)

	log.Printf("cache-products api listening on %s", httpAddr)
	if err := router.Run(httpAddr); err != nil {
		log.Fatal(err)
	}
}

func (a *App) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// getCatalog dùng cache-aside: đọc Redis trước, miss thì build catalog từ Postgres
// rồi ghi lại cache. Một key duy nhất chứa cả categories, products và sizes.
func (a *App) getCatalog(c *gin.Context) {
	ctx := c.Request.Context()
	clientID := c.Param("clientID")
	cacheKey := a.cache.Key(clientID)

	if catalog, hit := a.cache.Get(ctx, clientID); hit {
		c.JSON(http.StatusOK, gin.H{
			"source":    "cache",
			"cache_key": cacheKey,
			"catalog":   catalog,
		})
		return
	}

	catalog, err := a.repo.LoadCatalog(ctx, clientID)
	if errors.Is(err, ErrClientNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "client_not_found"})
		return
	}
	if err != nil {
		log.Println("load catalog:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "load_catalog_failed"})
		return
	}

	a.cache.Set(ctx, clientID, catalog)

	c.JSON(http.StatusOK, gin.H{
		"source":    "db",
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

	a.cache.Del(c.Request.Context(), clientID)
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

	a.cache.Del(c.Request.Context(), clientID)
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

	a.cache.Del(c.Request.Context(), clientID)
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

	a.cache.Del(c.Request.Context(), clientID)
	c.JSON(http.StatusOK, size)
}

func parseIDParam(c *gin.Context, name string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid " + name})
		return 0, false
	}
	return id, true
}
