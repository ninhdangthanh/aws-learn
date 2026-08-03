# Implement Plan v2 — quay về Cache Aside

Input: `docs/new-requirement-plan-1.md`. Thay thế `docs/implement-plan.md` (bản read model).

## Bối cảnh

Bản v1 dựng Redis thành read model: warm-up lúc start, rebuild sau mỗi write, resync 5 phút.
Nay **bỏ requirement** *"lúc nào cache cũng phải có toàn bộ data của source of truth"* để đổi lấy
sự đơn giản, vì catalog mỗi client chỉ ~50 product nên rebuild lúc miss rất rẻ.

Đây là **giảm độ phức tạp**, không phải làm lại. Data model, seed, toàn bộ tầng Postgres và order
path giữ nguyên 100%. Cache key cũng giữ nguyên `client:{clientID}:catalog`.

---

## 1. Cái gì giữ nguyên

| File | Trạng thái |
| --- | --- |
| `model.go` | giữ, chỉ bỏ 1 sentinel error |
| `postgres.go` | **giữ nguyên hoàn toàn** — `LoadCatalog`, CRUD product/size, `GetSizeForOrder` |
| `seed.go` | **giữ nguyên hoàn toàn** — 2 client, 7 category, 24 product, 42 size |

Cụ thể vẫn đúng như v1:

- Giá nằm ở `product_sizes`, product không có cột `price`; món một giá có size `"Mặc định"`.
- `client_id` kiểu string (`"1001"`, `"1002"`).
- Catalog chỉ chứa **category active + product active**.
- Payload shape category → product → size không đổi.
- Order đọc thẳng Postgres theo `size_id`, không đụng Redis.
- Client không tồn tại → 404 `client_not_found`.

---

## 2. Delta cần làm

| File | Thay đổi |
| --- | --- |
| `catalog.go` | **xoá cả file** — không còn warm-up / rebuild-on-write / resync |
| `redis.go` | `Get` bỏ trả `error`, lỗi Redis coi như miss; `Set`/`Del` chỉ log |
| `main.go` | read path thành cache-aside; write path `Del` thay vì rebuild; bỏ warm-up, resync, graceful shutdown |
| `model.go` | xoá `ErrCacheUnavailable`; `Catalog.RebuiltAt` → `CachedAt` |
| `docs.md` | viết lại theo cache-aside |
| `README.md` | viết lại bộ curl test |

---

## 3. Chi tiết

### 3.1 Xoá `catalog.go`

`CatalogService` tồn tại chỉ để phục vụ ba cơ chế đồng bộ chủ động. Cache-aside không có cơ chế nào
trong đó — cache chỉ được điền bởi read miss. Phần còn lại (`LoadCatalog` + `Set`) gọn tới mức nằm
thẳng trong handler là đúng nhất, đúng như pseudocode kinh điển của pattern.

### 3.2 `redis.go` — cache là optional

Đảo ngược quyết định của v1. Ở read model, Redis down là lỗi nghiêm trọng nên trả 503. Ở cache-aside,
**cache hỏng chỉ làm chậm chứ không được làm sai** — Redis down thì đọc thẳng DB và log, request vẫn
thành công.

```go
// Get trả (catalog, true) khi hit. Redis down hoặc payload hỏng đều coi như miss:
// cache-aside luôn có DB làm đường lui.
func (c *CatalogCache) Get(ctx context.Context, clientID string) (Catalog, bool)

func (c *CatalogCache) Set(ctx context.Context, clientID string, catalog Catalog)  // lỗi chỉ log
func (c *CatalogCache) Del(ctx context.Context, clientID string)                   // lỗi chỉ log
```

`TTL` đổi 24h → **1h**.

### 3.3 `main.go` — read path

```go
func (a *App) getCatalog(c *gin.Context) {
    ctx := c.Request.Context()
    clientID := c.Param("clientID")
    cacheKey := a.cache.Key(clientID)

    if catalog, hit := a.cache.Get(ctx, clientID); hit {
        c.JSON(http.StatusOK, gin.H{"source": "cache", "cache_key": cacheKey, "catalog": catalog})
        return
    }

    catalog, err := a.repo.LoadCatalog(ctx, clientID)
    if errors.Is(err, ErrClientNotFound) { → 404 client_not_found }
    if err != nil { → 500 load_catalog_failed }

    a.cache.Set(ctx, clientID, catalog)

    c.JSON(http.StatusOK, gin.H{
        "source": "db", "cache_key": cacheKey, "ttl": a.cache.TTL().String(), "catalog": catalog,
    })
}
```

Nhãn `source` đổi `redis|rebuilt` → **`cache|db`**, đúng thuật ngữ cache-aside và khớp bản demo gốc.

### 3.4 `main.go` — write path

Cả bốn handler `createProduct` / `updateProduct` / `deleteProduct` / `updateSize` đổi:

```go
a.catalog.RebuildAfterWrite(ctx, clientID)   // v1
a.cache.Del(ctx, clientID)                   // v2
```

Không rebuild ngay. Request đọc đầu tiên sau đó tự dựng lại cache.

### 3.5 `main.go` — startup

Không warm-up, không resync. Không còn goroutine nền nên bỏ luôn `signal.NotifyContext` +
`server.Shutdown`, quay lại `router.Run(httpAddr)`. Redis rỗng lúc start là chuyện bình thường.

### 3.6 `model.go`

- Xoá `ErrCacheUnavailable` (không còn handler nào trả 503).
- `Catalog.RebuiltAt` → `CachedAt` (`json:"cached_at"`). Ở v1 field này cho biết resync chạy lần cuối
  khi nào; ở v2 nó cho biết entry cache được dựng lúc nào — hữu ích để nhìn thấy stale window.

---

## 4. Đánh đổi — nhận rõ cái mất đi

Bỏ requirement "cache luôn đầy đủ" đồng nghĩa chấp nhận 4 điều sau. Cả 4 đều chấp nhận được ở quy mô
~50 product/client, nhưng phải ghi ra để sau này biết ngưỡng nào cần đổi lại.

**a. Request đầu tiên sau mỗi write chậm.** Read ngay sau write luôn là miss → 4 câu SELECT + build
JSON + SET. Ở v1 nó luôn hit vì write đã ghi sẵn cache.

**b. Cache stampede.** N request đồng thời vào một key vừa bị `DEL` sẽ cùng `LoadCatalog` từ Postgres.
Ở v1 hiếm vì miss hiếm; ở v2 miss xảy ra sau *mỗi* lần write nên xác suất cao hơn hẳn. Fix nếu cần:
`golang.org/x/sync/singleflight` quanh nhánh miss. **Plan này không làm** — giữ đơn giản.

**c. Race "DEL trước, SET sau" — cache stale cả tiếng.** Đây là điểm yếu kinh điển của cache-aside và
là cái v1 *không* có:

```text
T1 (read)  : miss, SELECT DB xong -> đang giữ catalog giá 40k
T2 (write) : UPDATE giá 45k, commit, DEL key  (key đang trống, no-op)
T1 (read)  : SET key = catalog giá 40k, TTL 1h
=> Redis giữ giá cũ tới 1 tiếng, không có cơ chế nào tự chữa
```

Ở v1 không xảy ra vì write luôn `SET` bản mới **sau** commit. Đổi lại v1 có rebuild race
(hai write song song ghi đè nhau) — nhưng resync 5 phút chữa được, còn ở đây thì không.

Chấp nhận cho demo. Giảm nhẹ được bằng TTL ngắn, hoặc delayed double-delete, hoặc versioned key.

**d. Không còn tự hồi phục.** Ai sửa thẳng dưới DB (không qua API) thì cache sai tới khi hết TTL 1h.
Ở v1 resync chữa trong tối đa 5 phút. README nên demo đúng case này để thấy rõ khác biệt.

---

## 5. Docs

`docs.md` viết lại theo cache-aside cho catalog (không phải flat product list như bản gốc). Phải giữ
lại bảng so sánh Cache Aside ↔ Read Model và mục 4 ở trên — giá trị học tập của repo nằm ở chỗ thấy
được vì sao chọn cái này bỏ cái kia, chứ không phải ở việc có mỗi một pattern.

`README.md` viết lại bộ curl, bám đúng thứ tự:

1. `GET /catalog` lần 1 → `source: db`, lần 2 → `source: cache`
2. `PUT /sizes/:id` đổi giá → `GET` lại → `source: db` (key đã bị xoá), giá mới
3. Category/product inactive không xuất hiện trong catalog
4. `POST /orders` lấy giá từ DB — sửa giá thẳng dưới DB rồi so cache vs order
5. Case lỗi: 404 client, 404 product/size, cross-tenant, order món inactive
6. Reset data

---

## 6. Verify

- [ ] `go build ./...` và `go vet ./...` sạch
- [ ] `docker exec redis redis-cli KEYS 'client:*:catalog'` — **rỗng** lúc app vừa start (không warm-up)
- [ ] `GET /catalog` lần 1 `source: db`, lần 2 `source: cache`
- [ ] `TTL client:1001:catalog` ≈ 3600
- [ ] Sau mỗi write, key biến mất; `GET` kế tiếp là `source: db`
- [ ] `POST /orders` trả giá mới ngay cả khi cache đang giữ giá cũ
- [ ] Product inactive và category inactive không có trong catalog nhưng order vẫn reject đúng
- [ ] Log không còn dòng `catalog warm-up` / `catalog resync`

**Điều kiện tiên quyết chưa xong:** bảng `products` cũ (schema cache-aside đời đầu, có cột `price`,
thiếu `category_id`) vẫn nằm trong `idempotency_lab` và chặn `AutoMigrate`. Data model của v2 giống
hệt v1 nên vẫn cần drop:

```bash
docker exec idempotency-postgres-1 psql -U postgres -d idempotency_lab \
  -c 'DROP TABLE IF EXISTS products CASCADE;'
```
