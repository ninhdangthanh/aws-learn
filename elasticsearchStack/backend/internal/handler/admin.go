package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"es-sync-backend/internal/store"
)

// list — GET /products: liệt kê từ Postgres (source of truth) cho Admin UI.
// Dùng SQL (không phải ES) để create/update/delete phản ánh NGAY, không chờ worker.
func (h *Handler) list(c *gin.Context) {
	limit := atoiDefault(c.Query("limit"), 50)
	tenant := tenantOf(c)
	ps, err := h.st.ListProducts(c, tenant, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if ps == nil {
		ps = []store.Product{}
	}
	c.JSON(http.StatusOK, gin.H{"items": ps, "count": len(ps), "tenant": tenant})
}

// esUpdatedAt — chuỗi updated_at đúng như lúc index (ES lưu _source verbatim).
func esUpdatedAt(t time.Time) string {
	b, _ := json.Marshal(t)
	return strings.Trim(string(b), `"`)
}

// reconcileDeep — đối soát SÂU (5.4d): so updated_at từng lô id (SQL là nguồn đúng).
// Phát hiện doc thiếu trong ES (missing) và doc cũ hơn Postgres (stale).
// ?fix=true -> re-index các doc lệch từ Postgres bằng _bulk (SQL luôn thắng).
func (h *Handler) reconcileDeep(c *gin.Context) {
	fix := c.Query("fix") == "true"
	const batch = 500
	var after int64
	checked, missing, stale := 0, 0, 0
	var mismatched []store.Product

	for {
		ps, err := h.st.ScanProductsAfter(c, after, batch)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if len(ps) == 0 {
			break
		}
		ids := make([]string, len(ps))
		for i, p := range ps {
			ids[i] = strconv.FormatInt(p.ID, 10)
		}
		esMap, err := h.es.MgetUpdatedAt(c, ids)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		for _, p := range ps {
			checked++
			id := strconv.FormatInt(p.ID, 10)
			esVal, ok := esMap[id]
			switch {
			case !ok:
				missing++
				mismatched = append(mismatched, p)
			case esVal != esUpdatedAt(p.UpdatedAt):
				stale++
				mismatched = append(mismatched, p)
			}
		}
		after = ps[len(ps)-1].ID
	}

	fixed := 0
	if fix && len(mismatched) > 0 {
		if err := h.es.Bulk(c, buildBulk(mismatched)); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		h.es.Refresh(c)
		fixed = len(mismatched)
	}

	c.JSON(http.StatusOK, gin.H{
		"checked":       checked,
		"missing_in_es": missing,
		"stale_in_es":   stale,
		"mismatched":    len(mismatched),
		"fixed":         fixed,
		"fix_applied":   fix,
		// in_sync = trạng thái SAU thao tác: đã fix hết coi như đồng bộ.
		"in_sync": len(mismatched) == 0 || fixed == len(mismatched),
	})
}
