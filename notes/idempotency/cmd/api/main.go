package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

type createOrderRequest struct {
	UserID               string `json:"user_id"`
	SKU                  string `json:"sku"`
	Quantity             int    `json:"quantity"`
	SimulateDelaySeconds int    `json:"simulate_delay_seconds"`
}

type orderResponse struct {
	OrderID int64  `json:"order_id"`
	UserID  string `json:"user_id"`
	SKU     string `json:"sku"`
	Status  string `json:"status"`
}

type idempotencyRecord struct {
	RequestHash    string
	Status         string
	ResponseStatus sql.NullInt64
	ResponseBody   sql.NullString
}

func main() {
	dsn := getenv("POSTGRES_DSN", "postgres://app:app@localhost:5432/idempotency_lab?sslmode=disable")

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /orders", createOrderHandler(db))
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	log.Println("api listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func createOrderHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		if key == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "missing_idempotency_key",
			})
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
			return
		}

		var req createOrderRequest
		if err := json.Unmarshal(body, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
			return
		}
		if req.UserID == "" || req.SKU == "" || req.Quantity <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "validation_failed"})
			return
		}

		// Scope key theo operation + user.
		// Trong production có thể là tenant_id:user_id:operation_name.
		// Nếu không scope, user khác nhau dùng cùng key ngẫu nhiên có thể conflict nhau.
		scope := "create_order:user:" + req.UserID
		requestHash := hashRequest(body)

		inserted, err := insertProcessingIdempotencyKey(ctx, db, scope, key, requestHash)
		if err != nil {
			log.Println("insert idempotency key:", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
			return
		}

		if !inserted {
			waitForResult := r.URL.Query().Get("wait") == "1"
			handleExistingIdempotencyRecord(ctx, db, w, scope, key, requestHash, waitForResult)
			return
		}

		// Chỉ request insert thành công mới được chạy side effect.
		// time.Sleep ở đây cố tình làm cửa sổ race dài ra để bạn mở terminal thứ hai
		// và gọi duplicate request trong lúc status vẫn là "processing".
		if req.SimulateDelaySeconds > 0 {
			log.Printf("simulate slow create order: sleeping %d seconds", req.SimulateDelaySeconds)
			time.Sleep(time.Duration(req.SimulateDelaySeconds) * time.Second)
		}

		resp, err := createOrderAndStoreResult(ctx, db, scope, key, req)
		if err != nil {
			log.Println("create order:", err)
			_ = markIdempotencyFailed(ctx, db, scope, key, http.StatusInternalServerError, map[string]string{
				"error": "internal_error",
			})
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
			return
		}

		writeJSON(w, http.StatusCreated, resp)
	}
}

func insertProcessingIdempotencyKey(ctx context.Context, db *sql.DB, scope, key, requestHash string) (bool, error) {
	var insertedKey string

	// Đây là pattern "insert trước, xử lý sau".
	// Không làm kiểu SELECT trước rồi INSERT sau, vì 2 request song song có thể cùng SELECT không thấy key.
	err := db.QueryRowContext(ctx, `
		INSERT INTO idempotency_keys (scope, key, request_hash, status, expires_at)
		VALUES ($1, $2, $3, 'processing', now() + interval '7 days')
		ON CONFLICT (scope, key) DO NOTHING
		RETURNING key
	`, scope, key, requestHash).Scan(&insertedKey)

	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func handleExistingIdempotencyRecord(ctx context.Context, db *sql.DB, w http.ResponseWriter, scope, key, requestHash string, waitForResult bool) {
	deadline := time.Now().Add(8 * time.Second)

	for {
		record, err := loadIdempotencyRecord(ctx, db, scope, key)
		if err != nil {
			log.Println("load idempotency key:", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
			return
		}

		if record.RequestHash != requestHash {
			writeJSON(w, http.StatusConflict, map[string]string{
				"error":   "idempotency_key_reused_with_different_payload",
				"message": "same Idempotency-Key must be retried with the same request body",
			})
			return
		}

		switch record.Status {
		case "succeeded", "failed":
			if record.ResponseStatus.Valid && record.ResponseBody.Valid {
				writeRawJSON(w, int(record.ResponseStatus.Int64), []byte(record.ResponseBody.String))
				return
			}
			writeJSON(w, http.StatusConflict, map[string]string{"error": "idempotency_record_has_no_response"})
			return

		case "processing":
			if !waitForResult || time.Now().After(deadline) {
				// 425 Too Early nói với client rằng operation đang chạy,
				// retry sau bằng cùng key là an toàn.
				writeJSON(w, http.StatusTooEarly, map[string]string{"status": "processing"})
				return
			}

			// Đây không phải production-grade long polling.
			// Nó chỉ giúp reproduce dễ hơn: request duplicate đợi request đầu hoàn tất.
			time.Sleep(500 * time.Millisecond)
		}
	}
}

func createOrderAndStoreResult(ctx context.Context, db *sql.DB, scope, key string, req createOrderRequest) (orderResponse, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return orderResponse{}, err
	}
	defer tx.Rollback()

	var orderID int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO orders (user_id, sku, quantity, idempotency_scope, idempotency_key)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, req.UserID, req.SKU, req.Quantity, scope, key).Scan(&orderID)
	if err != nil {
		return orderResponse{}, err
	}

	resp := orderResponse{
		OrderID: orderID,
		UserID:  req.UserID,
		SKU:     req.SKU,
		Status:  "created",
	}
	respBody, err := json.Marshal(resp)
	if err != nil {
		return orderResponse{}, err
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE idempotency_keys
		SET status = 'succeeded',
		    response_status = $1,
		    response_body = $2::jsonb,
		    resource_type = 'order',
		    resource_id = $3,
		    updated_at = now()
		WHERE scope = $4 AND key = $5
	`, http.StatusCreated, string(respBody), strconv.FormatInt(orderID, 10), scope, key)
	if err != nil {
		return orderResponse{}, err
	}

	return resp, tx.Commit()
}

func markIdempotencyFailed(ctx context.Context, db *sql.DB, scope, key string, status int, body any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}

	_, err = db.ExecContext(ctx, `
		UPDATE idempotency_keys
		SET status = 'failed',
		    response_status = $1,
		    response_body = $2::jsonb,
		    updated_at = now()
		WHERE scope = $3 AND key = $4
	`, status, string(raw), scope, key)
	return err
}

func loadIdempotencyRecord(ctx context.Context, db *sql.DB, scope, key string) (idempotencyRecord, error) {
	var record idempotencyRecord
	err := db.QueryRowContext(ctx, `
		SELECT request_hash, status, response_status, response_body::text
		FROM idempotency_keys
		WHERE scope = $1 AND key = $2
	`, scope, key).Scan(&record.RequestHash, &record.Status, &record.ResponseStatus, &record.ResponseBody)
	return record, err
}

func hashRequest(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	raw, err := json.Marshal(body)
	if err != nil {
		http.Error(w, "json encode failed", http.StatusInternalServerError)
		return
	}
	writeRawJSON(w, status, raw)
}

func writeRawJSON(w http.ResponseWriter, status int, raw []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(raw)
	_, _ = w.Write([]byte("\n"))
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
