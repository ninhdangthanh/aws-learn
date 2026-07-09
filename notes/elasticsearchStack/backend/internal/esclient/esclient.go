// Package esclient — ES client mỏng bằng net/http (không cần SDK nặng).
// App luôn đọc/ghi qua ALIAS, không trỏ thẳng index thật (để reindex zero-downtime).
package esclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	base  string // http://localhost:9200
	alias string // "products"
	http  *http.Client
}

func New(base, alias string) *Client {
	return &Client{base: base, alias: alias, http: &http.Client{Timeout: 10 * time.Second}}
}

func (c *Client) Alias() string { return c.alias }

// do gửi request, trả body + status. Không coi 4xx/5xx là error ở đây —
// caller tự quyết (ví dụ 409 external-version là chuyện bình thường).
func (c *Client) do(ctx context.Context, method, path string, body []byte) (int, []byte, error) {
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, r)
	if err != nil {
		return 0, nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, err // ES down -> lỗi transport, worker sẽ retry
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b, nil
}

// Ping — ES còn sống không.
func (c *Client) Ping(ctx context.Context) error {
	st, _, err := c.do(ctx, http.MethodGet, "/", nil)
	if err != nil {
		return err
	}
	if st != 200 {
		return fmt.Errorf("es ping status %d", st)
	}
	return nil
}

// EnsureIndexAlias — bootstrap: đảm bảo alias trỏ tới 1 index có mapping.
// Nếu tồn tại index THẬT trùng tên alias (vd index practice từ Phase 2-4) -> xóa,
// rồi tạo {alias}_v1 + gắn alias. Idempotent.
func (c *Client) EnsureIndexAlias(ctx context.Context) error {
	// alias đã tồn tại?
	if st, _, _ := c.do(ctx, http.MethodHead, "/_alias/"+c.alias, nil); st == 200 {
		return nil
	}
	// có index thật trùng tên alias không?
	if st, _, _ := c.do(ctx, http.MethodHead, "/"+c.alias, nil); st == 200 {
		if st, b, err := c.do(ctx, http.MethodDelete, "/"+c.alias, nil); err != nil || st >= 300 {
			return fmt.Errorf("xóa index cũ %q thất bại (%d): %s", c.alias, st, b)
		}
	}
	v1 := c.alias + "_v1"
	if st, b, err := c.do(ctx, http.MethodPut, "/"+v1, mappingBody()); err != nil || st >= 300 {
		return fmt.Errorf("tạo %s thất bại (%d): %s", v1, st, b)
	}
	actions := fmt.Sprintf(`{"actions":[{"add":{"index":%q,"alias":%q}}]}`, v1, c.alias)
	if st, b, err := c.do(ctx, http.MethodPost, "/_aliases", []byte(actions)); err != nil || st >= 300 {
		return fmt.Errorf("gắn alias thất bại (%d): %s", st, b)
	}
	return nil
}

// IndexDoc — PUT doc qua alias với EXTERNAL VERSION.
// version = updated_at epoch millis: ES chỉ ghi khi version gửi lên > version hiện tại.
// 409 = đã có bản mới hơn (stale write) -> coi là OK, bỏ qua an toàn.
func (c *Client) IndexDoc(ctx context.Context, id string, doc []byte, version int64) error {
	path := fmt.Sprintf("/%s/_doc/%s?version=%d&version_type=external", c.alias, id, version)
	st, b, err := c.do(ctx, http.MethodPut, path, doc)
	if err != nil {
		return err
	}
	if st == 409 {
		return nil // bản mới hơn đã thắng
	}
	if st >= 300 {
		return fmt.Errorf("index %s status %d: %s", id, st, b)
	}
	return nil
}

// DeleteDoc — xóa qua alias, cũng dùng external version để không "sống lại" bản cũ.
func (c *Client) DeleteDoc(ctx context.Context, id string, version int64) error {
	path := fmt.Sprintf("/%s/_doc/%s?version=%d&version_type=external", c.alias, id, version)
	st, b, err := c.do(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	if st == 404 || st == 409 {
		return nil // đã không còn / đã có bản mới hơn
	}
	if st >= 300 {
		return fmt.Errorf("delete %s status %d: %s", id, st, b)
	}
	return nil
}

// Search — proxy query DSL thô, trả raw JSON để handler bóc.
func (c *Client) Search(ctx context.Context, query []byte) ([]byte, error) {
	st, b, err := c.do(ctx, http.MethodPost, "/"+c.alias+"/_search", query)
	if err != nil {
		return nil, err
	}
	if st >= 300 {
		return nil, fmt.Errorf("search status %d: %s", st, b)
	}
	return b, nil
}

// Count — số document trong alias.
func (c *Client) Count(ctx context.Context) (int64, error) {
	st, b, err := c.do(ctx, http.MethodGet, "/"+c.alias+"/_count", nil)
	if err != nil {
		return 0, err
	}
	if st >= 300 {
		return 0, fmt.Errorf("count status %d: %s", st, b)
	}
	var r struct {
		Count int64 `json:"count"`
	}
	if err := json.Unmarshal(b, &r); err != nil {
		return 0, err
	}
	return r.Count, nil
}

// Bulk — index cả lô NDJSON (dùng cho backfill). Trả lỗi nếu ES báo errors.
func (c *Client) Bulk(ctx context.Context, ndjson []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/"+c.alias+"/_bulk", bytes.NewReader(ndjson))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-ndjson")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("bulk status %d: %s", resp.StatusCode, b)
	}
	var r struct {
		Errors bool `json:"errors"`
	}
	_ = json.Unmarshal(b, &r)
	if r.Errors {
		return fmt.Errorf("bulk có lỗi: %s", truncate(b, 500))
	}
	return nil
}

// Refresh — ép ES nhìn thấy doc ngay (test/backfill). Production hiếm khi cần.
func (c *Client) Refresh(ctx context.Context) {
	_, _, _ = c.do(ctx, http.MethodPost, "/"+c.alias+"/_refresh", nil)
}

func truncate(b []byte, n int) string {
	if len(b) > n {
		return string(b[:n])
	}
	return string(b)
}
