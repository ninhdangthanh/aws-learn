// Package outbox — worker poll bảng outbox và sync sang ES với retry (at-least-once).
package outbox

import (
	"context"
	"log"
	"time"

	"es-sync-backend/internal/esclient"
	"es-sync-backend/internal/store"
)

type Worker struct {
	st   *store.Store
	es   *esclient.Client
	poll time.Duration
}

func New(st *store.Store, es *esclient.Client, poll time.Duration) *Worker {
	return &Worker{st: st, es: es, poll: poll}
}

// Run — vòng lặp tới khi ctx hủy. Mỗi tick xử lý 1 batch outbox.
func (w *Worker) Run(ctx context.Context) {
	log.Printf("[outbox] worker started (poll=%s)", w.poll)
	t := time.NewTicker(w.poll)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Printf("[outbox] worker stopped")
			return
		case <-t.C:
			w.tick(ctx)
		}
	}
}

func (w *Worker) tick(ctx context.Context) {
	n, err := w.st.ProcessOutboxBatch(ctx, 100, w.handle)
	if err != nil {
		log.Printf("[outbox] batch error: %v", err)
		return
	}
	if n > 0 {
		log.Printf("[outbox] synced %d row(s) to ES", n)
	}
}

// handle — thực thi 1 outbox row lên ES. Lỗi transport (ES down) trả về non-nil
// -> store KHÔNG mark processed -> retry vòng sau. 409 stale đã coi là OK trong esclient.
func (w *Worker) handle(r store.OutboxRow) error {
	ctx := context.Background()
	switch r.Op {
	case "index":
		return w.es.IndexDoc(ctx, r.AggregateID, r.Payload, r.Version)
	case "delete":
		return w.es.DeleteDoc(ctx, r.AggregateID, r.Version)
	default:
		log.Printf("[outbox] op lạ %q, bỏ qua", r.Op)
		return nil // op không hợp lệ -> mark processed để không kẹt
	}
}
