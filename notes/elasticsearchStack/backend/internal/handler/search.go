package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"es-sync-backend/internal/store"
)

// buildBulk — NDJSON cho _bulk backfill, _id = product id (idempotent).
func buildBulk(ps []store.Product) []byte {
	var out []byte
	for _, p := range ps {
		meta, _ := json.Marshal(map[string]any{"index": map[string]any{"_id": strconv.FormatInt(p.ID, 10)}})
		doc, _ := json.Marshal(p)
		out = append(out, meta...)
		out = append(out, '\n')
		out = append(out, doc...)
		out = append(out, '\n')
	}
	return out
}

// search — GET /search: dựng bool query từ query params rồi hỏi ES qua alias.
// KHÔNG query SQL để search (ES là store phục vụ search).
//
// Params: q, category, status, brand, in_stock, min_price, max_price, size, from.
func (h *Handler) search(c *gin.Context) {
	var must []any
	if q := c.Query("q"); q != "" {
		must = append(must, map[string]any{
			"multi_match": map[string]any{
				"query":  q,
				"fields": []string{"name^3", "description"},
			},
		})
	} else {
		must = append(must, map[string]any{"match_all": map[string]any{}})
	}

	var filter []any
	addTerm := func(field, val string) {
		if val != "" {
			filter = append(filter, map[string]any{"term": map[string]any{field: val}})
		}
	}
	addTerm("category", c.Query("category"))
	addTerm("status", c.Query("status"))
	addTerm("brand", c.Query("brand"))
	if v := c.Query("in_stock"); v != "" {
		filter = append(filter, map[string]any{"term": map[string]any{"in_stock": v == "true"}})
	}
	if rng := priceRange(c.Query("min_price"), c.Query("max_price")); rng != nil {
		filter = append(filter, map[string]any{"range": map[string]any{"price": rng}})
	}

	size := atoiDefault(c.Query("size"), 20)
	from := atoiDefault(c.Query("from"), 0)

	body := map[string]any{
		"from":             from,
		"size":             size,
		"track_total_hits": true,
		"query": map[string]any{
			"bool": map[string]any{"must": must, "filter": filter},
		},
	}
	raw, _ := json.Marshal(body)
	res, err := h.es.Search(c, raw)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	// bóc kết quả ES thành response gọn cho FE.
	var parsed struct {
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
			Hits []struct {
				Score  float64         `json:"_score"`
				Source json.RawMessage `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.Unmarshal(res, &parsed); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	items := make([]json.RawMessage, 0, len(parsed.Hits.Hits))
	for _, hh := range parsed.Hits.Hits {
		items = append(items, hh.Source)
	}
	c.JSON(http.StatusOK, gin.H{
		"total": parsed.Hits.Total.Value,
		"size":  size,
		"from":  from,
		"items": items,
	})
}

func priceRange(min, max string) map[string]any {
	r := map[string]any{}
	if min != "" {
		if f, err := strconv.ParseFloat(min, 64); err == nil {
			r["gte"] = f
		}
	}
	if max != "" {
		if f, err := strconv.ParseFloat(max, 64); err == nil {
			r["lte"] = f
		}
	}
	if len(r) == 0 {
		return nil
	}
	return r
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	if n, err := strconv.Atoi(s); err == nil && n >= 0 {
		return n
	}
	return def
}
