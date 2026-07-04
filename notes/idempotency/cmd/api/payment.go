package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

// Demo 4: add-payment, pay-off, pass-order-refund là nhóm API có rủi ro
// double-charge/double-refund thật. Idempotency store ở đây dùng Redis
// (TTL native) thay vì Postgres, để so sánh trực tiếp với Demo 1/2.
//
// Redis chỉ là fast-path cache: nó có thể mất trước khi TTL hết hạn
// (restart, eviction). Vì vậy side effect thật (bảng payments) vẫn có
// UNIQUE (operation, idempotency_key) làm lưới an toàn cuối — xem
// migrations/init.sql.

const paymentIdempotencyTTL = 7 * 24 * time.Hour

type paymentRequest struct {
	OrderID              string  `json:"order_id"`
	Amount               float64 `json:"amount"`
	SimulateDelaySeconds int     `json:"simulate_delay_seconds"`
}

type paymentRecordResponse struct {
	PaymentID int64   `json:"payment_id"`
	Operation string  `json:"operation"`
	OrderID   string  `json:"order_id"`
	Amount    float64 `json:"amount"`
	Status    string  `json:"status"`
}

// idempotencyCacheRecord là toàn bộ state lưu trong 1 Redis key: request
// hash để phát hiện reuse-key-khác-payload, và response để trả lại y hệt
// khi client retry.
type idempotencyCacheRecord struct {
	RequestHash    string          `json:"request_hash"`
	Status         string          `json:"status"` // processing | succeeded | failed
	ResponseStatus int             `json:"response_status,omitempty"`
	ResponseBody   json.RawMessage `json:"response_body,omitempty"`
}

// paymentHandler dùng chung cho cả 3 operation, vì luồng idempotency và
// shape request/response giống hệt nhau — chỉ khác tên operation.
func paymentHandler(db *sql.DB, rdb *redis.Client, operation string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		if key == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing_idempotency_key"})
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
			return
		}

		var req paymentRequest
		if err := json.Unmarshal(body, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
			return
		}
		if req.OrderID == "" || req.Amount <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "validation_failed"})
			return
		}

		// Scope theo operation + order_id: retry add-payment cho order A
		// không được đụng retry pay-off cho order A dù Idempotency-Key
		// client gửi trùng ngẫu nhiên.
		scope := operation + ":order:" + req.OrderID
		requestHash := hashRequest(body)
		redisKey := "idempotency:" + scope + ":" + key

		initialRaw, err := json.Marshal(idempotencyCacheRecord{
			RequestHash: requestHash,
			Status:      "processing",
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
			return
		}

		// SETNX: chỉ request đầu tiên "thắng" và được chạy side effect.
		// Tương đương INSERT ... ON CONFLICT DO NOTHING ở Demo 1, nhưng
		// TTL do Redis quản lý native thay vì cột expires_at + job dọn dẹp.
		acquired, err := rdb.SetNX(ctx, redisKey, initialRaw, paymentIdempotencyTTL).Result()
		if err != nil {
			log.Println("redis setnx:", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
			return
		}

		if !acquired {
			waitForResult := r.URL.Query().Get("wait") == "1"
			handleExistingPaymentIdempotency(ctx, rdb, w, redisKey, requestHash, waitForResult)
			return
		}

		// time.Sleep cố tình làm cửa sổ race dài ra, giống Demo 1, để dễ
		// mở terminal khác gọi duplicate trong lúc status vẫn "processing".
		if req.SimulateDelaySeconds > 0 {
			log.Printf("simulate slow %s: sleeping %d seconds", operation, req.SimulateDelaySeconds)
			time.Sleep(time.Duration(req.SimulateDelaySeconds) * time.Second)
		}

		resp, status, err := chargeAndStorePayment(ctx, db, operation, key, req)
		if err != nil {
			log.Println("process payment:", err)
			failedRaw, _ := json.Marshal(idempotencyCacheRecord{
				RequestHash:    requestHash,
				Status:         "failed",
				ResponseStatus: http.StatusInternalServerError,
				ResponseBody:   json.RawMessage(`{"error":"internal_error"}`),
			})
			_ = rdb.Set(ctx, redisKey, failedRaw, paymentIdempotencyTTL).Err()
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
			return
		}

		respBody, err := json.Marshal(resp)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
			return
		}

		succeededRaw, err := json.Marshal(idempotencyCacheRecord{
			RequestHash:    requestHash,
			Status:         "succeeded",
			ResponseStatus: status,
			ResponseBody:   respBody,
		})
		if err == nil {
			if err := rdb.Set(ctx, redisKey, succeededRaw, paymentIdempotencyTTL).Err(); err != nil {
				// Side effect đã xong và đã ghi vào payments (nguồn sự thật).
				// Lỗi ở đây chỉ ảnh hưởng cache: retry kế tiếp có thể chạy lại
				// nhánh "processing" và dựa vào UNIQUE constraint để tự phát
				// hiện đã xử lý rồi, thay vì trả lại response cache.
				log.Println("redis cache result:", err)
			}
		}

		writeRawJSON(w, status, respBody)
	}
}

func handleExistingPaymentIdempotency(ctx context.Context, rdb *redis.Client, w http.ResponseWriter, redisKey, requestHash string, waitForResult bool) {
	deadline := time.Now().Add(8 * time.Second)

	for {
		raw, err := rdb.Get(ctx, redisKey).Result()
		if errors.Is(err, redis.Nil) {
			// Key hết TTL đúng lúc đang đợi: coi như chưa từng có, để
			// client tự quyết định có retry bằng key mới hay không.
			writeJSON(w, http.StatusGone, map[string]string{"error": "idempotency_key_expired"})
			return
		}
		if err != nil {
			log.Println("redis get:", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
			return
		}

		var record idempotencyCacheRecord
		if err := json.Unmarshal([]byte(raw), &record); err != nil {
			log.Println("unmarshal idempotency record:", err)
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
			writeRawJSON(w, record.ResponseStatus, record.ResponseBody)
			return

		case "processing":
			if !waitForResult || time.Now().After(deadline) {
				// 425 Too Early: operation đang chạy, retry sau bằng cùng
				// key là an toàn.
				writeJSON(w, http.StatusTooEarly, map[string]string{"status": "processing"})
				return
			}
			// Không phải production-grade long polling, chỉ giúp demo dễ
			// reproduce: request duplicate đợi request đầu hoàn tất.
			time.Sleep(500 * time.Millisecond)
		}
	}
}

func chargeAndStorePayment(ctx context.Context, db *sql.DB, operation, idempotencyKey string, req paymentRequest) (paymentRecordResponse, int, error) {
	var paymentID int64
	err := db.QueryRowContext(ctx, `
		INSERT INTO payments (operation, order_id, amount, idempotency_key)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`, operation, req.OrderID, req.Amount, idempotencyKey).Scan(&paymentID)

	if isUniqueViolation(err) {
		// Redis SETNX lẽ ra đã chặn duplicate từ trước; rơi vào nhánh này
		// nghĩa là cache Redis đã mất (TTL/restart) trước khi request cũ
		// kịp ghi lại kết quả. Constraint ở Postgres vẫn chặn được việc
		// tạo dòng tiền thứ hai — đây chính là double-charge/double-refund
		// nếu không có nó.
		var existingID int64
		var amount float64
		if lookupErr := db.QueryRowContext(ctx, `
			SELECT id, amount FROM payments
			WHERE operation = $1 AND idempotency_key = $2
		`, operation, idempotencyKey).Scan(&existingID, &amount); lookupErr != nil {
			return paymentRecordResponse{}, 0, lookupErr
		}

		return paymentRecordResponse{
			PaymentID: existingID,
			Operation: operation,
			OrderID:   req.OrderID,
			Amount:    amount,
			Status:    "already_processed",
		}, http.StatusOK, nil
	}
	if err != nil {
		return paymentRecordResponse{}, 0, err
	}

	return paymentRecordResponse{
		PaymentID: paymentID,
		Operation: operation,
		OrderID:   req.OrderID,
		Amount:    req.Amount,
		Status:    "succeeded",
	}, http.StatusCreated, nil
}

func isUniqueViolation(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "23505"
}
