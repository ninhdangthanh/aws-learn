# Note phỏng vấn Middle Backend Golang — Rà soát codebase SaaS

> Bản này viết cho **phỏng vấn Golang**, đi sâu vào 12 service Go (`saas-microservice-*`, ~330k dòng).
> Mỗi phát hiện đều ghi rõ: **API nào**, **data gì**, **lý thuyết đằng sau**.
> File cũ `INTERVIEW-NOTES-INFRA.md` nghiêng về TypeScript — bản này bổ sung và sửa lại.

## Mục lục
1. [Bản đồ 12 service Go](#1)
2. [Câu hỏi 1: Compound index](#2)
3. [Câu hỏi 2: Materialized view](#3)
4. [Câu hỏi 3: Redis — và thứ thay thế nó](#4)
5. [Câu hỏi 4: Index nào tối ưu cho query nào](#5)
6. [Câu hỏi 5+6: RabbitMQ — data gì, 2 queue nhận không](#6)
7. [Câu hỏi 7: Tại sao reports dùng Postgres](#7)
8. [Kỹ thuật Go đáng nói nhất](#8)
9. [Bug thật để kể trong phỏng vấn](#9)
10. [Bộ câu hỏi + trả lời mẫu](#10)

---

<a name="1"></a>
## 1. Bản đồ 12 service Go

| Service | Files | Lines | Go | Vai trò |
|---|---|---|---|---|
| `order-pos` | 179 | 70.5k | 1.25.5 | **Lớn nhất.** Đơn hàng POS, KDS, split bill, refund, export |
| `menu` | 220 | 68.2k | 1.23.11 | Menu, sản phẩm, topping, LTO. **Có Elasticsearch cache** |
| `payment` | 113 | 37.3k | 1.23.11 | Thanh toán, tokenization, Square webhook |
| `store` | 158 | 33.2k | 1.25.5 | Cửa hàng, checklist, **queue number counter** |
| `system` | 162 | 24.9k | 1.25.11 | Client config, app version, **audit log consumer** |
| `inventory` | 145 | 23.4k | 1.24.9 | Tồn kho, PO, transfer |
| `promotion` | 80 | 22.4k | 1.23.11 | Khuyến mãi |
| `customer` | 78 | 16.5k | 1.23.11 | Khách hàng, điểm |
| `tenant` | 85 | 15.5k | 1.23.7 | User, session, token, permission |
| `handlequeue-menu` | 94 | 12.9k | 1.24 | **Worker thuần** — chỉ tiêu thụ queue |
| `report` | 50 | 10.5k | 1.24.0 | Report |
| `handlequeue-inventory` | 61 | 7.4k | 1.23.0 | **Worker thuần** + **Change Streams** |

**Stack chung:** Gin + mongo-driver v2 + streadway/amqp + gRPC + Viper + Logrus + OpenTelemetry/Jaeger.

> **Điểm đáng nói ngay:** có 2 service `handlequeue-*` **không phục vụ HTTP**, chỉ chạy consumer.
> Đây là **tách read/write path và tách workload** — API service không bị block bởi job nặng,
> và scale được độc lập (worker cần nhiều CPU, API cần nhiều connection).

---

<a name="2"></a>
## 2. Câu hỏi 1 — Compound index ở đâu?

### 2.1 Tổng kết số liệu (đã rà cả TS lẫn Go)

| Nguồn | Cách khai báo | Số compound index |
|---|---|---|
| TypeScript (Mongoose) | `Schema.index({...})` | **121** |
| Go (mongo-driver) | `mongo.IndexModel{Keys: bson.D{...}}` | **14** compound + ~227 single-field |
| PostgreSQL (TypeORM) | `@Index(name, [...], {unique})` | **2** |

**Điểm quan trọng về Go:** index được tạo **trong code lúc runtime**, không bằng migration.
Hai pattern:

```go
// Pattern A — trong init(), chạy 1 lần khi package load
func init() {
    orderCollection = db.Collection("order")
    orderCollectionOnce.Do(func() {
        indexes := []mongo.IndexModel{...}
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        _, err := orderCollection.Indexes().CreateMany(ctx, indexes)
    })
}
// saas-microservice-order-pos/models/order.go

// Pattern B — lazy, trong hàm lấy collection
func (m *QueueNumberCounter) model() *mongo.Collection {
    coll := database.M.Collection("queue_number_counters")
    queueNumberCounterOnce.Do(func() { ... CreateMany(ctx, indexes) })
    return coll
}
// saas-microservice-store/models/queue_number_counter.go
```

> **Câu hỏi phỏng vấn cài sẵn:** *"Tạo index trong code có vấn đề gì?"*
> Trả lời: `createIndex` trên collection đã có dữ liệu lớn sẽ **block** (foreground build) hoặc
> chạy nền tốn IO (background). Deploy 10 pod cùng lúc = 10 lần gọi `CreateMany` song song.
> Mongo idempotent nên không lỗi, nhưng vẫn tốn tài nguyên.
> Đúng ra nên tách thành **migration job** chạy 1 lần trước khi rollout.
> Và ở đây **lỗi tạo index chỉ được `log`, không fail startup** → service chạy bình thường mà thiếu index.

### 2.2 Compound index trong Go — 14 cái

| Service | File:line | Fields | Options |
|---|---|---|---|
| store | `models/queue_number_counter.go:41` | `day_jst, client_uuid, store_uuid` | **UNIQUE** |
| system | `modelsV2/historyCashCount.repo.go:31` | `client_uuid, store_uuid, business_date, terminal_id` | |
| system | `models/site_setting.go:46` | `client_uuid, is_active, is_delete` | |
| system | `modelsV2/window_service_log.go:41` | `client_uuid, store_code` | |
| inventory | `models/materialOOSWarning.go:35` | `client_uuid, is_active, is_delete` | |
| inventory | `models/storeBatchMaterial.go:46` | `client_uuid, is_delete` | |
| inventory | `models/predictSales.go:35` | `client_uuid, is_active, is_delete` | |
| order-pos | `models/hookTracking.go:30` | `order_uuid, status_code` | |
| order-pos | `models/customerAddressNote.go:31` | `customer_uuid, full_address, is_delete` | |
| tenant | `models/token_verify.go:27` | `user_uuid, disabled` | |
| store | `models/checklist.go:40` | `is_delete, is_active` | |
| store | `models/checklist_type.go:37` | `is_delete, is_active` | |
| store | `models/checklist_store.go:43` | `is_delete, is_active` | |
| customer | `models/wait_confirm_customer.go:40` | (single) `created_at` | **TTL 300s** |

### 2.3 ⭐ Compound index xịn nhất repo — `queue_number_counters`

**API:** `POST /v1/queue-number` (service `store`)
**Data:** request `{store_uuid}` + header `X-CLIENT-ID`; response `{queue_number: "007", seq: 7, day_jst, client_uuid, store_uuid}`
**Nghiệp vụ:** cấp số thứ tự chờ cho khách tại quầy, reset về 1 mỗi ngày theo giờ Nhật.

```go
// saas-microservice-store/models/queue_number_counter.go:41
indexes := []mongo.IndexModel{
    {
        // Unique compound — DB enforces no duplicate triplet.
        // This index is MANDATORY for correctness; without it
        // concurrent upserts may create duplicate docs.
        Keys: bson.D{
            {Key: "day_jst", Value: 1},
            {Key: "client_uuid", Value: 1},
            {Key: "store_uuid", Value: 1},
        },
        Options: options.Index().SetUnique(true).SetName("uniq_day_client_store"),
    },
    {
        // TTL: auto-purge counters 30 days after last update
        Keys:    bson.D{{Key: "updated_at", Value: 1}},
        Options: options.Index().SetExpireAfterSeconds(30 * 24 * 3600),
    },
}
```

**Vì sao đây là ví dụ hoàn hảo — 5 lý thuyết trong 1 hàm:**

```go
func (m *QueueNumberCounter) Issue(ctx context.Context, dayJST, clientUUID, storeUUID string) (int64, error) {
    // (1) FAIL-FAST: nếu index không tạo được thì từ chối phục vụ,
    //     vì không có unique index thì kết quả có thể sai.
    if queueNumberCounterIndexErr != nil {
        return 0, fmt.Errorf("queue_number_counters index unavailable: %w", queueNumberCounterIndexErr)
    }

    filter := bson.M{"day_jst": dayJST, "client_uuid": clientUUID, "store_uuid": storeUUID}
    update := bson.M{
        "$inc":         bson.M{"seq": 1},                    // (2) ATOMIC INCREMENT
        "$set":         bson.M{"updated_at": now},
        "$setOnInsert": bson.M{"day_jst": dayJST, ...},      // (3) chỉ set khi INSERT
    }
    opts := options.FindOneAndUpdate().
        SetUpsert(true).                                     // (4) UPSERT
        SetReturnDocument(options.After)                     //     trả về giá trị SAU khi tăng

    var result QueueNumberCounter
    err := coll.FindOneAndUpdate(ctx, filter, update, opts).Decode(&result)
    if err != nil && mongo.IsDuplicateKeyError(err) {
        // (5) RETRY khi upsert race: 2 goroutine cùng insert lần đầu,
        //     1 thắng, 1 nhận E11000. Lần 2 chắc chắn đi vào nhánh $inc thuần.
        err = coll.FindOneAndUpdate(ctx, filter, update, opts).Decode(&result)
    }
    return result.Seq, err
}
```

**Lý thuyết phải nói được:**
- **`FindOneAndUpdate` là atomic ở mức single-document.** MongoDB đảm bảo serialize các thao tác trên cùng 1 document. Không cần transaction, không cần lock ứng dụng.
- **`$inc` là read-modify-write nguyên tử** thực hiện *trong server*, khác hoàn toàn với `find()` → `+1` → `save()` (cái này lost update chắc chắn).
- **`upsert` + unique index = idempotent creation.** Không có unique index, 2 upsert đồng thời trên document chưa tồn tại có thể tạo ra **2 document** → 2 khách nhận cùng số.
- **`$setOnInsert`** tách metadata khởi tạo khỏi field cập nhật liên tục.
- **Retry đúng 1 lần** là đủ, vì sau lần 1 document chắc chắn tồn tại.

**Và có test chứng minh** — `models/queue_number_counter_integration_test.go`:
```go
//go:build integration          // ← build tag, không chạy trong unit test thường

func TestQueueNumberCounter_Concurrent100(t *testing.T) {
    const N = 100
    var wg sync.WaitGroup
    results := make(chan int64, N)     // buffered channel = không block goroutine
    errs    := make(chan error, N)

    for i := 0; i < N; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            n, err := new(models.QueueNumberCounter).Issue(ctx, testDayJST, clientUUID, testStore)
            if err != nil { errs <- err; return }
            results <- n
        }()
    }
    wg.Wait()
    close(results); close(errs)       // đóng SAU Wait → range không deadlock

    seen := make(map[int64]bool, N)
    for n := range results {
        if seen[n] { t.Fatalf("duplicate seq detected: %d", n) }
        seen[n] = true
    }
    // Bắt buộc phải là [1..100], không trùng, không thiếu
}
```

> **Đây là đoạn code bạn nên mang vào phỏng vấn.** Nó thể hiện: hiểu concurrency,
> biết dùng `sync.WaitGroup` + buffered channel đúng cách, biết viết test cho race condition,
> và biết dùng build tag để tách integration test.
> **Câu hỏi cài sẵn:** *"Vì sao channel phải buffered?"* → Nếu unbuffered, goroutine sẽ block ở
> `results <- n` cho tới khi có người đọc; nhưng người đọc chỉ chạy sau `wg.Wait()` → **deadlock**.

### 2.4 Ngược lại: `order` collection — 13 index đơn, 0 compound

```go
// saas-microservice-order-pos/models/order.go — init()
indexes := []mongo.IndexModel{
    {Keys: bson.D{{Key: "client_uuid",       Value: -1}}},
    {Keys: bson.D{{Key: "uuid",              Value: -1}}},
    {Keys: bson.D{{Key: "order_status_code", Value: -1}}},
    {Keys: bson.D{{Key: "order_status_uuid", Value: -1}}},
    {Keys: bson.D{{Key: "store_uuid",        Value: -1}}},
    {Keys: bson.D{{Key: "store_code",        Value: -1}}},
    {Keys: bson.D{{Key: "external_id",       Value: -1}}},
    {Keys: bson.D{{Key: "tracking_uuid",     Value: -1}}},
    {Keys: bson.D{{Key: "is_active",         Value: -1}}},
    {Keys: bson.D{{Key: "is_delete",         Value: -1}}},
    {Keys: bson.D{{Key: "code",              Value: -1}}},
    {Keys: bson.D{{Key: "number",            Value: -1}}},
    {Keys: bson.D{{Key: "created_by",        Value: -1}}},
}
```

**Đây là collection nóng nhất hệ thống, và không có một compound index nào.** Chi tiết ở [mục 5](#5).

---

<a name="3"></a>
## 3. Câu hỏi 2 — Materialized view

**Kết quả rà soát: KHÔNG có, ở cả Postgres lẫn Mongo.**
- Không `CREATE MATERIALIZED VIEW` / `REFRESH MATERIALIZED VIEW` ở bất kỳ đâu.
- Không có file `.sql` nào trong repo (schema quản lý ngoài; TypeORM để `synchronize: false`).
- Mongo không có khái niệm materialized view; thứ gần nhất là `$merge`/`$out` — **cũng không dùng**.

**Thứ thay thế — 2 tầng:**

**Tầng 1 — Bảng tổng hợp trong Postgres** (`saas-management-reports/src/lib/postgres.lib.ts:26-38`):
`DashboardOrderHourlyReport`, `MenuReport`, `OrderPaymentReport`, `OrderPromotionReport`,
`CountPromotion`, `StoreForecast`, `ProductQuantityForecast`, `PricingTier`.
Cập nhật **incremental** qua consumer RabbitMQ, không refresh toàn bộ.

**Tầng 2 — Collection "flat" trong Mongo** (đây là thứ mới, chưa nói ở file trước):
```
saas-microservice-handlequeue-inventory/models/flat_material_quantity.go
saas-microservice-handlequeue-inventory/models/flat_material_master.go
```
Tiền tố `flat_` = **denormalize sẵn**, gộp nhiều collection thành 1 document phẳng để đọc
không cần `$lookup`. Đây chính là **materialized view thủ công trong Mongo**.

**Cách trả lời:**
> "Bọn em không dùng `MATERIALIZED VIEW`. Lý do: `REFRESH` không `CONCURRENTLY` sẽ khoá
> ACCESS EXCLUSIVE, còn `CONCURRENTLY` thì cần unique index và chậm hơn nhiều — mà quan trọng nhất là
> MV **refresh toàn bộ**, trong khi dữ liệu bọn em append-only theo ngày, chỉ cần cập nhật phần tăng thêm.
> Nên bọn em tự quản bảng tổng hợp, cập nhật bằng event. Bên Mongo thì có các collection `flat_*`
> — cùng ý tưởng: denormalize sẵn để đọc không cần join.
> Cái giá phải trả là **tự lo consistency**, thứ mà MV cho sẵn."

---

<a name="4"></a>
## 4. Câu hỏi 3 — Redis: có dùng không, cache cái gì?

### 4.1 Redis: KHÔNG dùng (đã xác minh kỹ)
- 16 `package.json`: không có `redis`/`ioredis`/`cache-manager`/`bullmq`.
- 12 `go.mod`: không có `go-redis`/`redigo`.
- Chữ `ioredis` duy nhất trong `saas-management-reports/package-lock.json` là **optional peer-dep của TypeORM** (cho query cache của nó) — không cài, không dùng.

### 4.2 ⭐ NHƯNG có cache thật — bằng **Elasticsearch** (phát hiện quan trọng nhất bản này)

`saas-microservice-menu/service/elasticsearchService/cache.go` — một **cache layer hoàn chỉnh** viết trên Elasticsearch index tên `cache`.

**Cấu trúc bản ghi cache:**
```go
type CacheObj struct {
    Type         string   `json:"type"`          // "product_aa"  — namespace
    Key          string   `json:"key"`           // md5 hash của toàn bộ query params
    Value        string   `json:"value"`         // JSON response đã serialize
    StoreUuids   []string `json:"store_uuids"`   // ← TAG để invalidate
    ProductUuids []string `json:"product_uuids"` // ← TAG để invalidate
    CreatedAt    string   `json:"created_at"`
}
```

**API dùng nó:** `GET /v1/product-aa` (`ProductAAController.ListAA`, `controllers/productAA.go:143`)
— API lấy danh sách sản phẩm cho app đặt hàng, là **API nóng nhất** của service menu.

**Data cache:** toàn bộ response danh sách sản phẩm (`[]request.GetListResponse`) — bao gồm sản phẩm, option group, topping, giá, tag, campaign.

**Cache key** (`productAA.go:38-141`) — md5 của **28 tham số** đã sort:
```go
func (self *ProductAAController) getCacheKey(req request.GetListRequest) string {
    cacheKey := ""
    arr := strings.Split(req.Uuids, ",")
    sort.Strings(arr)                       // ← SORT trước khi ghép
    for _, val := range arr { cacheKey += val }
    // ... lặp lại cho CategoryUuid, StoreUuid, StoreGroupUuid, TagUuid,
    //     CampaignUuid, BrandUuids, Filters, ... (14 field dạng list)
    cacheKey += fmt.Sprintf("%d", req.Page)
    cacheKey += fmt.Sprintf("%.2f", req.MinPrice)
    // ... (14 field scalar)
    hashKey := md5.Sum([]byte(cacheKey))
    return fmt.Sprintf("%x", hashKey)
}
```
> **Điểm hay:** `sort.Strings(arr)` trước khi ghép → `?uuids=A,B` và `?uuids=B,A` cho **cùng một key**.
> Đây là **cache key normalization** — nếu không làm, hit rate sẽ tụt thảm hại.
> Nói được chi tiết này là ăn điểm.

**Luồng cache-aside đầy đủ** (`productAA.go:162-280`):
```go
// 1. Feature flag PER TENANT — bật/tắt cache theo từng khách hàng
if config.GetConfigurationInt(clientUUID, "product_aa_cache") == 1 && req.TagCampaignUuid != "default" {
    cacheKey := self.getCacheKey(req)
    cacheVal, found := cacheES.Get("product_aa", cacheKey)
    if found {
        json.Unmarshal([]byte(cacheVal), &cacheProds)
        // 2. QUAN TRỌNG: tồn kho KHÔNG lấy từ cache — query Mongo tươi
        //    rồi ghép vào response đã cache
        inventoryList, _ := inventoryModel.Find(cond)   // cond: client_uuid + sku $in [...]
        for _, item := range inventoryList {
            inventoryOG[item.Sku] = item.Quantity - item.OrderedQuantity
        }
        // ... trả response
    }
}
// 3. Cache miss → query đầy đủ → ghi cache kèm TAG
cacheES.Set("product_aa", cacheKey, resp, productUuids, storeUuids)
```

> **Đây là kỹ thuật rất đáng nói: partial cache / hybrid.**
> Phần **ít thay đổi** (tên, giá, mô tả, ảnh sản phẩm) thì cache.
> Phần **thay đổi liên tục** (tồn kho `quantity - ordered_quantity`) thì luôn query tươi rồi ghép vào.
> → Vừa được tốc độ của cache, vừa không bao giờ bán hàng đã hết.
> Trong phỏng vấn gọi tên nó là: **cache the slow-changing, compute the fast-changing**.

**Cache invalidation bằng TAG — qua gRPC** (`grpc/service/product.go:39-44`):
```go
func (ps *ProductService) DeleteCacheByProductUuids(ctx context.Context, req *pb.DeleteCacheByProductUuidsReq) (*pb.DeleteCacheByProductUuidsRes, error) {
    cacheES := elasticsearchService.EsCache{}
    cacheES.Delete(req.Uuids)          // → ES DeleteByQuery { terms: { product_uuids: [...] } }
    return &pb.DeleteCacheByProductUuidsRes{Uuids: req.Uuids}, nil
}
```
→ Service khác sửa sản phẩm → gọi gRPC `DeleteCacheByProductUuids([uuid...])` → menu-service
xoá **mọi** entry cache có chứa uuid đó trong mảng `product_uuids`.

> **Đây là tag-based cache invalidation** — giống `cache tags` của Laravel, hoặc Redis Set lưu
> `product:<uuid>:keys` rồi `SREM`. Cách này **chính xác hơn TTL rất nhiều**: cache sống mãi cho tới
> khi dữ liệu thực sự đổi. Có 3 hàm invalidate: `DeleteCacheByProductUuids`,
> `DeleteCacheByStoreUuids`, `DeleteCacheByKey`.

### 4.3 Vì sao dùng Elasticsearch làm cache là lựa chọn đáng bàn

**Nói được cả 2 mặt mới là senior:**

**Lý do hợp lý:**
- ES **đã có sẵn** trong hệ thống (dùng cho universal search) → không phải thêm hạ tầng.
- `DeleteByQuery` trên mảng `product_uuids` cho phép **tag invalidation** mà Redis thuần không làm được (Redis phải tự duy trì reverse index).
- Cache value có thể to (danh sách vài trăm sản phẩm) — ES xử lý document lớn tốt.

**Nhược điểm nghiêm trọng — phải chỉ ra được:**

| Vấn đề | Chi tiết |
|---|---|
| **Không có TTL** | `created_at` khai báo kiểu `keyword` (string!) chứ không phải `date` → **không thể range query để hết hạn**. Entry sống vĩnh viễn cho tới khi bị invalidate thủ công. Sót 1 đường invalidate = stale data mãi mãi. |
| **Ghi tốn 3 round-trip** | `Set()` gọi `IndexCache()` (kiểm tra index tồn tại) → `DeleteCacheByKey()` → `SetCache()`. Redis chỉ cần 1 lệnh `SETEX`. |
| **`WithRefresh("true")`** | `cache.go:224` — ép ES refresh segment **ngay lập tức** mỗi lần ghi cache. Đây là thao tác **rất đắt** trong Lucene, phá vỡ near-real-time batching. Với QPS cao sẽ giết cluster. |
| **Latency** | ES query ~5-50ms. Redis ~0.1-1ms. Chênh 1-2 bậc độ lớn. |
| **Không có eviction policy** | Redis có `maxmemory-policy allkeys-lru`. ES index `cache` sẽ phình vô hạn. |
| **`recover()` nuốt lỗi** | `es.go:83-90` — `defer func(){ if r := recover(); r != nil { results = SearchResults{}; err = nil } }()` → panic bị nuốt và **trả `err = nil`** → caller tưởng cache miss bình thường. Che giấu sự cố. |

**Câu trả lời chuẩn:**
> "Hệ thống em không dùng Redis. Cache của API sản phẩm được xây trên Elasticsearch,
> tận dụng ES đã có sẵn cho search. Điểm hay là **invalidate theo tag** — service khác sửa sản phẩm
> thì gọi gRPC để xoá đúng những entry chứa product_uuid đó, chính xác hơn TTL.
> Nhưng nếu được thiết kế lại em sẽ dùng Redis cho phần cache: ES không có TTL trong implement hiện tại
> (`created_at` lưu dạng keyword nên không range được), mỗi lần ghi tốn 3 round-trip và có
> `refresh=true` — rất đắt với Lucene. Redis sẽ cho `SETEX` 1 lệnh, latency thấp hơn 1-2 bậc,
> và có sẵn LRU eviction. Phần tag invalidation thì em duy trì Set `product:<uuid>:keys` rồi `SREM`."

### 4.4 Các "Redis-substitute" khác trong repo

| Nhu cầu | Cách chuẩn (Redis) | Repo dùng gì | File | Đánh giá |
|---|---|---|---|---|
| Cache response | `SETEX` | **Elasticsearch** index `cache` | `menu/service/elasticsearchService/cache.go` | 🟡 Chạy được, nhiều nhược điểm |
| In-process cache | — | `patrickmn/go-cache` (đã import) | `menu/controllers/productAA.go:17` | 🟡 Import nhưng **field `Cache *cache.Cache` không được khởi tạo** ở `routes/router.go` → dùng sẽ nil-panic |
| Sinh số tuần tự | `INCR` | Mongo `FindOneAndUpdate` + `$inc` + unique index | `store/models/queue_number_counter.go` | ✅ Làm rất chuẩn |
| Distributed lock | `SET NX PX` | Mongo insert vào `event_lock` | `handlequeue-inventory/models/event_lock.go` | ❌ **Không có unique index → không khoá gì cả**, xem [mục 9](#9) |
| Hết hạn dữ liệu | `EXPIRE` | Mongo TTL index | `store/models/queue_number_counter.go:56` (30 ngày), `customer/models/wait_confirm_customer.go:40` (300s) | ✅ |
| Delayed job | Sorted Set `ZADD` | RabbitMQ `x-delayed-message` plugin | 7 service Go | ✅ |
| Rate limiting | `INCR` + `EXPIRE` | **Không có** | — | ❌ Thiếu |
| Cache token auth | Cache session | **gRPC call mỗi request** | middleware `Authentication()` | ❌ Nợ kỹ thuật lớn |

---

<a name="5"></a>
## 5. Câu hỏi 4 — Index nào tối ưu cho query nào? (phần quan trọng nhất)

### 5.1 Lý thuyết phải thuộc

**Prefix rule:** index `{a,b,c}` phục vụ `{a}`, `{a,b}`, `{a,b,c}` — **không** phục vụ `{b}` hay `{c}`.

**ESR rule** — thứ tự field tối ưu: **E**quality → **S**ort → **R**ange.
Lý do: sau một field range, các field phía sau **không còn sorted** trong vùng scan → không dùng để sort được.

**Một query Mongo chỉ dùng được MỘT index.** (Index intersection tồn tại nhưng planner hiếm khi chọn và thường chậm hơn.) → **N index đơn ≠ 1 compound index.**

**Sargable:** predicate phải để cột "trần" thì mới dùng được index. `field::date BETWEEN` hay `$regex` không neo đầu → non-sargable.

### 5.2 ⭐ Case study: `GET /v1/orders` — nơi index sai rõ nhất

**API:** `GET /v1/orders` → `OrderPosController.List` (`controllers/orderPos.go:1757`)
**Data trả về:** danh sách đơn + chi tiết + trạng thái + loại đơn + thiết bị (join 4 collection)

**Query được build:**
```go
cond := bson.M{"client_uuid": clientUuid}              // luôn có — EQUALITY

if req.CustomerUuid    != "" { cond["customer_uuid"]     = req.CustomerUuid }      // EQ
if req.OrderDeviceUuid != "" { cond["order_device_uuid"] = req.OrderDeviceUuid }   // EQ
if req.OrderTypeUuid   != "" { cond["order_type_uuid"]   = req.OrderTypeUuid }     // EQ
if req.OrderStatusUuid != "" { cond["order_status_uuid"] = req.OrderStatusUuid }   // EQ

if req.FromDate != "" { cond["collection_time"] = bson.M{"$gte": fromDate} }       // RANGE
if req.ToDate   != "" { cond["collection_time"] = bson.M{"$lte": toDate}   }       // RANGE

if req.CustomerName  != "" { cond["name"]  = bson.M{"$regex": customerName, "$options": "si"} }
if req.CustomerPhone != "" { cond["phone"] = bson.M{"$regex": phoneNumber,  "$options": "si"} }
```
Rồi `Pagination()` thêm:
```go
conditions["is_active"] = constant.IS_ACTIVE     // EQ
conditions["is_delete"] = constant.UNDELETE      // EQ
```
và sort theo `order.order_time`, `$skip` + `$limit`.

**Query thực tế đầy đủ:**
```js
{ client_uuid: X, is_active: 1, is_delete: 0,
  order_status_uuid: Y,
  collection_time: { $gte: ..., $lte: ... } }
sort: { order_time: -1 }
```

**Index hiện có:** 13 index đơn. Planner chọn được **đúng 1** — nhiều khả năng `{client_uuid: -1}`.
Với hệ multi-tenant, 1 tenant lớn có thể chiếm phần lớn collection → **quét gần như toàn bộ**,
rồi lọc `is_active`, `is_delete`, `order_status_uuid`, `collection_time` **trong bộ nhớ**,
rồi **sort in-memory** (giới hạn 32MB, vượt là lỗi `Sort exceeded memory limit`).

**Index cần tạo — áp dụng ESR:**
```go
{
    Keys: bson.D{
        {Key: "client_uuid",       Value: 1},   // E — tenant, luôn có
        {Key: "is_delete",         Value: 1},   // E — luôn có (Pagination thêm)
        {Key: "is_active",         Value: 1},   // E — luôn có
        {Key: "order_status_uuid", Value: 1},   // E — filter phổ biến nhất
        {Key: "collection_time",   Value: -1},  // R + S — range VÀ sort, để CUỐI
    },
    Options: options.Index().SetName("idx_orders_list"),
}
```
**Vì sao đúng:**
- 4 field equality đứng trước → thu hẹp cực mạnh trước khi chạm range.
- `collection_time` vừa là range vừa là sort → đặt cuối cùng, Mongo **quét index theo đúng thứ tự sort** → **không cần sort in-memory** (trong `explain` sẽ mất stage `SORT`).
- `-1` khớp hướng sort giảm dần; Mongo đọc ngược index được nên hướng không bắt buộc, nhưng khớp thì tốt hơn.

**Cách xác minh — nói ra để chứng minh bạn làm thật:**
```js
db.order.find({...}).sort({...}).explain("executionStats")
// So sánh:
//   winningPlan.stage: COLLSCAN → IXSCAN
//   totalKeysExamined / nReturned  → càng gần 1.0 càng tốt
//   totalDocsExamined              → phải giảm mạnh
//   có stage "SORT" không          → nếu còn thì index chưa phục vụ được sort
//   executionTimeMillis
```

### 5.3 Hai field KHÔNG BAO GIỜ dùng được index trong API này

```go
cond["name"]  = bson.M{"$regex": customerName, "$options": "si"}
cond["phone"] = bson.M{"$regex": phoneNumber,  "$options": "si"}
```
- `$regex` **không neo đầu** (`^`) → Mongo phải quét toàn bộ, index vô dụng.
- `$options: "i"` (case-insensitive) → **kể cả có `^` cũng vẫn không dùng được index** (trừ khi dùng collation).
- Thêm nữa: regex do người dùng nhập → **rủi ro ReDoS**.

**Giải pháp đúng:**
1. **Text index** + `$text: {$search: ...}` — cho tìm tên.
2. Hoặc lưu thêm field `name_normalized` (lowercase, bỏ dấu) rồi so sánh prefix có neo `^`:
   `{name_normalized: {$regex: "^" + q}}` → dùng được index.
3. Với `phone`: chuẩn hoá về E.164 rồi **equality match** — số điện thoại không cần fuzzy search.

### 5.4 Compound index Go tốt — để so sánh

```go
// saas-microservice-system/modelsV2/historyCashCount.repo.go:31
Keys: bson.D{
    {Key: "client_uuid",   Value: 1},   // E
    {Key: "store_uuid",    Value: 1},   // E
    {Key: "business_date", Value: 1},   // R/S
    {Key: "terminal_id",   Value: 1},   // ⚠ đứng SAU range → không seek được
}
```
**API:** lịch sử kiểm kê tiền mặt theo cửa hàng/ngày.
Ba field đầu đúng ESR. `terminal_id` đứng sau range nên chỉ có tác dụng khi **covered query**
(tất cả field cần đọc đều nằm trong index → Mongo không phải fetch document).
Nếu `terminal_id` là filter thường xuyên thì phải đưa lên **trước** `business_date`.

```go
// saas-microservice-order-pos/models/hookTracking.go:30
Keys: bson.D{{Key: "order_uuid", Value: 1}, {Key: "status_code", Value: 1}}
```
2 equality — chuẩn. Dùng để tra "webhook cho đơn X ở trạng thái Y đã xử lý chưa" → **chống xử lý trùng webhook**.

### 5.5 Nhóm index vô dụng — 3 chỗ trong Go, 14 chỗ trong TS

```go
// saas-microservice-store/models/checklist.go:40
Keys: bson.D{{Key: "is_delete", Value: 1}, {Key: "is_active", Value: 1}}
```
2 field boolean → chỉ có 4 tổ hợp giá trị → **selectivity ~25%**.
Planner gần như chắc chắn bỏ qua và chọn COLLSCAN, vì đọc index rồi fetch 25% document
còn đắt hơn quét thẳng. Index này chỉ tốn chi phí ghi, không mang lại gì.
→ Phải có `client_uuid` đứng đầu mới có ý nghĩa.

---

<a name="6"></a>
## 6. Câu hỏi 5+6 — RabbitMQ: dùng ở đâu, chở data gì, có 2 queue cùng nhận không?

### 6.1 Lý thuyết

```
Producer → [Exchange] --binding key--> [Queue] → Consumer
```
Producer **không gửi thẳng vào queue** (trừ default exchange `""`). Exchange dựa vào **binding** quyết định copy vào queue nào.

| Exchange type | Route theo | Repo dùng ở đâu |
|---|---|---|
| `direct` | routing key khớp **chính xác** | `ex.out_of_stock.<tenant>`, `ORDER_POS` |
| `topic` | khớp **pattern** (`*`=1 từ, `#`=0+ từ) | `audit_log_exchange`, `OSS-PRODUCT`, `OSS-INVENTORY` |
| `fanout` | bỏ qua key, copy vào **mọi** queue | **Không dùng** |
| `x-delayed-message` | plugin, giao trễ theo header | 7 service Go |

**Mấu chốt:** 1 message vào nhiều queue **khi và chỉ khi** ≥2 binding cùng khớp → exchange **copy** message, mỗi queue 1 bản độc lập, ack riêng.
**Khác hoàn toàn** với nhiều consumer trên **cùng 1 queue** → round-robin, mỗi message chỉ 1 consumer nhận (competing consumers).

### 6.2 Bảng đầy đủ: exchange → data → API

| Exchange | Type | Publisher | Queue(s) | Routing key | **Data payload** |
|---|---|---|---|---|---|
| `audit_log_exchange` | topic | 6 service TS (`menu`,`promotion`,`store`,`payment`,`auth`,`user`) qua middleware `auditLog.ts` | `OSS_USER`, `OSS_AUTH`, `OSS_MENU`, `OSS_STORE` | `<scope>.<action>` vd `menu.create` | `{ip, user_uuid, client_uuid, action, scope, status, url, method, target, metadata:{user_agent, device, browser, os, location}, created_at}` |
| `ORDER_POS` | direct/topic | `order-pos` (Go) `services/orderIntegrate/publishMessage.go` | (ngoài repo: KDS/đối tác) | `CREATED`,`UPDATED`,`FINISH`,`CANCEL`,`REFUND_REQUESTED_TERMINAL` | `{client_uuid, uuid, total, currency:"JPY", status, order_items:[{product_uuid, product_name, quantity, price}], created_at, updated_at, updated_by}` |
| `ex.out_of_stock.<client_uuid>` | direct | `saas-management-inventory` | POS/KDS client | routing key = `store_uuid` | `{type:"PRODUCT"\|"TOPPING"\|"ITEM", data:{...}}`, TTL 2h |
| `OSS-PRODUCT` | topic | menu, customer, order | `product-price-logging`, `product-price-schedule`, `pricing-tier-schedule`, `sync-customer-smageri`, `sync-points-smageri`, `sync-order-smageri` | `PRICE_LOG_CREATE.CMD`, `PRICE_SCHEDULE.CMD`, `PRICING_TIER_SCHEDULE.CMD`, `SYNC_CUSTOMER_SCHEDULE.CMD`, `SYNC_POINTS_SCHEDULE.CMD`, `SYNC_ORDER_SCHEDULE.CMD` | Lệnh ghi log giá, lịch đổi giá, đồng bộ khách/điểm/đơn với **Smaregi** (POS Nhật) |
| `OSS-INVENTORY` | topic | inventory | `inventory-po-received`, `inventory-create-transfer-cmd` | `PO.RECEIVED`, `TRANSFER.CREATE.CMD` | Phiếu nhập đã nhận, lệnh tạo phiếu chuyển kho |
| `OSS-INVENTORY-V2-MOVEMENT` | topic | inventory | `inventory-v2-goods-receipt`, `inventory-v2-goods-return` | `GOODSRECEIPT.RECEIVED`, `GOODSRETURN.RETURNED` | Biến động tồn kho v2 |
| `OSS-PURCHASE-ORDER-REQUEST` | topic | inventory | `purchase-order-create-cmd` | `PO.CREATE.CMD` | Lệnh tạo đơn mua |
| `OSS-NOTIFICATION-PURCHASE-ORDER-REQUEST` | topic | inventory | `purchase-order-request-notification` | `PO.REQUEST.ACCEPTED` | Thông báo duyệt yêu cầu mua |

### 6.3 Queue trực tiếp (không qua exchange) — worker services

`handlequeue-inventory/main.go` — **8 queue, mỗi queue 1 goroutine**:
```go
go getAMQPQueue(cfg.GetString("rabbitmq.queue.inventory.schedule_export_material"),     ctl.ScheduleExportMaterial)
go getAMQPQueue(cfg.GetString("rabbitmq.queue.inventory.schedule_export_material_qty"), ctl.ScheduleExportMaterialQty)
go getAMQPQueue(cfg.GetString("rabbitmq.queue.inventory.schedule_export_item_to_s3"),   ctl.ExportMaterialQtyToS3)
go getAMQPQueue(cfg.GetString("rabbitmq.queue.inventory.potential"),                    inventoryCtl.InventoryPotentialHandler)
go getAMQPQueue(cfg.GetString("rabbitmq.queue.inventory.material_stock_report"),        materialStockReportCtl.CreateMaterialStockReport)
go getAMQPQueue(cfg.GetString("rabbitmq.queue.inventory.minusQuantity"),                retailInventoryCtl.MinusQuantity)
go getAMQPQueue(cfg.GetString("rabbitmq.queue.inventory.revertQuantity"),               retailInventoryCtl.RevertQuantity)
go getAMQPQueue(cfg.GetString("rabbitmq.queue.inventory.refundQuantity"),               retailInventoryCtl.RefundQuantity)
```
**Data:** `{client_uuid, timezone}` cho export; `{product_uuid, variant_uuid, location_uuid, quantity}` cho trừ/hoàn tồn kho.

> **Điểm đáng nói:** `minusQuantity` / `revertQuantity` / `refundQuantity` là 3 queue riêng cho
> 3 thao tác trên cùng một tài nguyên (tồn kho). Đây là **command queue** — mỗi lệnh 1 queue,
> cho phép scale và giám sát độc lập, nhưng **không đảm bảo thứ tự** giữa các queue.
> Nếu `minus` và `revert` của cùng 1 sản phẩm bị xử lý lệch thứ tự → tồn kho sai.
> Giải pháp: dùng 1 queue + routing key, hoặc consistent hashing theo `product_uuid`.

### 6.4 ⭐ Trả lời trực tiếp: "Có chỗ nào bắn vào exchange mà 2 queue nhận không?"

**KHÔNG — và tôi đã kiểm tra toàn bộ 8 exchange.**

Chi tiết `audit_log_exchange` (`saas-microservice-system/service/queue/auditLogSetup.go:22-27`):
```go
var queue2routingKey = map[QueueName][]string{
    AuditLogOSSUser:  {"user.*"},
    AuditLogOSSAuth:  {"auth.*"},
    AuditLogOSSMenu:  {"menu.*"},
    AuditLogOSSStore: {"store.*"},
}
```
4 pattern **loại trừ lẫn nhau** — không routing key nào khớp được 2 pattern. Mỗi message tối đa 1 queue.

**Cách trả lời hay:**
> "Hiện tại chưa có. Cả 8 exchange đều dùng binding key rời rạc nên mỗi message chỉ vào 1 queue.
> Nhưng việc chọn **topic** thay vì **direct** chính là để mở đường: nếu cần thêm một service
> Analytics nghe toàn bộ audit log, em chỉ khai báo thêm 1 queue bind pattern `#` — không sửa
> một dòng nào ở phía producer. Lúc đó 1 message sẽ vào 2 queue."

Vẽ ra giấy:
```
routing key "menu.create":
  binding "menu.*"  → khớp → OSS_MENU
  binding "#"       → khớp → ANALYTICS       ← 1 message, 2 queue, 2 bản độc lập
```

**Hai bug thật quanh exchange này (kể tiếp để ghi điểm):**

1. **Message bị vứt im lặng.** `payment` và `promotion` publish `payment.*` / `promotion.*`
   nhưng **không queue nào bind** 2 pattern đó. Publish với `mandatory=false` → RabbitMQ
   **drop không báo lỗi**. Audit log 2 service này mất trắng.
   *Phát hiện bằng:* `mandatory=true` + handler `basic.return`, hoặc cắm **Alternate Exchange**.

2. **Queue có mà không có consumer.** `main.go:105-106` chỉ start 2/4:
   ```go
   go queueService.ConsumeAuditLog(queue.AuditLogOSSUser,  10, auditLogCtl.AuditLogHandler)
   go queueService.ConsumeAuditLog(queue.AuditLogOSSStore, 10, auditLogCtl.AuditLogHandler)
   // OSS_AUTH và OSS_MENU: declare + bind, KHÔNG consume
   ```
   Được cứu bởi args ở `auditLogSetup.go:35-40`:
   ```go
   "x-message-ttl": int32(24 * time.Hour / time.Millisecond),   // message sống 24h
   "x-expires":     int32(72 * time.Hour / time.Millisecond),   // queue tự xoá sau 72h idle
   ```

### 6.5 Worker pool Go — đọc kỹ, đây là code phỏng vấn hay hỏi

```go
// saas-microservice-system/service/queue/amqp.go
func (r *AmqpQueueService) workerPool(numWorker int, queueName string,
                                      msgs <-chan amqp.Delivery,
                                      handler func(string, []byte) error) {
    var wg sync.WaitGroup
    for i := 0; i < numWorker; i++ {
        wg.Add(1)
        go r.worker(&wg, i, queueName, msgs, handler)   // N worker cùng đọc 1 channel
    }
    wg.Wait()
}

func (s *AmqpQueueService) worker(wg *sync.WaitGroup, index int, queueName string,
                                  c <-chan amqp.Delivery,
                                  handler func(string, []byte) error) {
    defer wg.Done()
    for {
        select {
        case <-s.Context.Done():          // graceful shutdown
            return
        case job, ok := <-c:
            if !ok {                      // channel đã đóng
                job.Ack(false)            // ⚠ BUG: job là zero-value
                return
            }
            if err := handler(job.RoutingKey, job.Body); err != nil {
                log.Println("Handler fail", err.Error())
            }
            if err := job.Ack(false); err != nil {   // ⚠ BUG: ACK KỂ CẢ KHI LỖI
                log.Println("ACK fail", err.Error())
                continue
            }
        }
    }
}
```

**Lý thuyết đúng:** đây là **fan-out pattern** — N goroutine cùng nhận từ 1 channel, Go runtime
tự phân phối. Kết hợp `Qos(numWorker, 0, false)` ở `auditLogSetup.go:88` → prefetch = số worker,
nên broker không đẩy quá số message mà pool xử lý nổi. **Đây là backpressure đúng cách.**

**Hai bug — nói ra là ăn điểm lớn:**

1. **Ack kể cả khi handler lỗi.** `handler` trả error → chỉ `log` → vẫn `Ack(false)`
   → RabbitMQ xoá message → **mất vĩnh viễn**. Đúng ra phải:
   ```go
   if err := handler(job.RoutingKey, job.Body); err != nil {
       job.Nack(false, false)   // requeue=false → đẩy sang Dead Letter Exchange
       continue
   }
   job.Ack(false)
   ```
   Và cần khai báo `x-dead-letter-exchange` trên queue. **Toàn repo không có một DLQ nào.**

2. **`job.Ack()` trên zero-value.** Khi channel đóng (`ok == false`), `job` là
   `amqp.Delivery{}` với `Acknowledger == nil` → gọi `Ack` sẽ lỗi hoặc panic.
   Đúng ra chỉ cần `return`.

### 6.6 Reliability — bảng tổng kết

| Kỹ thuật | Có? | Chi tiết |
|---|---|---|
| `durable` exchange + queue | ✅ | Mọi nơi |
| `persistent` message (deliveryMode=2) | ✅ | `amqp.Persistent` |
| Manual ack (`autoAck=false`) | ✅ | `Consume(q,"",false,...)` |
| Prefetch / QoS | ✅ | Go: `Qos(10)` phổ biến, `Qos(2)` cho menu. TS: `prefetch(15)`/`prefetch(5)` |
| Worker pool | ✅ | `system`, `handlequeue-*` |
| Reconnect + retry | ✅ | Có ở mọi service (⚠ `saas-management-reports` bị comment out dòng reconnect) |
| Tách channel producer/consumer | ✅ | Đúng — AMQP channel không thread-safe |
| Delayed message | ✅ | `x-delayed-message` plugin, 7 service |
| Message TTL | ✅ | `x-message-ttl` 24h, per-message `expiration` 2h |
| **Publisher Confirms** | ❌ | Không có → không biết broker đã nhận chưa |
| **Dead Letter Queue** | ❌ | Không có → poison message hoặc mất message |
| **Consistent ordering** | ❌ | 3 queue riêng cho minus/revert/refund tồn kho |

---

<a name="7"></a>
## 7. Câu hỏi 7 — Tại sao `reports` dùng Postgres khi hệ thống chạy Mongo?

### 7.1 Câu trả lời 30 giây
> "MongoDB là **OLTP** cho nghiệp vụ vận hành — ghi nhiều, đọc theo key, schema linh hoạt theo tenant.
> Nhưng báo cáo là **OLAP**: quét hàng triệu dòng chi tiết đơn, GROUP BY nhiều chiều, tính tỉ lệ tích luỹ,
> window function. Aggregation Pipeline của Mongo làm được nhưng chậm, khó viết, và quan trọng nhất là
> **chạy chung cluster với luồng bán hàng** — một report nặng có thể làm nghẽn việc tạo đơn.
> Nên bọn em ETL đơn hàng sang PostgreSQL dạng bảng phẳng và chạy toàn bộ report ở đó."

### 7.2 Bằng chứng cứng trong code

**(a) Chỉ 2 service chạm Postgres:** `saas-management-reports` (TypeORM) và `saas-management-order` (`pg` Pool thuần, `max: 20`).
`saas-management-reports` kết nối **cả hai** — Mongo cho master data, Postgres cho fact data.

**(b) Schema là star schema kinh điển** (`src/models/order.postgre.ts`, ~55 cột):
```ts
business_date;    // '2026-07-29'
business_year;    // 2026        ← pre-compute
business_month;   // 7           ← pre-compute
business_day;     // 29          ← pre-compute
business_quater;  // 'Q3'        ← pre-compute
business_hour;    // 14          ← pre-compute
```
> OLTP chỉ lưu 1 cột `timestamp` rồi `EXTRACT()`. Ở đây **pre-compute** để tránh function call
> trên hàng triệu dòng và cho phép index/partition trực tiếp. Đây là **denormalization for read**.

**(c) `allowDiskUse` xuất hiện 10 lần** trong `saas-management-order` → aggregation đã **vượt trần 100MB RAM** của Mongo và phải spill ra đĩa. Đây là bằng chứng thực tế nhất: họ đã đụng giới hạn.

### 7.3 Thứ Postgres làm được mà Mongo làm rất khổ — ABC Report

`saas-management-reports/src/services/abcReport/abcReport.service.ts:66-115`:
```sql
WITH product_revenue AS (
    SELECT product_code, product_name,
           COALESCE(NULLIF(product_category_code, ''), 'UNKNOWN') AS product_category_code,
           SUM(quantity) AS quantity,
           SUM(ROUND(price::numeric) * quantity) AS revenue
    FROM order_details
    WHERE order_uuid = ANY($1::text[])
    GROUP BY product_code, product_name, ...
),
total_calc AS (
    SELECT *,
           SUM(revenue) OVER (PARTITION BY product_code) AS group_revenue_by_product,
           SUM(revenue) OVER ()                          AS total_revenue
    FROM product_revenue
)
SELECT product_code, quantity, revenue,
       SUM(revenue) OVER (ORDER BY group_revenue_by_product DESC, product_code ASC)
         AS cumulative_revenue,
       ROUND((revenue / NULLIF(total_revenue,0)) * 100, 2) AS individual
FROM total_calc;
```

| Kỹ thuật | Vai trò |
|---|---|
| **CTE** (`WITH`) | Chia query thành bước. Từ PG12 mặc định `NOT MATERIALIZED` nếu dùng 1 lần → planner inline được |
| **`SUM() OVER (ORDER BY ...)`** | **Running total** không cần self-join. Mongo phải `$setWindowFields` (5.0+) hoặc kéo về app tính bằng JS |
| **`SUM() OVER ()`** frame rỗng | Tổng toàn bảng đặt cạnh từng dòng — không cần query thứ 2 |
| **`PARTITION BY`** | Chia nhóm tính riêng trong 1 lượt quét |
| **`NULLIF(x,0)`** | Chống chia 0 → trả NULL thay vì lỗi |
| **`= ANY($1::text[])`** | Truyền mảng làm tham số, tránh build `IN (...)` bằng string |

| | MongoDB Aggregation | PostgreSQL |
|---|---|---|
| Running total | `$setWindowFields`, cú pháp rườm rà | `SUM() OVER (ORDER BY)` — 1 dòng |
| JOIN | `$lookup` — nested loop, không có hash join | Hash/Merge/Nested-loop, planner tự chọn |
| Giới hạn RAM | 100MB/stage, phải `allowDiskUse` | `work_mem` cấu hình được, spill tự động |
| Planner | Hạn chế | Cost-based, `EXPLAIN ANALYZE` chi tiết |
| BI tool | Phải viết pipeline JS | SQL — cắm được mọi tool |

### 7.4 Đánh đổi phải thừa nhận
- **Eventual consistency** giữa Mongo và Postgres — report có độ trễ so với vận hành.
- Phải **build và vận hành ETL**.
- **Hai hệ database** = hai bộ kỹ năng vận hành, hai chỗ backup, hai chỗ có thể hỏng.
- Không có transaction xuyên 2 hệ → cần **outbox pattern** hoặc idempotent ETL (repo chọn cách 2, bằng unique index).

---

<a name="8"></a>
<a name="8"></a>
## 8. Kỹ thuật Go đáng nói nhất

### 8.1 Bảng khảo sát (đã quét 12 service, trừ `vendor/`)

| Kỹ thuật | Lần | Service nổi bật |
|---|---|---|
| `gRPC` | 2153 | menu=440, order-pos=374, store=332 |
| `defer` | 625 | inventory=107, store=105 |
| `OpenTelemetry`/Jaeger | 471 | Tất cả 12 service |
| `chan` | 336 | menu=87, handlequeue-menu=64 |
| `go func` / goroutine | 329 | order-pos=101, inventory=51 |
| `BulkWrite` | 190 | inventory=74, handlequeue-inventory=41 |
| `context.WithTimeout` | 148 | inventory=61, store=19 |
| `sync.Once` | 136 | store=22, order-pos=20 |
| `sync.WaitGroup` | 83 | handlequeue-inventory=20, store=20 |
| `Aggregate` | 42 | order-pos=16, menu=7 |
| `select {}` | 40 | store=6, handlequeue-inventory=5 |
| `FindOneAndUpdate` | 55 | inventory=16, payment=9 |
| `sync.Mutex/RWMutex` | 15 | rải rác |
| `errgroup` | 9 | **order-pos=7**, tenant=2 |
| `context.WithCancel` | 13 | rải rác |
| Mongo Transaction | 2 | **chỉ menu** |
| `sync/atomic` | 2 | chỉ menu |
| `time.Ticker` | 1 | chỉ menu |

### 8.2 ⭐ MongoDB Change Streams — kỹ thuật xịn nhất repo

**Ở đâu:** `saas-microservice-handlequeue-inventory/controllers/retailInventory.go:274-302` (`WatchQuantityUpdate`)
và `controllers/inventory.go:551-564` (`WatchStockUpdate`).

**Nghiệp vụ:** thông báo "hàng đã có lại" (back-in-stock) cho khách trong waiting list.

```go
pipeline := mongo.Pipeline{
    {{Key: "$match", Value: bson.D{
        {Key: "$or", Value: bson.A{
            bson.D{{Key: "operationType", Value: "update"}},
            bson.D{{Key: "operationType", Value: "insert"}},
        }},
    }}},
}
opts := options.ChangeStream().
    SetFullDocument(options.UpdateLookup)     // ← lấy document ĐẦY ĐỦ sau update,
                                              //   không chỉ delta
changeStream, err := models.NewRetailInventoryModel(ctx).Collection.Watch(ctx, pipeline, opts)
defer changeStream.Close(ctx)

for changeStream.Next(ctx) {
    select {
    case <-ctx.Done(): return          // graceful shutdown
    default:
    }
    var changeDoc ChangeDocument
    changeStream.Decode(&changeDoc)
    ...
}
```

**Data trong change event:**
```go
type ChangeDocument struct {
    OperationType string                    // "insert" | "update"
    ID   struct{ Data string }              // resume token
    DocumentKey struct{ ID string }         // _id của document
    FullDocument struct {                   // ← nhờ SetFullDocument(UpdateLookup)
        ClientUuid, ProductUuid, VariantUuid, LocationUuid, StoreUuid string
        Quantity int
    }
    UpdateDescription struct {
        UpdatedFields struct{ Quantity int }   // chỉ field thay đổi
    }
}
```

**Luồng đầy đủ — 6 bước, chạm 4 hệ thống:**
```
retail_inventory.quantity thay đổi
  → Change Stream bắt được (đọc từ oplog của replica set)
  → EventLock.Insert()                       ← chống xử lý trùng (nhưng HỎNG, xem mục 9)
  → gRPC customer-service: GetCustomerWaitingList(client, store, product, variant, qty)
  → gRPC system-service: Detail(client_uuid)  ← lấy config kênh thông báo + ngôn ngữ
  → RabbitMQ: publish vào queue email HOẶC line (tuỳ client.WaitingListAutomationMethod)
     data: {client_uuid, to, template_code, language, parameters:{customer_name, product_list}}
  → gRPC customer-service: UpdateHistoryLog(..., "SEND_MAIL")
```

**Lý thuyết phải nói được:**
- Change Streams đọc từ **oplog** của replica set → **bắt buộc phải có replica set**, standalone không dùng được.
- **Resume token** (`changeDoc.ID.Data`) cho phép resume sau khi restart bằng `SetResumeAfter()`.
  → **Repo KHÔNG lưu resume token** → restart pod là mất hết event trong lúc down.
- `SetFullDocument(UpdateLookup)` làm thêm **1 lần đọc document** cho mỗi event — tốn hơn, nhưng cần vì `updateDescription` chỉ có delta.
- `$match` đặt trong pipeline → **lọc ở phía server**, giảm băng thông. Đúng cách.

**So sánh với RabbitMQ — câu hỏi phỏng vấn hay:**

| | Change Stream | Publish event từ code |
|---|---|---|
| Không bỏ sót | ✅ Bắt mọi thay đổi, kể cả sửa tay trong DB | ❌ Quên gọi publish là mất event |
| Coupling | Gắn chặt với schema DB | Gắn với contract nghiệp vụ |
| Ngữ nghĩa | Chỉ biết "quantity đổi từ 0→5" | Biết "hàng về do nhập kho PO#123" |
| Scale | Mỗi pod nhận **mọi** event → cần dedup | Broker tự phân phối |
| Resume | Cần tự lưu resume token | Broker lo |

> Trả lời khéo: *"Change Stream đảm bảo không bỏ sót — kể cả khi có người sửa DB bằng tay.
> Nhưng nó gắn chặt với schema và mất ngữ cảnh nghiệp vụ. Với back-in-stock thì hợp lý vì
> điều kiện kích hoạt thuần tuý là 'quantity từ 0 lên > 0'. Với các nghiệp vụ cần ngữ cảnh
> thì em vẫn dùng domain event qua RabbitMQ."*

### 8.3 `errgroup` — 2 cách dùng, 1 đúng 1 sai

**Cách ĐÚNG** — `controllers/driverAppController/driverApp.go:168-182`
**API:** `GET /v1/delivery-app/order/:uuid` (app tài xế giao hàng)
```go
eg, gCtx := errgroup.WithContext(c.Request.Context())
orderAction := NewOrderHelper(gCtx, clientUUID, orderUUID)

goroutineCtx := context.WithoutCancel(ctx)
eg.Go(orderAction.getOrderDetail)
eg.Go(func() error { return orderAction.getPayments(goroutineCtx)    })
eg.Go(func() error { return orderAction.getLogTime(goroutineCtx)     })
eg.Go(func() error { return orderAction.getOrderTaxRate(goroutineCtx)})
if err := eg.Wait(); err != nil { ... }
```
4 truy vấn độc lập chạy song song → latency = max(4) thay vì sum(4). Chuẩn.

> **Nhưng có nuance đáng nói:** dùng `errgroup.WithContext` (để có cancel lan truyền) rồi lại
> truyền `context.WithoutCancel(ctx)` vào goroutine → **vô hiệu hoá chính cơ chế cancel đó**.
> Có lẽ để tránh lỗi `context canceled` nhiễu log, nhưng hệ quả là 1 goroutine lỗi thì 3 cái kia
> vẫn chạy tới cùng, tốn tài nguyên vô ích. `context.WithoutCancel` là API Go 1.21+.

> **Bug phụ:** cùng file, `orderOpts := options.FindOne(); orderOpts.SetProjection(bson.M{...})`
> được xây với 22 field nhưng **không bao giờ truyền vào `FindOne`** → projection không áp dụng,
> vẫn kéo về document đầy đủ. Dead code.

**Cách SAI** — `controllers/splitBill.go:43-190`
**API:** `POST /v1/split-bill` (tách hoá đơn khi nhiều người trả riêng)
```go
var g errgroup.Group                    // ⚠ KHÔNG dùng WithContext
for _, bill := range req.SplitBill {
    g.Go(func() error { /* insert SplitBill      */ })
    g.Go(func() error { /* insertMany SplitBillItem      */ })
    g.Go(func() error { /* insertMany SplitBillPromotion */ })
    g.Go(func() error { /* insert SplitBillTable         */ })
    g.Go(func() error { /* update Order is_split_bill=1  */ })
    if err := g.Wait(); err != nil {    // ⚠ Wait() BÊN TRONG vòng lặp
        c.JSON(http.StatusBadRequest, respond.CreatedFail())
        return
    }
}
```

**Bốn vấn đề — bàn được rất sâu:**

1. **Không có transaction.** 5 lệnh ghi vào 5 collection song song. Nếu cái thứ 3 lỗi,
   cái 1, 2, 4, 5 **đã commit rồi** → dữ liệu nửa vời: hoá đơn tách tồn tại nhưng thiếu item.
   Và `clearData()` chạy **trước** đó đã `DeleteMany` sạch dữ liệu cũ → **mất luôn bản gốc**.
   Đúng ra phải dùng `session.WithTransaction` (Mongo 4.0+ hỗ trợ multi-document transaction trên replica set).

2. **`errgroup.Group` không có `WithContext`** → 1 goroutine lỗi, 4 cái kia vẫn chạy tới cùng.
   `errgroup.WithContext` sẽ cancel ctx chung để các goroutine khác dừng sớm.

3. **`g.Wait()` bên trong vòng lặp** → các `bill` xử lý **tuần tự**, chỉ song song trong 1 bill.
   Và tái sử dụng cùng một `errgroup.Group` sau `Wait()` là dùng sai ý đồ thiết kế.

4. **Không giới hạn concurrency.** `errgroup` có `SetLimit(n)` — nếu request có 50 bill thì
   sẽ bung 250 goroutine đập vào Mongo cùng lúc, dễ cạn connection pool (`MaxPoolSize: 50`).

> **Loop variable:** `for _, bill := range ... { g.Go(func(){ ... bill ... }) }` —
> code này **an toàn** vì `go.mod` khai `go 1.25.5`, mà từ **Go 1.22** biến vòng lặp là
> per-iteration. Nếu là Go ≤1.21 thì đây là bug kinh điển: mọi goroutine thấy giá trị cuối cùng.
> **Biết phân biệt mốc 1.22 này là điểm cộng rất lớn trong phỏng vấn Go.**

### 8.4 `sync.Once` — singleton connection (136 lần)

```go
// saas-microservice-order-pos/database/database.go
var (
    db   *mongo.Database
    once sync.Once
)
func New() *mongo.Database {
    once.Do(func() {
        opts := options.Client()
        opts.SetMaxPoolSize(50)
        opts.SetMinPoolSize(10)
        opts.SetMaxConnIdleTime(30 * time.Second)
        opts.SetMonitor(initJaeger())          // OpenTelemetry hook vào driver
        client, err := mongo.Connect(opts)
        if err != nil { log.Fatalln(err.Error()) }
        db = client.Database(database)
    })
    return db
}
```
**Lý thuyết:** `sync.Once` đảm bảo khởi tạo **đúng 1 lần** kể cả khi nhiều goroutine gọi đồng thời
— và các goroutine gọi sau sẽ **block cho tới khi lần đầu xong** (khác với chỉ check `if db == nil`,
vốn có race). Bên trong dùng `atomic` + `Mutex`, rẻ hơn `Mutex` thuần cho đường đã khởi tạo.

**Connection pool:** `MaxPoolSize: 50, MinPoolSize: 10, MaxConnIdleTime: 30s`
→ pool là **per-process**. Chạy 10 pod = tối đa 500 connection tới Mongo. Cần đối chiếu với
`maxIncomingConnections` của Mongo.

### 8.5 🔴 "Read/write splitting" giả — bug kiến trúc quan trọng nhất

`saas-microservice-order-pos/database/database.go` có **2 hàm**:
```go
func New() *mongo.Database         { once.Do(func(){ ... }); return db }
func NewReadNode() *mongo.Database { onceRead.Do(func(){ ... }); return dbRead }
```
Và 21 model dùng nó:
```go
// saas-microservice-order-pos/models/order.go
orderCollection     = db.Collection("order")        // write
orderReadCollection = dbRead.Collection("order")    // read
```

**Nhưng đọc kỹ `NewReadNode()`:**
```go
host := cfg.GetString("database.host")       // ← CÙNG host
port := cfg.GetString("database.port")       // ← CÙNG port
user := cfg.GetString("database.username")   // ← CÙNG user
uri = fmt.Sprintf(`mongodb://%s:%s@%s:%s/?authMechanism=SCRAM-SHA-256`, ...)
opts.SetMaxPoolSize(50)
opts.SetMinPoolSize(10)
// KHÔNG có SetReadPreference(readpref.SecondaryPreferred())
// KHÔNG có ?readPreference= trong URI
// KHÔNG có host riêng cho read node
```

**→ `NewReadNode()` tạo ra connection pool THỨ HAI tới CHÍNH primary đó, với read preference mặc định là `primary`.**

**Hậu quả:**
- **Không có chút read-scaling nào.** Mọi read vẫn đập vào primary.
- **Gấp đôi connection**: mỗi pod giữ 100 connection thay vì 50. 10 pod = 1000 connection.
- Tăng độ phức tạp code (21 model phải phân biệt 2 collection) mà **không thu được lợi ích gì**.

**Cách sửa đúng:**
```go
opts.SetReadPreference(readpref.SecondaryPreferred())
// hoặc trong URI: ?readPreference=secondaryPreferred&maxStalenessSeconds=90
```

> **Đây là câu trả lời TUYỆT VỜI cho "bạn từng phát hiện vấn đề kiến trúc nào?"**
> Và nhớ nói tiếp về **trade-off**: đọc từ secondary là **eventual consistency** —
> có replication lag. Với danh sách đơn hàng lịch sử thì chấp nhận được;
> nhưng với read-after-write (vừa tạo đơn, đọc lại ngay) thì **sẽ đọc trượt**.
> Nên cần phân loại: read nào chịu được stale, read nào phải từ primary.
> `maxStalenessSeconds` giúp giới hạn độ trễ tối đa chấp nhận.

### 8.6 Aggregation pipeline — thứ tự stage quyết định hiệu năng

**SAI** — `saas-microservice-order-pos/models/order.go` `Pagination()`
**API:** `GET /v1/orders`
```go
pipeline := []bson.M{
    {"$match":  conditions},
    {"$lookup": {from: "order_detail",  localField: "uuid", ...}},   // ⚠ lookup TRƯỚC
    {"$lookup": {from: "order_status",  ...}}, {"$unwind": "$orderstatus"},
    {"$lookup": {from: "order_type",    ...}}, {"$unwind": "$ordertype"},
    {"$lookup": {from: "order_device",  ...}}, {"$unwind": "$orderdevice"},
    {"$sort":   {"order.order_time": 1}},
    {"$skip":   offset},
    {"$limit":  limit},                                              // ⚠ limit CUỐI
}
```
→ Nếu `$match` trả về 50.000 đơn: Mongo làm **4 lookup × 50.000 document**, rồi sort 50.000,
rồi **vứt đi 49.980** để lấy 20 dòng. Lãng phí khủng khiếp.

**ĐÚNG** — cùng file, hàm `FindOneAggregate()`:
```go
pipeline := []bson.M{
    {"$match": conditions},
    {"$sort":  {"order_time": -1}},
    {"$limit": 1},                    // ← thu hẹp TRƯỚC
    {"$lookup": {from: "order_detail", ...}},
}
```

**Nguyên tắc vàng:** `$match` → `$sort` → `$skip`/`$limit` → `$lookup` → `$project`.
Thu hẹp tập dữ liệu **sớm nhất có thể**, join **muộn nhất có thể**.

**Hai bug nữa trong `Pagination()`:**

1. **Sort key không tồn tại.** `{"$sort": {"order.order_time": 1}}` — field là `order.order_time`
   (lồng), nhưng document `Order` có `order_time` ở **top-level**. Sort trên field không tồn tại
   → mọi document bằng nhau → **thứ tự tuỳ ý** → phân trang không ổn định
   (một đơn có thể xuất hiện ở cả trang 1 và trang 2, hoặc biến mất).

2. **`$unwind` không có `preserveNullAndEmptyArrays: true`.** Nếu `order_status` lookup
   trả mảng rỗng (status bị xoá) → `$unwind` **loại document đó khỏi kết quả**.
   → Đơn hàng **biến mất im lặng** khỏi API. Rất khó debug.

3. **Offset pagination** (`$skip`) — deep pagination: `$skip 100000` buộc Mongo đếm qua
   100.000 document. Giải pháp: **cursor/keyset pagination** —
   `{order_time: {$lt: lastSeenTime}}` + `$limit`.

### 8.7 Bug logic trong chính `GET /v1/orders`

```go
if req.FromDate != "" {
    cond["collection_time"] = bson.M{"$gte": fromDate}
}
if req.ToDate != "" {
    cond["collection_time"] = bson.M{"$lte": toDate}    // ⚠ GHI ĐÈ $gte!
}
```
Truyền cả `from_date` và `to_date` → điều kiện `$gte` **bị thay thế hoàn toàn**,
chỉ còn `$lte`. → API trả về **mọi đơn từ đầu đến `to_date`**, bỏ qua `from_date`.

**Sửa:**
```go
timeRange := bson.M{}
if req.FromDate != "" { timeRange["$gte"] = fromDate }
if req.ToDate   != "" { timeRange["$lte"] = toDate   }
if len(timeRange) > 0 { cond["collection_time"] = timeRange }
```

### 8.8 Các kỹ thuật Go khác đáng nhắc

| Kỹ thuật | Ở đâu | Ghi chú |
|---|---|---|
| **Graceful shutdown** | `handlequeue-inventory/main.go` | `signal.Notify(termChan, SIGINT, SIGTERM)` + `chan os.Signal` buffered size 1 (bắt buộc, nếu unbuffered có thể miss signal) |
| **Build tag** | `queue_number_counter_integration_test.go:1` | `//go:build integration` → tách integration test khỏi `go test ./...` |
| **`defer` cho cleanup** | 625 lần | `defer cancel()`, `defer changeStream.Close(ctx)`, `defer ch.Close()`, `defer wg.Done()` |
| **`context.WithTimeout`** | 148 lần | `ctx, cancel := context.WithTimeout(ctx, 10*time.Second); defer cancel()` — chống goroutine leak |
| **`BulkWrite`** | 190 lần | Gộp nhiều thao tác 1 round-trip. `ordered: false` cho phép song song |
| **OpenTelemetry vào driver** | `opts.SetMonitor(initJaeger())` | Trace **từng câu Mongo** trong Jaeger — rất mạnh để tìm query chậm |
| **`recover()` trong ES search** | `menu/service/elasticsearchService/es.go:83` | ⚠ Nuốt panic và trả `err = nil` → che giấu sự cố |
| **Mongo Transaction** | Chỉ 2 lần, chỉ trong `menu` | Hầu như không dùng — đây là điểm yếu (xem `splitBill.go`) |

---

<a name="9"></a>
## 9. Bug thật để kể trong phỏng vấn

> Xếp theo mức độ ấn tượng. Chọn 2-3 cái kể, đừng kể hết.

### 🥇 #1 — `event_lock` không khoá gì cả (distributed lock hỏng)

**File:** `saas-microservice-handlequeue-inventory/models/event_lock.go` + `controllers/retailInventory.go:384`

**Ý đồ:** `WatchQuantityUpdate` chạy trên **mọi pod**. Mỗi pod đều nhận cùng một change stream event.
`EventLock` được dùng để đảm bảo **chỉ 1 pod** gửi email thông báo.

```go
lock := models.EventLock{
    Id:         eventID,          // resume token của change event
    Instance:   instance,         // hostname
    Collection: "retail_inventory",
    CreatedAt:  time.Now(),
}
_, err = lock.Insert()
if err != nil {
    continue      // ← kỳ vọng: pod khác đã chiếm lock, bỏ qua
}
// ... gửi email
```

**Vì sao hỏng:**
```go
type EventLock struct {
    Id string `json:"id" bson:"id"`    // ← field tên "id", KHÔNG PHẢI "_id"
    ...
}
func (model *EventLock) Insert() (interface{}, error) {
    coll := db.Collection("event_lock")
    resp, err := coll.InsertOne(context.TODO(), model)   // ← luôn thành công
    return resp, err
}
```
1. Field là `id`, không phải `_id` → Mongo tự sinh `_id` ObjectID **mới toanh** mỗi lần insert.
2. **Không có unique index trên `id`** — `main.go` `createDatabaseIndexes()` chỉ tạo index cho
   `ProductBatch` và `ProductBatchLog`.
3. → `InsertOne` **luôn thành công** → `err == nil` → **mọi pod đều đi qua và cùng gửi email**.

**Hậu quả thật:** chạy 3 pod → khách hàng nhận **3 email giống hệt nhau** cho 1 lần hàng về.

**Sửa:**
```go
// Cách 1: dùng _id làm khoá tự nhiên
type EventLock struct {
    ID string `bson:"_id"`     // resume token làm _id → unique sẵn
    ...
}
// InsertOne lần 2 sẽ trả E11000 DuplicateKey → pod đó bỏ qua. Đúng ý đồ.

// Cách 2: giữ field `id` nhưng tạo unique index + TTL
{Keys: bson.D{{Key: "id", Value: 1}}, Options: options.Index().SetUnique(true)},
{Keys: bson.D{{Key: "created_at", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(3600)},

// Rồi kiểm tra ĐÚNG loại lỗi:
if _, err := lock.Insert(); err != nil {
    if mongo.IsDuplicateKeyError(err) { continue }   // pod khác đã chiếm
    return err                                       // lỗi thật
}
```
**Thiếu nữa:** không có TTL index → collection `event_lock` phình vô hạn.

> **Điểm mấu chốt để nói:** *"Cùng codebase này có một chỗ làm ĐÚNG y hệt pattern đó —
> `queue_number_counter.go` — với unique index, TTL, và fail-fast khi index không tạo được.
> Còn `event_lock` thì thiếu cả ba. Cùng ý tưởng, một cái đúng một cái sai."*
> Việc bạn **đối chiếu được 2 chỗ trong cùng repo** cho thấy bạn đọc code có hệ thống.

### 🥈 #2 — Read/write splitting giả
Xem [mục 8.5](#8). Gấp đôi connection, zero lợi ích.

### 🥉 #3 — Worker pool Ack kể cả khi lỗi → mất message
Xem [mục 6.5](#6). Cộng với việc **toàn repo không có DLQ nào**.

### #4 — Date range filter bị ghi đè
`GET /v1/orders`: `to_date` ghi đè `from_date`. Xem [mục 8.7](#8).

### #5 — `$lookup` trước `$limit`
4 lookup trên toàn bộ tập match rồi mới lấy 20 dòng. Xem [mục 8.6](#8).

### #6 — SQL Injection
`saas-management-order/src/services/order_sales/orderSales.service.ts:13-27`
```ts
WHERE store_uuid = '${params.store_uuid}'        // string interpolation
  AND client_uuid = '${params.client_uuid}'
```
Trong khi cùng repo, `abcReport.service.ts` lại dùng `$1, $2` đúng chuẩn.

### #7 — `business_date::date` non-sargable
`saas-management-reports`: cột `business_date` kiểu `text`, query `business_date::date BETWEEN ...`
→ ép kiểu trên cột làm index vô hiệu → **Seq Scan**. Mà bảng `orders` trong Postgres
**không khai báo index nào**. Sửa: đổi cột sang `date`, hoặc expression index, hoặc so sánh dạng string
(chạy được vì `YYYY-MM-DD` sort lexicographic = sort theo ngày).

### #8 — `$unwind` không `preserveNullAndEmptyArrays` → đơn hàng biến mất
### #9 — `recover()` nuốt panic và trả `err = nil` (ES search)
### #10 — `orderOpts` projection xây rồi không dùng (dead code)

---

<a name="10"></a>
## 10. Bộ câu hỏi + trả lời mẫu (Golang middle)

### Q1. "Compound index hoạt động thế nào? Thứ tự field có quan trọng không?"
→ **Prefix rule** + **ESR**. Rồi lấy ví dụ thật: collection `order` có 13 index đơn, 0 compound,
trong khi `GET /v1/orders` filter đồng thời 5 field và sort 1 field.
Giải thích: **Mongo chỉ dùng được 1 index cho 1 query** → 13 index đơn ≠ 1 compound.
Đưa ra index đề xuất `{client_uuid, is_delete, is_active, order_status_uuid, collection_time:-1}`
và cách verify bằng `explain("executionStats")`.

### Q2. "Làm sao sinh số tuần tự an toàn khi có nhiều pod?"
→ `queue_number_counter.go`. `FindOneAndUpdate` + `$inc` + `upsert` + **unique compound index**
+ retry `IsDuplicateKeyError`. Giải thích: Mongo serialize thao tác trên **single document**,
nên `$inc` là atomic; nhưng nếu **không có unique index**, 2 upsert đồng thời khi document chưa
tồn tại có thể tạo **2 document** → 2 khách cùng số.
Kể luôn có **test `Concurrent100`** verify không trùng, không thiếu trong [1..100].

### Q3. "Kể một bug concurrency bạn từng gặp."
→ `event_lock` ([mục 9 #1](#9)). Kể đủ: ý đồ (chỉ 1 pod gửi email) → cơ chế (insert lock)
→ vì sao hỏng (field `id` ≠ `_id`, không unique index) → hậu quả (khách nhận N email)
→ cách sửa (2 phương án) → và **đối chiếu với `queue_number_counter` làm đúng trong cùng repo**.

### Q4. "`errgroup` khác gì `sync.WaitGroup`?"
→ `WaitGroup` chỉ đợi; **không thu error, không cancel**. `errgroup`:
(a) `Wait()` trả về **error đầu tiên**; (b) `WithContext` **cancel ctx chung** khi có lỗi
→ goroutine khác dừng sớm; (c) `SetLimit(n)` giới hạn concurrency.
Rồi kể `splitBill.go` dùng `errgroup.Group` **không có** `WithContext`, `Wait()` **bên trong** vòng lặp,
và **không có transaction** cho 5 lệnh ghi vào 5 collection.

### Q5. "Go 1.22 thay đổi gì về loop variable?"
→ Trước 1.22: biến vòng lặp là **một** biến dùng lại → goroutine capture bằng reference
→ tất cả thấy giá trị cuối. Từ 1.22: **per-iteration**, mỗi vòng một biến mới.
Rồi nói: *"Code `splitBill.go` capture `bill` trong `g.Go(func(){...})` — nhìn thì giống bug kinh điển,
nhưng `go.mod` khai `go 1.25.5` nên an toàn. Em vẫn phải check `go.mod` mới kết luận được."*

### Q6. "Một message RabbitMQ có thể vào nhiều queue không?"
→ **Có** — khi ≥2 binding cùng khớp, exchange **copy**. Phân biệt rõ với **nhiều consumer trên 1 queue**
= round-robin, mỗi message 1 consumer.
Rồi kể `audit_log_exchange`: hiện tại 4 binding loại trừ nhau nên chưa xảy ra, nhưng chọn `topic`
là để mở đường — thêm queue bind `#` là có ngay fan-out mà không sửa producer.
Kể tiếp 2 bug: `payment.*`/`promotion.*` không ai bind → drop im lặng; `OSS_AUTH`/`OSS_MENU`
bind mà không có consumer.

### Q7. "Hệ thống đảm bảo message không mất thế nào?"
→ Chuỗi đầy đủ: `durable` exchange/queue → `persistent` message → `Qos` prefetch → manual `Ack`.
Rồi **tự nêu lỗ hổng**: (a) worker pool **Ack kể cả khi handler lỗi** → mất message;
(b) **không có DLQ** ở bất kỳ đâu; (c) **không có Publisher Confirms**.
Đưa cách sửa cụ thể: `Nack(false, false)` + `x-dead-letter-exchange`.

### Q8. "At-least-once delivery là gì? Xử lý sao?"
→ Manual ack → consumer chết trước khi ack thì message được giao **lại**.
→ Hệ quả: handler **phải idempotent**.
→ Repo giải bằng: unique compound index (`uniq_day_client_store`, `idx_order_details_uniq`),
`upsert: true` (29 chỗ TS), `FindOneAndUpdate` (55 chỗ Go), và `hookTracking{order_uuid, status_code}`
để chống webhook trùng.
→ Chốt: *"exactly-once **delivery** là bất khả thi trong hệ phân tán; cái đạt được là
exactly-once **processing** = at-least-once + idempotency."*

### Q9. "Change Stream là gì? So với publish event từ code thì sao?"
→ Đọc từ **oplog** của replica set. Bảng so sánh ở [mục 8.2](#8).
Kể luồng back-in-stock: change stream → lock → gRPC ×2 → RabbitMQ → gRPC.
Nêu điểm thiếu: **không lưu resume token** → restart pod là mất event trong lúc down.

### Q10. "Tại sao dùng 2 database?"
→ **Polyglot persistence**: OLTP (Mongo) vs OLAP (Postgres). Nhấn mạnh **cách ly tải**.
Bằng chứng: `allowDiskUse` 10 lần → đã chạm trần 100MB aggregation của Mongo.
Thừa nhận cái giá: eventual consistency, phải build ETL, vận hành 2 hệ, không có transaction xuyên hệ.

### Q11. "Bạn phát hiện vấn đề kiến trúc nào chưa?"
→ **Read/write splitting giả** ([mục 8.5](#8)). `NewReadNode()` build URI y hệt `New()`,
không có `SetReadPreference` → pool thứ 2 tới chính primary. Gấp đôi connection, zero lợi ích.
Rồi nói trade-off của việc sửa đúng: secondary = eventual consistency, cần phân loại
read nào chịu được stale, dùng `maxStalenessSeconds` để giới hạn.

### Q12. "Cache invalidation làm thế nào?"
→ Kể **tag-based invalidation** trên ES cache ([mục 4.2](#4)): mỗi entry lưu kèm mảng
`product_uuids` / `store_uuids`; service khác sửa sản phẩm → gọi gRPC `DeleteCacheByProductUuids`
→ `DeleteByQuery` xoá đúng những entry liên quan. Chính xác hơn TTL nhiều.
Rồi nêu **partial cache**: tồn kho không cache, luôn query tươi rồi ghép vào response đã cache.
Rồi nêu nhược điểm: không có TTL (`created_at` lưu `keyword` nên không range được),
`refresh=true` mỗi lần ghi rất đắt với Lucene, không có eviction policy.
Chốt bằng 3 vấn đề kinh điển: **stampede** (fix: lock hoặc TTL jitter),
**penetration** (fix: cache null / bloom filter), **invalidation**.

---

## 11. File cần mở lại khi ôn

| Chủ đề | File |
|---|---|
| ⭐ Atomic counter + unique index + retry | `saas-microservice-store/models/queue_number_counter.go` |
| ⭐ Test concurrency (WaitGroup + buffered chan) | `saas-microservice-store/models/queue_number_counter_integration_test.go` |
| ⭐ API dùng counter | `saas-microservice-store/controllers/queueNumber.go` |
| ⭐ Change Streams + luồng back-in-stock | `saas-microservice-handlequeue-inventory/controllers/retailInventory.go:274-500` |
| ⭐ ES cache: key normalize, partial cache | `saas-microservice-menu/controllers/productAA.go:38-280` |
| ⭐ ES cache: Get/Set/tag invalidation | `saas-microservice-menu/service/elasticsearchService/cache.go` |
| ⭐ gRPC endpoint invalidate cache | `saas-microservice-menu/grpc/service/product.go:39-44` |
| 🐛 Distributed lock hỏng | `saas-microservice-handlequeue-inventory/models/event_lock.go` |
| 🐛 Read/write split giả | `saas-microservice-order-pos/database/database.go:79-125` |
| 🐛 13 index đơn, 0 compound | `saas-microservice-order-pos/models/order.go` (init) |
| 🐛 `$lookup` trước `$limit` + sort key sai | `saas-microservice-order-pos/models/order.go` `Pagination()` |
| 🐛 Date range bị ghi đè | `saas-microservice-order-pos/controllers/orderPos.go:1757+` `List()` |
| 🐛 Worker pool Ack khi lỗi | `saas-microservice-system/service/queue/amqp.go` `worker()` |
| errgroup đúng | `saas-microservice-order-pos/controllers/driverAppController/driverApp.go:168` |
| errgroup sai + không transaction | `saas-microservice-order-pos/controllers/splitBill.go:43-190` |
| Topic exchange + 4 binding + TTL | `saas-microservice-system/service/queue/auditLogSetup.go` |
| Publisher Go + payload order | `saas-microservice-order-pos/services/orderIntegrate/publishMessage.go` |
| 8 queue / 8 goroutine + graceful shutdown | `saas-microservice-handlequeue-inventory/main.go` |
| Singleton + pool + Jaeger monitor | `saas-microservice-order-pos/database/database.go:31-77` |
| SQL window function + CTE | `saas-management-reports/src/services/abcReport/abcReport.service.ts:66-115` |
| Star schema Postgres | `saas-management-reports/src/models/order.postgre.ts` |

---

## 12. Sơ đồ vẽ bảng trắng

```
                    ┌──────────────────────────────┐
   HTTP ───────────►│  Gin API services (Go)       │
                    │  order-pos, menu, store, ...  │
                    └──┬────────┬─────────┬────────┘
          gRPC (sync)  │        │         │  AMQP (async)
     ┌─────────────────┘        │         └──────────────┐
     ▼                          ▼                        ▼
┌──────────┐            ┌──────────────┐         ┌────────────────┐
│Elastic-  │            │  MongoDB     │         │   RabbitMQ     │
│search    │            │  replica set │         │                │
│          │            │              │         │ audit_log_ex   │
│ index    │            │ 121 idx (TS) │         │ ORDER_POS      │
│ "cache"  │◄──tag──────┤ 14  idx (Go) │         │ OSS-PRODUCT    │
│ product_ │ invalidate │ TTL, unique  │         │ OSS-INVENTORY  │
│ aa       │            │              │         │ ex.out_of_     │
│          │            │  Change      │         │  stock.<tid>   │
│ ❌ no TTL│            │  Streams ────┼────────►│ x-delayed-msg  │
└──────────┘            └──────┬───────┘         └───┬────────┬───┘
                               │                     │        │
                          ETL ─┘                     ▼        ▼
                               ▼              ┌───────────┐ ┌────────┐
                        ┌──────────────┐      │handlequeue│ │POS/KDS │
                        │ PostgreSQL   │      │-inventory │ │ client │
                        │ (OLAP)       │      │-menu      │ └────────┘
                        │ star schema  │      │(no HTTP)  │
                        │ window func  │      └───────────┘
                        │ 8 bảng "MV"  │
                        └──────────────┘

❌ KHÔNG CÓ: Redis · Dead Letter Queue · Publisher Confirms
            · Materialized View · Rate limiting · Resume token
🔴 HỎNG:    read/write splitting (giả) · event_lock (không unique index)
```
