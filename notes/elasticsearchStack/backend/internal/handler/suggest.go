package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// suggest — GET /suggest?q=: autocomplete (6.4).
// Dùng field name.suggest (search_as_you_type) với multi_match type=bool_prefix
// trên name.suggest + ._2gram + ._3gram -> gõ tới đâu gợi ý tới đó.
// LUÔN ép tenant filter (6.6): tenant A không gợi ý ra tên sản phẩm của tenant B.
func (h *Handler) suggest(c *gin.Context) {
	tenant := tenantOf(c)
	q := c.Query("q")
	if q == "" {
		c.JSON(http.StatusOK, gin.H{"suggestions": []string{}, "tenant": tenant})
		return
	}
	size := atoiDefault(c.Query("size"), 8)

	body := map[string]any{
		"size":    size,
		"_source": []string{"name"},
		"query": map[string]any{
			"bool": map[string]any{
				"must": []any{map[string]any{"multi_match": map[string]any{
					"query": q,
					"type":  "bool_prefix",
					"fields": []string{
						"name.suggest", "name.suggest._2gram", "name.suggest._3gram",
					},
				}}},
				"filter": []any{map[string]any{"term": map[string]any{"tenant_id": tenant}}},
			},
		},
	}

	res, err := h.esSearchParsed(c, body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	seen := map[string]bool{}
	out := make([]string, 0, len(res.Hits.Hits))
	for _, hh := range res.Hits.Hits {
		if n, _ := hh.Source["name"].(string); n != "" && !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	c.JSON(http.StatusOK, gin.H{"suggestions": out, "tenant": tenant})
}
