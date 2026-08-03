package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// createOrder cố tình không đọc Redis: giá tiền luôn lấy trực tiếp từ Postgres.
// Đây là ranh giới giữa query path (có cache) và command path (không cache).
//
// Chạy ba pha:
//  1. đọc toàn bộ size từ DB, chặn món không bán được
//  2. so giá client đang hiển thị với giá DB, lệch thì 409 và không tạo order
//  3. tính tiền theo giá DB
func (a *App) createOrder(c *gin.Context) {
	ctx := c.Request.Context()
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

	// Pha 1 — sizes[i] luôn khớp với req.Items[i] vì vòng lặp return ngay khi lỗi.
	sizes := make([]OrderSize, 0, len(req.Items))
	for _, item := range req.Items {
		if item.SizeID <= 0 || item.Quantity <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "size_id and quantity must be positive"})
			return
		}

		size, err := a.repo.GetSizeForOrder(ctx, clientID, item.SizeID)
		if errors.Is(err, ErrSizeNotFound) || (err == nil && !size.ProductActive) {
			// Món đã bị tắt cũng là dấu hiệu client đang cầm menu cũ — kiểm tra cache luôn.
			refreshed := false
			if err == nil {
				refreshed = a.reconcileStaleCache(ctx, clientID, []OrderSize{size})
			}
			c.JSON(http.StatusBadRequest, gin.H{
				"error":             fmt.Sprintf("size %d is not available", item.SizeID),
				"catalog_refreshed": refreshed,
			})
			return
		}
		if err != nil {
			log.Println("get size:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "get_size_failed"})
			return
		}
		sizes = append(sizes, size)
	}

	// Pha 2 — gom hết dòng lệch giá vào một response để client sửa một lần.
	var changes []PriceChange
	var newTotal int64
	for i, item := range req.Items {
		size := sizes[i]
		newTotal += size.Price * int64(item.Quantity)

		if item.ExpectedPrice > 0 && item.ExpectedPrice != size.Price {
			changes = append(changes, PriceChange{
				SizeID:        size.SizeID,
				Name:          fmt.Sprintf("%s (%s)", size.ProductName, size.SizeName),
				ExpectedPrice: item.ExpectedPrice,
				CurrentPrice:  size.Price,
			})
		}
	}
	if len(changes) > 0 {
		refreshed := a.reconcileStaleCache(ctx, clientID, sizes)
		c.JSON(http.StatusConflict, gin.H{
			"error":             "price_changed",
			"message":           "Giá đã thay đổi, vui lòng tải lại menu và xác nhận lại",
			"changed_items":     changes,
			"new_total":         newTotal,
			"catalog_refreshed": refreshed,
		})
		return
	}

	// Pha 3
	lines := make([]OrderLine, 0, len(req.Items))
	var total int64
	for i, item := range req.Items {
		size := sizes[i]
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

// reconcileStaleCache xoá catalog cache khi nó thật sự lệch so với DB, trả về
// true nếu đã xoá.
//
// Quan trọng: giá lệch KHÔNG đồng nghĩa cache sai. Khách mở menu một tiếng trước
// rồi mới bấm đặt thì cache vẫn có thể đang đúng, chỉ có browser của khách là cũ.
// Nếu cứ hễ lệch là DEL thì một client cũ đủ sức làm cache bị xoá liên tục, mọi
// request đọc sau đó đều miss vô ích. Nên phải so cache với DB trước.
func (a *App) reconcileStaleCache(ctx context.Context, clientID string, dbSizes []OrderSize) bool {
	cached, hit := a.cache.Get(ctx, clientID)
	if !hit {
		return false // không có cache thì không có gì để chữa
	}

	for _, size := range dbSizes {
		cachedPrice, found := cached.SizePrice(size.SizeID)

		stale := ""
		switch {
		case found && !size.ProductActive:
			stale = "món đã tắt nhưng cache vẫn còn"
		case found && cachedPrice != size.Price:
			stale = fmt.Sprintf("giá cache %d != DB %d", cachedPrice, size.Price)
		case !found && size.ProductActive:
			stale = "món đang bán nhưng cache không có"
		}
		if stale == "" {
			continue
		}

		log.Printf("catalog cache lệch DB (client=%s size=%d: %s) -> xoá key", clientID, size.SizeID, stale)
		a.cache.Del(ctx, clientID)
		return true
	}
	return false
}
