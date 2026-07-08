package esclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// mappingBody — mapping tường minh cho products (backend làm chủ mapping).
func mappingBody() []byte {
	return []byte(`{
	  "settings": { "number_of_shards": 1, "number_of_replicas": 0 },
	  "mappings": {
	    "properties": {
	      "id":          { "type": "long" },
	      "name":        { "type": "text", "fields": { "raw": { "type": "keyword" } } },
	      "description": { "type": "text" },
	      "sku":         { "type": "keyword" },
	      "status":      { "type": "keyword" },
	      "category":    { "type": "keyword" },
	      "brand":       { "type": "keyword" },
	      "price":       { "type": "scaled_float", "scaling_factor": 100 },
	      "in_stock":    { "type": "boolean" },
	      "created_at":  { "type": "date" },
	      "updated_at":  { "type": "date" }
	    }
	  }
	}`)
}

// ReindexSwap — reindex zero-downtime (Phase 5.5):
//  1. tạo index mới {alias}_v{ts} với mapping hiện tại
//  2. _reindex từ alias sang index mới
//  3. swap alias atomic (remove index cũ, add index mới) trong 1 lệnh
//  4. xóa (các) index cũ
//
// App không bao giờ downtime vì luôn đọc/ghi qua alias.
func (c *Client) ReindexSwap(ctx context.Context) (string, error) {
	// index nào đang đứng sau alias?
	old, err := c.indicesBehindAlias(ctx)
	if err != nil {
		return "", err
	}
	newIdx := fmt.Sprintf("%s_v%d", c.alias, time.Now().Unix())

	if st, b, err := c.do(ctx, http.MethodPut, "/"+newIdx, mappingBody()); err != nil || st >= 300 {
		return "", fmt.Errorf("tạo %s (%d): %s", newIdx, st, b)
	}

	body := fmt.Sprintf(`{"source":{"index":%q},"dest":{"index":%q,"version_type":"external"}}`, c.alias, newIdx)
	if st, b, err := c.do(ctx, http.MethodPost, "/_reindex?refresh=true", []byte(body)); err != nil || st >= 300 {
		return "", fmt.Errorf("reindex (%d): %s", st, b)
	}

	// swap alias atomic
	actions := fmt.Sprintf(`{"actions":[{"add":{"index":%q,"alias":%q}}`, newIdx, c.alias)
	for _, o := range old {
		actions += fmt.Sprintf(`,{"remove":{"index":%q,"alias":%q}}`, o, c.alias)
	}
	actions += `]}`
	if st, b, err := c.do(ctx, http.MethodPost, "/_aliases", []byte(actions)); err != nil || st >= 300 {
		return "", fmt.Errorf("swap alias (%d): %s", st, b)
	}

	// dọn index cũ
	for _, o := range old {
		_, _, _ = c.do(ctx, http.MethodDelete, "/"+o, nil)
	}
	return newIdx, nil
}

func (c *Client) indicesBehindAlias(ctx context.Context) ([]string, error) {
	st, b, err := c.do(ctx, http.MethodGet, "/_alias/"+c.alias, nil)
	if err != nil {
		return nil, err
	}
	if st >= 300 {
		return nil, fmt.Errorf("get alias (%d): %s", st, b)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(m))
	for idx := range m {
		out = append(out, idx)
	}
	return out, nil
}
