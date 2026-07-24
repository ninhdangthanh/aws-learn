package esclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// MgetUpdatedAt — lấy field updated_at của nhiều doc trong 1 lần (_mget).
// Trả map[id]updated_at CHỈ cho doc tồn tại (doc mất tích -> không có key).
// _source của ES là verbatim -> so sánh chuỗi updated_at với Postgres là đủ để phát hiện lệch.
func (c *Client) MgetUpdatedAt(ctx context.Context, ids []string) (map[string]string, error) {
	if len(ids) == 0 {
		return map[string]string{}, nil
	}
	body, _ := json.Marshal(map[string]any{"ids": ids})
	st, b, err := c.do(ctx, http.MethodPost, "/"+c.alias+"/_mget?_source=updated_at", body)
	if err != nil {
		return nil, err
	}
	if st >= 300 {
		return nil, fmt.Errorf("mget status %d: %s", st, b)
	}
	var r struct {
		Docs []struct {
			ID     string `json:"_id"`
			Found  bool   `json:"found"`
			Source struct {
				UpdatedAt string `json:"updated_at"`
			} `json:"_source"`
		} `json:"docs"`
	}
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, err
	}
	out := make(map[string]string, len(r.Docs))
	for _, d := range r.Docs {
		if d.Found {
			out[d.ID] = d.Source.UpdatedAt
		}
	}
	return out, nil
}
