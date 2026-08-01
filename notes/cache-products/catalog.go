package main

import (
	"context"
	"fmt"
	"log"
	"time"
)

// CatalogService giữ read model của Redis đồng bộ với Postgres qua ba cơ chế:
//
//	Rebuild  — sau mỗi write, chỉ rebuild đúng client bị ảnh hưởng
//	WarmUp   — lúc app start, dựng lại toàn bộ client
//	Resync   — mỗi 5 phút lặp lại WarmUp để tự hồi phục
type CatalogService struct {
	repo  *CatalogRepo
	cache *CatalogCache
}

func NewCatalogService(repo *CatalogRepo, cache *CatalogCache) *CatalogService {
	return &CatalogService{repo: repo, cache: cache}
}

// Rebuild đọc catalog từ Postgres rồi ghi đè key Redis của client đó.
// Lỗi ghi Redis được bọc thành ErrCacheUnavailable để handler trả 503.
func (s *CatalogService) Rebuild(ctx context.Context, clientID string) (Catalog, error) {
	catalog, err := s.repo.LoadCatalog(ctx, clientID)
	if err != nil {
		return Catalog{}, err
	}
	if err := s.cache.Set(ctx, clientID, catalog); err != nil {
		return Catalog{}, fmt.Errorf("%w: %v", ErrCacheUnavailable, err)
	}
	return catalog, nil
}

// RebuildAfterWrite dùng sau khi commit DB. Rebuild fail chỉ log chứ không làm
// hỏng response — dữ liệu đã nằm trong source of truth, resync sẽ chữa cache.
func (s *CatalogService) RebuildAfterWrite(ctx context.Context, clientID string) {
	if _, err := s.Rebuild(ctx, clientID); err != nil {
		log.Printf("rebuild catalog client=%s: %v (resync sẽ chữa)", clientID, err)
	}
}

// WarmUp dựng lại catalog cho mọi client. Một client fail thì log và đi tiếp,
// không chặn app start.
func (s *CatalogService) WarmUp(ctx context.Context) error {
	clientIDs, err := s.repo.ListClientIDs(ctx)
	if err != nil {
		return fmt.Errorf("list clients: %w", err)
	}

	start := time.Now()
	ok := 0
	for _, clientID := range clientIDs {
		if _, err := s.Rebuild(ctx, clientID); err != nil {
			log.Printf("warm-up client=%s: %v", clientID, err)
			continue
		}
		ok++
	}
	log.Printf("catalog warm-up: %d/%d client trong %s", ok, len(clientIDs), time.Since(start).Round(time.Millisecond))
	return nil
}

// StartResync chạy nền, overwrite toàn bộ read model theo chu kỳ. Đây mới là cơ
// chế đồng bộ chính (TTL chỉ là safety net): Redis restart, eviction, hoặc ai đó
// sửa thẳng dưới DB đều tự hồi phục sau tối đa một chu kỳ.
func (s *CatalogService) StartResync(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				log.Println("catalog resync stopped")
				return
			case <-ticker.C:
				if err := s.WarmUp(ctx); err != nil {
					log.Println("catalog resync:", err)
				}
			}
		}
	}()
}
