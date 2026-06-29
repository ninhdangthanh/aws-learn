# Golang Core Interview Notes
## Theo CV Backend Go, Microservices, gRPC, RabbitMQ, MongoDB/PostgreSQL/Redis

File này tập trung vào các phần dễ bị hỏi khi CV ghi Golang là backend chính: interface, concurrency, context, memory, error handling, API/gRPC và production debugging. Bỏ qua cú pháp cơ bản như biến, function, struct literal.

---

## 1. Go trong backend production

Go hợp với backend service vì binary gọn, deploy dễ, concurrency rẻ, standard library mạnh cho HTTP/networking, và runtime đủ ổn cho service I/O-bound.

Trả lời tốt hơn "Go nhanh vì goroutine":

* Go giúp viết service concurrent đơn giản bằng goroutine, channel, `context`.
* Runtime scheduler quản lý nhiều goroutine trên ít OS thread.
* Static binary giúp deploy container/Lambda/ECS dễ hơn.
* Type system vừa đủ để code backend rõ contract, compile nhanh.

Trade-off:

* Error handling verbose.
* Không có exception/class inheritance truyền thống.
* Generic có nhưng không nên lạm dụng.
* GC có thể ảnh hưởng latency nếu allocation pattern xấu.

---

## 2. Interface

Interface trong Go là tập hợp method. Một type implement interface ngầm nếu có đủ method, không cần khai báo `implements`.

Điểm phỏng vấn hay hỏi:

* Interface nên được định nghĩa ở phía consumer, không nhất thiết ở package implementation.
* Interface nhỏ thường tốt hơn interface lớn.
* Dùng interface để tách dependency như repository, queue producer, external client, clock, logger.
* Không nên tạo interface cho mọi struct nếu chưa có nhu cầu test/mock hoặc nhiều implementation.

```go
type PaymentGateway interface {
    Charge(ctx context.Context, req ChargeRequest) (ChargeResult, error)
}
```

Câu trả lời tốt:

> Interface trong Go giúp service layer phụ thuộc vào behavior thay vì concrete implementation. Trong backend, tôi dùng nó để mock external service, đổi implementation queue/cache, hoặc tách domain khỏi framework. Nhưng tôi tránh tạo interface quá sớm vì có thể làm code phức tạp mà chưa có lợi ích thật.

### Empty interface và `any`

`interface{}` và `any` có nghĩa tương đương: nhận mọi kiểu. Dùng khi thật sự cần dữ liệu động như JSON, metadata, generic helper cũ.

Rủi ro:

* Mất type safety.
* Cần type assertion/type switch.
* Dễ panic nếu assertion sai.

```go
v, ok := input.(string)
if !ok {
    return errors.New("input must be string")
}
```

### Nil interface trap

Một interface chỉ nil khi cả dynamic type và dynamic value đều nil.

```go
var e *MyError = nil
var err error = e
fmt.Println(err == nil) // false
```

Cách tránh:

* Return `nil` trực tiếp nếu không có lỗi.
* Cẩn thận khi custom error là pointer receiver.

---

## 3. Pointer receiver vs value receiver

Dùng pointer receiver khi:

* Method cần mutate receiver.
* Struct lớn, copy tốn chi phí.
* Muốn nhất quán method set với các method pointer khác.
* Có field như `sync.Mutex`, không được copy sau khi dùng.

Dùng value receiver khi:

* Struct nhỏ, immutable-like.
* Method chỉ đọc dữ liệu.
* Value object như `Money`, `Coordinate`.

Interview answer:

> Với service/repository/client, tôi thường dùng pointer receiver vì object có dependency và không muốn copy. Với value object nhỏ, value receiver giúp code an toàn hơn.

---

## 4. Concurrency vs parallelism

* **Concurrency:** tổ chức chương trình để xử lý nhiều việc cùng lúc về mặt logic.
* **Parallelism:** thực thi nhiều việc cùng một thời điểm trên nhiều CPU core.

Go mạnh ở concurrency vì goroutine nhẹ và runtime scheduler quản lý việc chạy goroutine trên OS threads.

---

## 5. Goroutine và GMP scheduler

Go runtime dùng mô hình GMP:

* `G`: Goroutine, đơn vị công việc nhẹ.
* `M`: Machine, OS thread thực thi thật.
* `P`: Processor, logical processor giữ run queue.

Goroutine bắt đầu với stack nhỏ và có thể grow/shrink. Scheduler có work stealing để cân bằng tải giữa các `P`.

Không nên nói "goroutine không tốn tài nguyên". Câu đúng là:

> Goroutine rẻ hơn OS thread, nhưng không miễn phí. Nếu spawn theo từng request/job mà không giới hạn, vẫn có thể gây memory pressure, scheduler overhead, DB connection exhaustion hoặc dependency overload.

---

## 6. Channel

Channel dùng để giao tiếp giữa goroutine và đồng bộ hóa luồng dữ liệu.

Phân biệt:

* Unbuffered channel: send block cho tới khi có receiver.
* Buffered channel: send block khi buffer đầy.
* Receive từ closed channel trả zero value và `ok=false`.
* Send vào closed channel sẽ panic.
* Close channel nhiều lần sẽ panic.

Quy tắc thực tế:

* Sender thường là bên close channel.
* Không close channel nếu có nhiều sender mà không có coordinator.
* Không dùng channel chỉ vì "Go style"; mutex đôi khi rõ hơn.

### Channel close trong nhiều producer

Nếu nhiều goroutine cùng gửi vào một channel, nên có goroutine coordinator chờ `WaitGroup` rồi close.

```go
var wg sync.WaitGroup
for _, src := range sources {
    wg.Add(1)
    go func(src Source) {
        defer wg.Done()
        produce(src, out)
    }(src)
}

go func() {
    wg.Wait()
    close(out)
}()
```

---

## 7. Mutex vs channel

Dùng mutex khi:

* Bảo vệ shared state.
* Critical section nhỏ, logic đơn giản.
* Cần map/cache/counter in-memory.

Dùng channel khi:

* Cần pipeline, worker pool, fan-out/fan-in.
* Cần truyền ownership của data giữa goroutine.
* Cần serialize một luồng command vào một goroutine owner.

Interview answer:

> Channel không thay thế mutex. Nếu bài toán là bảo vệ state thì mutex thường rõ hơn. Nếu bài toán là phối hợp workflow hoặc truyền job/result thì channel hợp hơn.

---

## 8. Worker pool

Worker pool giới hạn số goroutine xử lý đồng thời để bảo vệ DB, Redis, API ngoài hoặc queue consumer.

```go
func worker(ctx context.Context, jobs <-chan Job, results chan<- Result) {
    for {
        select {
        case <-ctx.Done():
            return
        case job, ok := <-jobs:
            if !ok {
                return
            }
            results <- process(job)
        }
    }
}
```

Thành phần:

* `jobs` channel nhận việc.
* `results` channel trả kết quả nếu cần.
* `context.Context` để cancel.
* `sync.WaitGroup` hoặc `errgroup` để chờ worker kết thúc.

Lỗi thường gặp:

* Spawn goroutine vô hạn theo job.
* Không đóng jobs channel.
* Không drain results channel.
* Worker gọi dependency ngoài không timeout.
* Queue buffer quá lớn làm che giấu backpressure.

Metric nên expose:

* `queue_depth`
* `worker_busy`
* `job_latency`
* `job_error_total`
* retry/DLQ count

---

## 9. Pipeline, fan-out/fan-in và backpressure

Pipeline chia một quy trình lớn thành nhiều stage nối bằng channel.

```text
Input Files -> Parse -> Validate -> Transform -> Persist
```

Fan-out/fan-in:

* Fan-out: nhiều worker cùng đọc từ một input channel để tăng throughput.
* Fan-in: merge nhiều output channel thành một stream kết quả.

Backpressure:

* Nếu stage sau chậm, channel đầy và stage trước bị chậm theo.
* Đây là điểm tốt nếu muốn tự điều tiết tốc độ.
* Vẫn cần timeout/cancel để tránh treo toàn pipeline.

Nếu cần giữ thứ tự kết quả, phải gắn sequence number và sort lại ở aggregator.

---

## 10. Context

`context.Context` dùng để truyền cancellation, deadline, timeout và request-scoped values qua boundary.

Dùng trong:

* HTTP request lifecycle.
* DB query.
* gRPC call.
* RabbitMQ/Kafka consumer processing.
* Worker pool/pipeline.

```go
ctx, cancel := context.WithTimeout(parent, 2*time.Second)
defer cancel()

rows, err := db.QueryContext(ctx, query)
```

Best practices:

* Function nhận `ctx context.Context` ở argument đầu tiên.
* Không store context trong struct dài hạn.
* Luôn `defer cancel()` với context tạo ra.
* Không dùng context value cho optional params; chỉ dùng cho request id, trace id, auth info thật sự xuyên boundary.

Interview answer:

> Context giúp tránh goroutine leak và request treo khi client disconnect hoặc dependency chậm. Trong service có DB, Redis, gRPC, queue worker, tôi luôn truyền context xuống các call có thể block.

---

## 11. Goroutine leak

Goroutine leak xảy ra khi goroutine không bao giờ kết thúc dù request/job đã xong.

Nguyên nhân phổ biến:

* Chờ receive từ channel không bao giờ có dữ liệu.
* Send vào channel không có receiver.
* Không lắng nghe `ctx.Done()`.
* Ticker/timer không `Stop`.
* Worker không shutdown khi service stop.

Cách debug:

* `pprof/goroutine`.
* Metric số goroutine.
* Log lifecycle của worker.
* Test timeout/cancel path.

---

## 12. Error handling

Go ưu tiên error tường minh thay vì exception.

Best practices:

* Wrap lỗi có context: `fmt.Errorf("create order: %w", err)`.
* Dùng `errors.Is` với sentinel error.
* Dùng `errors.As` với custom error type.
* Không log cùng một lỗi ở quá nhiều tầng.
* Map lỗi domain sang HTTP status/gRPC code nhất quán.

```go
if errors.Is(err, ErrNotFound) {
    return nil, status.Error(codes.NotFound, "order not found")
}
```

### Panic/recover

Panic chỉ nên dùng cho lỗi lập trình hoặc trạng thái bất khả thi, không dùng như flow control.

Trong HTTP server, recovery middleware giúp process không crash vì một request. Nhưng sau recover vẫn cần log stack trace và trả error shape nhất quán.

---

## 13. Race condition và race detector

Race condition xảy ra khi nhiều goroutine truy cập cùng memory, ít nhất một ghi, không có đồng bộ hóa.

Cách xử lý:

* `sync.Mutex`
* `sync.RWMutex`
* `atomic`
* channel owner pattern
* tránh shared mutable state

Lệnh kiểm tra:

```bash
go test -race ./...
```

Trong phỏng vấn, nên liên hệ với inventory, wallet, session, cache in-memory, request counter.

---

## 14. Memory management

Go có GC, nhưng backend hiệu năng tốt vẫn cần hiểu allocation.

Khái niệm chính:

* Stack: local data, rẻ, goroutine stack grow/shrink.
* Heap: object sống vượt scope hoặc escape, chịu GC.
* Escape analysis: compiler quyết định biến nằm stack hay heap.
* GC pressure: nhiều allocation nhỏ, object sống lâu, slice giữ reference lớn.

Giảm allocation:

* Preallocate slice/map khi biết size gần đúng.
* Tránh convert `[]byte` <-> `string` trong hot path.
* Reuse buffer bằng `sync.Pool` cho object tạm lớn.
* Dùng benchmark `go test -bench -benchmem`.
* Dùng `pprof` để tìm allocation thật, không đoán.

Bug phổ biến:

```go
small := big[:10] // vẫn giữ reference tới underlying array lớn
```

Nếu cần giải phóng memory của `big`, copy phần nhỏ sang slice mới.

---

## 15. Slice và map internals

Slice gồm pointer tới array, length, capacity. Append có thể reuse underlying array hoặc allocate array mới.

Điểm hay hỏi:

* Pass slice vào function là copy header, underlying array vẫn share.
* Append có thể làm slice mới trỏ sang array mới.
* Sub-slice có thể giữ reference tới array lớn.

Map:

* Không an toàn cho concurrent read/write.
* Iteration order không ổn định.
* Nên preallocate nếu biết size.
* Dùng `sync.Map` cho case đặc biệt nhiều goroutine đọc/ghi với key độc lập; không phải thay thế mặc định cho map + mutex.

---

## 16. Defer

`defer` chạy khi function return, theo thứ tự LIFO.

Dùng cho:

* `cancel()`
* `rows.Close()`
* `file.Close()`
* `mu.Unlock()`
* tracing span end

Lưu ý:

* Argument của defer được evaluate ngay lúc gọi defer.
* Trong loop lớn/hot path, defer nhiều có thể tăng overhead hoặc delay resource release tới cuối function.

---

## 17. Generics

Generics giúp viết helper type-safe cho collection, cache wrapper, result type, repository helper. Nhưng backend Go không nên biến code thành framework phức tạp.

Dùng khi:

* Logic giống nhau trên nhiều type.
* Muốn giữ type safety thay vì `any`.
* Helper nhỏ, dễ đọc.

Tránh khi:

* Chỉ có một use case.
* Làm code khó debug.
* Ép domain logic vào abstraction quá sớm.

---

## 18. HTTP API trong Go

Các điểm dễ bị hỏi theo CV:

* Middleware: request id, auth, logging, recover, timeout.
* Validation: validate request ở boundary.
* Error shape: thống nhất `code`, `message`, `details`, `request_id`.
* Timeout: server timeout, handler context timeout, DB/Redis timeout.
* Graceful shutdown: stop nhận request mới, chờ request hiện tại, shutdown worker/queue.
* Idempotency: đặc biệt với payment/order/sync/offline POS.

Graceful shutdown cần:

* `http.Server.Shutdown(ctx)`.
* Stop consumer/worker.
* Close DB/Redis connection sau khi worker dừng.
* Có readiness probe chuyển fail trước khi shutdown nếu chạy Kubernetes.

---

## 19. gRPC trong Go

gRPC dùng HTTP/2 + Protocol Buffers, hợp cho internal service communication.

Điểm cần nắm:

* Unary vs streaming.
* Deadline/cancellation truyền qua context.
* Status code: `InvalidArgument`, `NotFound`, `AlreadyExists`, `FailedPrecondition`, `Unavailable`, `DeadlineExceeded`.
* Backward compatibility protobuf:
    * Không đổi field number.
    * Không reuse deleted field number.
    * Thêm field optional là hướng an toàn.
    * Reserve field đã xóa.
* Interceptor tương tự middleware: auth, logging, tracing, recover.

Trade-off:

* gRPC hiệu quả cho internal service, nhưng browser/client public thường REST dễ hơn.
* Debug thủ công khó hơn JSON nếu thiếu tooling.
* Schema versioning phải kỷ luật.

---

## 20. Go với RabbitMQ/queue worker

Điểm phỏng vấn thực tế:

* Ack sau khi xử lý thành công.
* Nack/requeue có kiểm soát, tránh retry storm.
* Prefetch giới hạn số message unacked mỗi consumer.
* DLQ cho poison message.
* Idempotent consumer vì message có thể bị xử lý lại.
* Context/timeout khi xử lý từng message.

Ví dụ câu trả lời:

> Với RabbitMQ consumer, tôi không ack ngay khi nhận message. Tôi xử lý xong, persist thành công, rồi mới ack. Nếu lỗi transient thì retry có backoff, nếu lỗi permanent hoặc quá số lần thì đưa vào DLQ. Consumer phải idempotent vì at-least-once delivery có thể tạo duplicate.

---

## 21. Go với database/cache

PostgreSQL/MySQL:

* Luôn truyền context vào query.
* Cấu hình connection pool theo capacity DB, không chỉ theo traffic app.
* Transaction ngắn, không gọi external API trong transaction nếu tránh được.
* Dùng optimistic/pessimistic lock cho inventory/wallet/order state.

Redis:

* Timeout ngắn.
* Cache-aside, TTL, invalidation.
* Chống cache stampede bằng singleflight/lock ngắn/randomized TTL.
* Cẩn thận hot key.

MongoDB:

* Context timeout cho query/aggregation.
* Index đúng query pattern.
* Tránh unbounded array/hot document.
* Cẩn thận transaction nhiều document vì chi phí cao hơn relational DB.

---

## 22. Testing, benchmark và profiling

Testing:

* Unit test domain/service với fake interface.
* Integration test DB/queue cho behavior quan trọng.
* Race test cho concurrency.
* Table-driven test cho nhiều case input/output.

Benchmark:

```bash
go test -bench=. -benchmem ./...
```

Profiling:

* CPU profile: hot function.
* Memory profile: allocation.
* Goroutine profile: leak/deadlock.
* Trace: scheduler/blocking analysis.

---

## 23. Câu hỏi phỏng vấn dễ gặp

### Interface trong Go khác gì Java/TypeScript?

Go implement interface ngầm theo method set. Không cần khai báo `implements`. Điều này giúp decoupling tự nhiên nhưng cũng cần đặt interface đúng chỗ, thường là phía consumer.

### Khi nào dùng channel, khi nào dùng mutex?

Mutex hợp để bảo vệ shared state. Channel hợp để truyền dữ liệu/job/result và phối hợp workflow. Channel không thay thế mutex trong mọi trường hợp.

### Làm sao tránh goroutine leak?

Truyền context, lắng nghe `ctx.Done()`, đóng channel đúng ownership, stop ticker/timer, giới hạn worker, và kiểm tra bằng goroutine profile/metrics.

### Vì sao cần context trong backend service?

Vì request có thể timeout/cancel, dependency có thể chậm, worker có thể cần shutdown. Context giúp hủy DB query, gRPC call, HTTP call và goroutine liên quan.

### Go có memory leak không nếu có GC?

Có thể có logical leak: goroutine không kết thúc, cache không eviction, ticker không stop, slice nhỏ giữ array lớn, reference còn sống quá lâu làm GC không thu được.

### Làm sao xử lý duplicate message trong RabbitMQ?

Thiết kế idempotent consumer: dùng unique key/dedup table, check trạng thái trước khi update, hoặc transaction bảo vệ side effect. Ack sau khi xử lý thành công.

### Go service bị latency spike thì debug gì?

Xem metrics request latency, DB/Redis latency, goroutine count, GC pause, CPU/memory, connection pool saturation. Sau đó dùng pprof CPU/memory/goroutine và trace/log theo request id.
