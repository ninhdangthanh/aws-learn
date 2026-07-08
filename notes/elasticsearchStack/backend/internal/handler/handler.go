// Package handler — REST API: /search (đọc ES) + CRUD products (ghi Postgres) + /admin ops.
package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"es-sync-backend/internal/esclient"
	"es-sync-backend/internal/store"
)

type Handler struct {
	st        *store.Store
	es        *esclient.Client
	writeMode string // "outbox" | "dual"
}

func New(st *store.Store, es *esclient.Client, writeMode string) *Handler {
	return &Handler{st: st, es: es, writeMode: writeMode}
}

func (h *Handler) useOutbox() bool { return h.writeMode != "dual" }

func (h *Handler) Register(r *gin.Engine) {
	r.GET("/healthz", h.health)
	r.GET("/search", h.search)
	r.GET("/products", h.list)
	r.POST("/products", h.create)
	r.PUT("/products/:id", h.update)
	r.DELETE("/products/:id", h.delete)
	r.POST("/admin/backfill", h.backfill)
	r.GET("/admin/reconcile", h.reconcile)
	r.GET("/admin/reconcile/deep", h.reconcileDeep)
	r.POST("/admin/reindex", h.reindex)
}

func (h *Handler) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "write_mode": h.writeMode})
}

// ---- CRUD (ghi Postgres = source of truth) ----

type productInput struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	SKU         string  `json:"sku"`
	Status      string  `json:"status"`
	Category    string  `json:"category"`
	Brand       string  `json:"brand"`
	Price       float64 `json:"price"`
	InStock     bool    `json:"in_stock"`
}

func (in productInput) toProduct() store.Product {
	return store.Product{
		Name: in.Name, Description: in.Description, SKU: in.SKU, Status: in.Status,
		Category: in.Category, Brand: in.Brand, Price: in.Price, InStock: in.InStock,
	}
}

func (h *Handler) create(c *gin.Context) {
	var in productInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if in.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	p, err := h.st.CreateProduct(c, in.toProduct(), h.useOutbox())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.dualWriteES(c, p, false) // no-op ở outbox mode
	c.JSON(http.StatusCreated, p)
}

func (h *Handler) update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id không hợp lệ"})
		return
	}
	var in productInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	p := in.toProduct()
	p.ID = id
	updated, err := h.st.UpdateProduct(c, p, h.useOutbox())
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "product không tồn tại"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.dualWriteES(c, updated, false)
	c.JSON(http.StatusOK, updated)
}

func (h *Handler) delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id không hợp lệ"})
		return
	}
	if err := h.st.DeleteProduct(c, id, h.useOutbox()); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "product không tồn tại"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.dualWriteES(c, store.Product{ID: id}, true)
	c.Status(http.StatusNoContent)
}

// dualWriteES — CHỈ chạy ở mode "dual": ghi thẳng ES ngay trong request.
// Nếu ES down -> log lỗi nhưng vẫn trả thành công -> DỮ LIỆU LỆCH (đây là bài học 5.2).
func (h *Handler) dualWriteES(c *gin.Context, p store.Product, isDelete bool) {
	if h.useOutbox() {
		return
	}
	var err error
	if isDelete {
		err = h.es.DeleteDoc(c, strconv.FormatInt(p.ID, 10), p.Version())
	} else {
		doc, _ := json.Marshal(p)
		err = h.es.IndexDoc(c, strconv.FormatInt(p.ID, 10), doc, p.Version())
	}
	if err != nil {
		// KHÔNG rollback Postgres -> lệch. Chỉ ghi log để quan sát.
		c.Header("X-Dual-Write-Drift", "true")
	}
}

// ---- Admin ----

// backfill — nạp toàn bộ products hiện có sang ES bằng _bulk, keyset theo id (idempotent).
func (h *Handler) backfill(c *gin.Context) {
	const batch = 500
	var after, total int64
	for {
		ps, err := h.st.ScanProductsAfter(c, after, batch)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if len(ps) == 0 {
			break
		}
		nd := buildBulk(ps)
		if err := h.es.Bulk(c, nd); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		total += int64(len(ps))
		after = ps[len(ps)-1].ID
	}
	h.es.Refresh(c)
	c.JSON(http.StatusOK, gin.H{"backfilled": total})
}

// reconcile — đối soát số lượng: COUNT(*) Postgres vs _count ES.
func (h *Handler) reconcile(c *gin.Context) {
	pgCount, err := h.st.CountProducts(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	pending, _ := h.st.PendingOutbox(c)
	resp := gin.H{"postgres": pgCount, "outbox_pending": pending}

	// ES có thể đang down (chính là kịch bản demo) -> degrade, đừng 500.
	esCount, esErr := h.es.Count(c)
	if esErr != nil {
		resp["elasticsearch"] = "unreachable"
		resp["es_error"] = esErr.Error()
		resp["in_sync"] = false
		c.JSON(http.StatusOK, resp)
		return
	}
	resp["elasticsearch"] = esCount
	resp["drift"] = pgCount - esCount
	resp["in_sync"] = pgCount == esCount && pending == 0
	c.JSON(http.StatusOK, resp)
}

// reindex — zero-downtime reindex qua alias (Phase 5.5).
func (h *Handler) reindex(c *gin.Context) {
	newIdx, err := h.es.ReindexSwap(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"reindexed_into": newIdx, "alias": h.es.Alias()})
}
