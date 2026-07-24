package handler

import (
	"encoding/json"
	"html"
	"net/http"
	"strconv"
	"strings"

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

// --- Phase 6 hằng số dùng cho search ---

// hlStart/hlEnd — sentinel (Unicode private-use) làm pre/post tag của highlight.
// Không xuất hiện trong text sản phẩm và KHÔNG bị html.EscapeString đụng tới,
// nên ta escape HTML nguyên đoạn rồi mới thay sentinel bằng <em> (6.1 — chống XSS).
const (
	hlStart = "\ue000"
	hlEnd   = "\ue001"
)

// sourceFields — _source filtering (6.7): list search chỉ trả field gọn,
// bỏ description (nặng, dùng highlight thay), updated_at, tenant_id.
var sourceFields = []string{"id", "name", "price", "brand", "category", "status", "in_stock", "sku", "created_at"}

// stableSort — sort ổn định cho search_after (6.2): _score desc + tie-breaker id asc.
var stableSort = []any{
	map[string]any{"_score": "desc"},
	map[string]any{"id": "asc"},
}

// facetAggs — facet brand + category bằng terms agg (6.3).
var facetAggs = map[string]any{
	"brand":    map[string]any{"terms": map[string]any{"field": "brand", "size": 20}},
	"category": map[string]any{"terms": map[string]any{"field": "category", "size": 20}},
}

// highlightSpec — highlight name + description với sentinel tag (6.1).
var highlightSpec = map[string]any{
	"pre_tags":  []string{hlStart},
	"post_tags": []string{hlEnd},
	"fields": map[string]any{
		"name":        map[string]any{"number_of_fragments": 0}, // cả field (tiêu đề ngắn)
		"description": map[string]any{"fragment_size": 140, "number_of_fragments": 1},
	},
}

// esResponse — phần response ES mà handler cần bóc.
type esResponse struct {
	Hits struct {
		Total struct {
			Value    int64  `json:"value"`
			Relation string `json:"relation"` // "eq" (chính xác) | "gte" (bị cap track_total_hits)
		} `json:"total"`
		Hits []struct {
			Score     float64             `json:"_score"`
			Source    map[string]any      `json:"_source"`
			Highlight map[string][]string `json:"highlight"`
			Sort      []any               `json:"sort"` // giá trị sort -> search_after trang sau
		} `json:"hits"`
	} `json:"hits"`
	Aggregations struct {
		Brand    esTermsAgg `json:"brand"`
		Category esTermsAgg `json:"category"`
	} `json:"aggregations"`
	Suggest struct {
		DidYouMean []struct {
			Text    string `json:"text"`
			Options []struct {
				Text string `json:"text"`
			} `json:"options"`
		} `json:"did_you_mean"`
	} `json:"suggest"`
}

type esTermsAgg struct {
	Buckets []struct {
		Key   string `json:"key"`
		Count int64  `json:"doc_count"`
	} `json:"buckets"`
}

// search — GET /search: search production-grade (Phase 6).
//
//	q, category, status, brand, in_stock, min_price, max_price, size, from,
//	search_after (JSON array), track_total_hits.
//
// Tính năng: highlight (6.1), track_total_hits + search_after (6.2),
// facet brand/category qua post_filter (6.3), synonym qua search_analyzer (6.4),
// zero-result fallback fuzzy -> did-you-mean (6.5), ÉP tenant filter (6.6), _source trim (6.7).
func (h *Handler) search(c *gin.Context) {
	tenant := tenantOf(c) // 6.6 — lấy từ context (header), KHÔNG từ body/query
	q := c.Query("q")
	brand := c.Query("brand")

	size := atoiDefault(c.Query("size"), 20)
	from := atoiDefault(c.Query("from"), 0)
	searchAfter := parseSearchAfter(c.Query("search_after"))

	filter := commonFilters(c, tenant)

	// build — dựng body search với must clause cho trước.
	build := func(must []any, withSuggest bool) map[string]any {
		body := map[string]any{
			"size":             size,
			"track_total_hits": trackTotal(c.Query("track_total_hits")),
			"_source":          sourceFields,
			"sort":             stableSort,
			"aggs":             facetAggs,
			"query": map[string]any{
				"bool": map[string]any{"must": must, "filter": filter},
			},
		}
		// Pagination: search_after (deep, ổn định) ưu tiên hơn from (6.2).
		if searchAfter != nil {
			body["search_after"] = searchAfter
		} else {
			body["from"] = from
		}
		// post_filter brand: hits chỉ brand đã chọn NHƯNG facet brand vẫn đủ mọi brand (6.3).
		if brand != "" {
			body["post_filter"] = map[string]any{"term": map[string]any{"brand": brand}}
		}
		if q != "" {
			body["highlight"] = highlightSpec
		}
		if withSuggest && q != "" {
			body["suggest"] = map[string]any{
				"did_you_mean": map[string]any{
					"text": q,
					"term": map[string]any{"field": "name", "suggest_mode": "always"},
				},
			}
		}
		return body
	}

	// 1) Query chính: operator=and (chặt).
	res, err := h.esSearchParsed(c, build(mustAnd(q), false))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	fallback := "none"
	didYouMean := ""

	// 2) Zero-result fallback (6.5): còn từ khóa mà 0 hit -> nới lỏng bằng fuzzy + or.
	if q != "" && res.Hits.Total.Value == 0 {
		if r2, err := h.esSearchParsed(c, build(mustFuzzy(q), false)); err == nil {
			res = r2
			fallback = "fuzzy"
		}
		// 3) Vẫn 0 -> gợi ý "did you mean" + danh sách phổ biến (match_all).
		if res.Hits.Total.Value == 0 {
			if r3, err := h.esSearchParsed(c, build(mustAll(), true)); err == nil {
				res = r3
				fallback = "suggest"
				didYouMean = extractDidYouMean(r3, q)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"total":             res.Hits.Total.Value,
		"total_relation":    res.Hits.Total.Relation, // "eq"=chính xác, "gte"=bị cap
		"size":              size,
		"from":              from,
		"items":             buildItems(res),
		"facets":            gin.H{"brand": buckets(res.Aggregations.Brand), "category": buckets(res.Aggregations.Category)},
		"fallback":          fallback,
		"did_you_mean":      didYouMean,
		"next_search_after": lastSort(res),
		"tenant":            tenant,
	})
}

// esSearchParsed — chạy 1 search body, parse ra esResponse.
func (h *Handler) esSearchParsed(c *gin.Context, body map[string]any) (esResponse, error) {
	raw, _ := json.Marshal(body)
	b, err := h.es.Search(c, raw)
	if err != nil {
		return esResponse{}, err
	}
	var parsed esResponse
	if err := json.Unmarshal(b, &parsed); err != nil {
		return esResponse{}, err
	}
	return parsed, nil
}

// buildItems — gộp _source + highlight (đã escape HTML) thành item cho FE.
func buildItems(res esResponse) []map[string]any {
	items := make([]map[string]any, 0, len(res.Hits.Hits))
	for _, hh := range res.Hits.Hits {
		item := hh.Source
		if item == nil {
			item = map[string]any{}
		}
		if len(hh.Highlight) > 0 {
			hl := make(map[string]string, len(hh.Highlight))
			for field, frags := range hh.Highlight {
				if len(frags) > 0 {
					hl[field] = escapeHighlight(frags[0])
				}
			}
			item["highlight"] = hl
		}
		items = append(items, item)
	}
	return items
}

// escapeHighlight — escape HTML nguyên đoạn (6.1) rồi thay sentinel bằng <em>.
// text gốc bị vô hiệu hóa (<script> -> &lt;script&gt;), chỉ tag <em> của ta sống sót.
func escapeHighlight(frag string) string {
	esc := html.EscapeString(frag)
	esc = strings.ReplaceAll(esc, hlStart, "<em>")
	esc = strings.ReplaceAll(esc, hlEnd, "</em>")
	return esc
}

// buckets — biến terms agg thành list {key,count} cho FE.
func buckets(a esTermsAgg) []gin.H {
	out := make([]gin.H, 0, len(a.Buckets))
	for _, b := range a.Buckets {
		out = append(out, gin.H{"key": b.Key, "count": b.Count})
	}
	return out
}

// lastSort — sort values của hit cuối -> search_after cho trang kế (6.2).
func lastSort(res esResponse) []any {
	if n := len(res.Hits.Hits); n > 0 {
		return res.Hits.Hits[n-1].Sort
	}
	return nil
}

// extractDidYouMean — ghép gợi ý sửa lỗi từ term suggester; "" nếu không đổi gì.
func extractDidYouMean(res esResponse, q string) string {
	if len(res.Suggest.DidYouMean) == 0 {
		return ""
	}
	var parts []string
	for _, tok := range res.Suggest.DidYouMean {
		if len(tok.Options) > 0 {
			parts = append(parts, tok.Options[0].Text)
		} else {
			parts = append(parts, tok.Text)
		}
	}
	sug := strings.Join(parts, " ")
	if strings.EqualFold(sug, q) {
		return ""
	}
	return sug
}

// commonFilters — filter áp cho cả hits lẫn facet count. LUÔN ép tenant (6.6).
// brand KHÔNG ở đây — nó là post_filter để giữ count các brand khác (6.3).
func commonFilters(c *gin.Context, tenant string) []any {
	filter := []any{map[string]any{"term": map[string]any{"tenant_id": tenant}}}
	add := func(field, val string) {
		if val != "" {
			filter = append(filter, map[string]any{"term": map[string]any{field: val}})
		}
	}
	add("category", c.Query("category"))
	add("status", c.Query("status"))
	if v := c.Query("in_stock"); v != "" {
		filter = append(filter, map[string]any{"term": map[string]any{"in_stock": v == "true"}})
	}
	if rng := priceRange(c.Query("min_price"), c.Query("max_price")); rng != nil {
		filter = append(filter, map[string]any{"range": map[string]any{"price": rng}})
	}
	return filter
}

// mustAnd — query chính: multi_match operator=and (mọi từ phải khớp).
func mustAnd(q string) []any {
	if q == "" {
		return mustAll()
	}
	return []any{map[string]any{"multi_match": map[string]any{
		"query": q, "fields": []string{"name^3", "description"}, "operator": "and",
	}}}
}

// mustFuzzy — nới lỏng: fuzziness AUTO (typo) + operator or + minimum_should_match (6.5).
func mustFuzzy(q string) []any {
	return []any{map[string]any{"multi_match": map[string]any{
		"query": q, "fields": []string{"name^3", "description"},
		"fuzziness": "AUTO", "operator": "or", "minimum_should_match": "2<70%",
	}}}
}

func mustAll() []any {
	return []any{map[string]any{"match_all": map[string]any{}}}
}

// trackTotal — "" | "true" -> true (đếm chính xác); "false" -> false (cap ~10000, relation gte);
// số -> track tới ngưỡng đó. Xem 6.2.
func trackTotal(s string) any {
	switch s {
	case "", "true":
		return true
	case "false":
		return false
	}
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return true
}

// parseSearchAfter — param JSON array, ví dụ [12.5,"340"]. nil nếu rỗng/lỗi.
func parseSearchAfter(s string) []any {
	if s == "" {
		return nil
	}
	var v []any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return nil
	}
	return v
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
