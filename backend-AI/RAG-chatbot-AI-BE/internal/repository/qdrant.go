package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type Qdrant struct {
	client     *http.Client
	baseURL    string
	collection string
}

type Point struct {
	ID      string                 `json:"id"`
	Vector  []float32              `json:"vector"`
	Payload map[string]interface{} `json:"payload,omitempty"`
}

type searchResultItem struct {
	ID      string                 `json:"id"`
	Score   float32                `json:"score"`
	Payload map[string]interface{} `json:"payload,omitempty"`
}

type searchResponse struct {
	Result []searchResultItem `json:"result"`
}

type SearchResult struct {
	ID      string
	Score   float32
	Payload map[string]interface{}
}

func NewQdrant(addr, collection string) (*Qdrant, error) {
	base := strings.TrimSuffix(addr, "/")
	if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
		base = "http://" + base
	}
	return &Qdrant{client: http.DefaultClient, baseURL: base, collection: collection}, nil
}

func (q *Qdrant) InitCollection(ctx context.Context, vectorSize int32) error {
	body := map[string]interface{}{
		"name": q.collection,
		"vectors": map[string]interface{}{
			"size":     vectorSize,
			"distance": "Cosine",
		},
	}
	resp, err := q.doRequest(ctx, http.MethodPut, "/collections/"+q.collection, body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 && resp.StatusCode != http.StatusConflict {
		return fmt.Errorf("qdrant init collection failed: %s", resp.Status)
	}
	return nil
}

func (q *Qdrant) UpsertPoints(ctx context.Context, points []Point) error {
	body := map[string]interface{}{"points": points}
	resp, err := q.doRequest(ctx, http.MethodPut, fmt.Sprintf("/collections/%s/points?wait=true", q.collection), body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("qdrant upsert failed: %s", resp.Status)
	}
	return nil
}

func (q *Qdrant) Search(ctx context.Context, vector []float32, topK int, scoreThreshold float64) ([]SearchResult, error) {
	body := map[string]interface{}{
		"vector":       vector,
		"top":          topK,
		"with_payload": true,
	}
	if scoreThreshold > 0 {
		body["score_threshold"] = scoreThreshold
	}
	resp, err := q.doRequest(ctx, http.MethodPost, fmt.Sprintf("/collections/%s/points/search", q.collection), body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("qdrant search failed: %s", resp.Status)
	}
	var result searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	out := make([]SearchResult, 0, len(result.Result))
	for _, item := range result.Result {
		out = append(out, SearchResult{ID: item.ID, Score: item.Score, Payload: item.Payload})
	}
	return out, nil
}

func (q *Qdrant) DeletePoints(ctx context.Context, ids []string) error {
	body := map[string]interface{}{"ids": ids}
	resp, err := q.doRequest(ctx, http.MethodPost, fmt.Sprintf("/collections/%s/points/delete", q.collection), body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("qdrant delete failed: %s", resp.Status)
	}
	return nil
}

func (q *Qdrant) CollectionExists(ctx context.Context) (bool, error) {
	resp, err := q.doRequest(ctx, http.MethodGet, fmt.Sprintf("/collections/%s", q.collection), nil)
	if err != nil {
		return false, err
	}
	if resp.StatusCode == http.StatusOK {
		return true, nil
	}
	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	return false, fmt.Errorf("qdrant collection check failed: %s", resp.Status)
}

func (q *Qdrant) doRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	var payload *bytes.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		payload = bytes.NewReader(data)
	} else {
		payload = bytes.NewReader([]byte{})
	}
	req, err := http.NewRequestWithContext(ctx, method, q.baseURL+path, payload)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return q.client.Do(req)
}
