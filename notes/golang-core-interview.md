# Golang Core Interview Notes
## Theo CV Backend Go, Microservices, gRPC, RabbitMQ, MongoDB/PostgreSQL/Redis

File này tập trung vào các phần dễ bị hỏi khi CV ghi Golang là backend chính: interface, concurrency, context, memory, error handling, API/gRPC và production debugging. Bỏ qua cú pháp cơ bản như biến, function, struct literal.

### Cách dùng file này

Khác biệt giữa Junior và Middle không nằm ở việc **biết định nghĩa**, mà ở việc trả lời được **vì sao**, **khi nào**, **lỗi gì** và **debug thế nào**.

Junior: *"Worker pool dùng để giới hạn goroutine."*

Middle: *"Trong feature import Excel 200.000 dòng, nếu spawn 200.000 goroutine thì scheduler, DB connection pool và Redis đều quá tải. Tôi giới hạn 20 worker, queue job bằng channel, dùng context để cancel và WaitGroup để shutdown sạch."*

Vì vậy phần lớn các mục dưới đây đều có cùng một cấu trúc:

1. **Định nghĩa** — đủ để trả lời câu hỏi mở đầu.
2. **Production example** — hệ thống thật: order service, RabbitMQ consumer, Redis cache, import Excel.
3. **Trade-off / lỗi thường gặp** — khi nào dùng, khi nào không, và cái gì sẽ nổ.
4. **Cách debug** — pprof, metric, race detector.
5. **Interview follow-up** — những câu interviewer đào sâu sau khi bạn trả lời xong phần cơ bản.

Nếu thời gian ôn có hạn, ưu tiên các mục: Context (10), Goroutine leak (11), Worker pool (8), Slice/map internals (15), Race condition + memory model (13), và Production debugging playbook (22.7).

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

### Production example: payment gateway

Interviewer gần như luôn hỏi tiếp: "Em đã dùng interface ở project nào?". Trả lời bằng một dependency thật.

```text
OrderService
    |
    +-- PaymentGateway (interface, định nghĩa ở package service)
              |
      ------------------------
      |                      |
StripeGateway          FakeGateway (test)
```

```go
// package order — consumer định nghĩa interface theo nhu cầu của mình
type PaymentGateway interface {
    Charge(ctx context.Context, req ChargeRequest) (ChargeResult, error)
}

type OrderService struct {
    payment PaymentGateway
    repo    OrderRepository
}
```

Trong test không gọi Stripe thật:

```go
type FakeGateway struct {
    err error
}

func (f *FakeGateway) Charge(ctx context.Context, req ChargeRequest) (ChargeResult, error) {
    if f.err != nil {
        return ChargeResult{}, f.err
    }
    return ChargeResult{ID: "ch_test_1", Status: "succeeded"}, nil
}

func TestCreateOrder_PaymentFailed(t *testing.T) {
    svc := NewOrderService(&FakeGateway{err: ErrCardDeclined}, repo)
    _, err := svc.Create(ctx, req)
    require.ErrorIs(t, err, ErrCardDeclined)
}
```

Ví dụ thứ hai là cache. Cùng một `Cache` interface, đổi implementation theo môi trường mà service không sửa một dòng nào:

```go
type Cache interface {
    Get(ctx context.Context, key string) ([]byte, error)
    Set(ctx context.Context, key string, val []byte, ttl time.Duration) error
}
```

| Implementation | Dùng ở đâu |
| --- | --- |
| `RedisCache` | production |
| `MemoryCache` | local dev, integration test |
| `NoopCache` | benchmark, debug khi nghi cache sai |

### Trade-off

* **Lợi:** test không cần network, đổi vendor (Stripe → Adyen) chỉ thêm một implementation, domain không import SDK bên ngoài.
* **Hại:** thêm một tầng gián tiếp, đọc code phải nhảy qua interface mới thấy logic thật; mock quá nhiều dễ dẫn tới test "xanh" nhưng không phản ánh hành vi thật của dependency.
* **Quy tắc của tôi:** chỉ tạo interface khi có ít nhất một trong ba lý do: cần mock, có ≥2 implementation thật, hoặc cần cắt dependency direction (domain không được biết infra).

### Interview follow-up

* *Interface nên đặt ở package nào?* Phía consumer. Package `order` định nghĩa `PaymentGateway`, package `stripe` chỉ cần có đủ method — không import ngược.
* *Vì sao Go không có `implements`?* Implicit satisfaction cho phép định nghĩa interface **sau khi** concrete type đã tồn tại, kể cả type ở third-party package.
* *Interface lớn hay nhỏ tốt hơn?* Nhỏ. `io.Reader` chỉ một method nên compose được ở mọi nơi. Interface 10 method thì mock nào cũng phải implement đủ 10.
* *Interface có tốn chi phí runtime không?* Có: method call qua interface là dynamic dispatch (itab lookup), không inline được như direct call. Trong hot path chạy hàng triệu lần thì đáng cân nhắc, còn ở mức service/repository thì không đáng kể so với latency DB.

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

### Ví dụ: cùng một method, hai kết quả khác nhau

Value receiver — method nhận **bản copy**, caller không thấy thay đổi:

```go
type Money struct {
    Amount int
}

func (m Money) Add(x int) {
    m.Amount += x // sửa trên bản copy
}

m := Money{Amount: 100}
m.Add(50)
fmt.Println(m.Amount) // 100 — KHÔNG đổi
```

```text
caller: Money{100}
            |
            | copy
            v
      method: Money{100} -> {150}   (bản copy, chết khi method return)
```

Pointer receiver — method nhận **địa chỉ**, caller thấy thay đổi:

```go
func (m *Money) Add(x int) {
    m.Amount += x
}

m := Money{Amount: 100}
m.Add(50)
fmt.Println(m.Amount) // 150
```

```text
caller: Money{100} @ 0xc000018030
            ^
            | pointer
      method sửa trực tiếp ô nhớ đó
```

### Production: repository/service luôn là pointer

```go
type UserRepository struct {
    db     *sql.DB
    redis  *redis.Client
    logger *zap.Logger
    mu     sync.Mutex
}

func (r *UserRepository) Create(ctx context.Context, u User) error { ... }
```

Nếu dùng value receiver, mỗi lần gọi method là copy toàn bộ struct: copy `*sql.DB` (pointer nên vẫn trỏ cùng pool, không quá hại), copy logger, và **copy cả `sync.Mutex`** — cái này thì hỏng thật: mỗi method call khóa một mutex riêng, mất hoàn toàn tác dụng đồng bộ. `go vet` sẽ báo `passes lock by value`.

Rule of thumb:

| Loại struct | Receiver | Ví dụ |
| --- | --- | --- |
| Value object nhỏ, immutable | value | `Money`, `Point`, `Coordinate`, `time.Time` |
| Có dependency, có state, có lock | pointer | `UserRepository`, `OrderService`, `http.Client` wrapper |

Nguyên tắc quan trọng nhất: **đừng trộn lẫn**. Nếu một method của type đã dùng pointer receiver thì cho tất cả method còn lại dùng pointer luôn, để method set nhất quán.

### Interview follow-up

* *`m.Add(50)` với pointer receiver mà `m` là value thì sao?* Go tự động lấy địa chỉ: `(&m).Add(50)`. Điều này chỉ hoạt động khi `m` là **addressable** — biến, field, phần tử slice. Phần tử map thì không: `myMap["k"].Add(50)` không compile được.
* *Type có pointer receiver có implement được interface khi truyền value không?* Không. `Money` (value) **không** satisfy interface nếu method dùng `*Money` receiver — phải truyền `&Money{}`. Đây là lỗi compile hay gặp nhất khi wire dependency.

Interview answer:

> Với service/repository/client, tôi thường dùng pointer receiver vì object có dependency và không muốn copy. Với value object nhỏ, value receiver giúp code an toàn hơn.

---

## 3.5. Reference, dereference, value type và reference type

Phần này nhiều người viết Go vài năm vẫn nhầm, và interviewer rất hay dùng nó để phân loại ứng viên.

### Reference và dereference

```go
a := 10
p := &a   // p giữ ĐỊA CHỈ của a
```

```text
a  @ 0x100  ->  [ 10 ]
                  ^
p  @ 0x200  ->  [ 0x100 ]
```

Dereference là "đi theo địa chỉ để tới ô nhớ thật":

```go
fmt.Println(*p)  // 10  — đọc giá trị tại 0x100
*p = 20          // ghi vào 0x100
fmt.Println(a)   // 20  — a đổi theo
```

Điểm khác C mà người từ C/C++ hay nhầm:

```go
*p++     // Go: HỢP LỆ, tương đương (*p)++ -> tăng GIÁ TRỊ được trỏ tới (a = 21)
(*p)++   // giống hệt dòng trên, chỉ rõ ràng hơn
```

Trong C, `*p++` nghĩa là "dereference rồi **tăng con trỏ**" (pointer arithmetic). Trong Go thì **không có pointer arithmetic**: `++` là một *statement* áp lên biểu thức `*p`, nên nó luôn tăng giá trị được trỏ tới. Đây cũng là lý do Go an toàn hơn — không thể trượt con trỏ ra ngoài vùng nhớ hợp lệ.

Hệ quả thực tế: pointer trong Go chỉ dùng để **chia sẻ** và **mutate** dữ liệu, không dùng để duyệt bộ nhớ. Muốn duyệt thì dùng slice.

### Value type — copy toàn bộ

`int`, `float`, `bool`, `string`, `struct`, `array`.

```go
a := 10
b := a
b = 20
// a == 10, b == 20 — hoàn toàn độc lập
```

### Reference-like type — copy header, chung dữ liệu

`slice`, `map`, `channel`, `func`, `pointer`.

```go
a := []int{1, 2}
b := a       // copy slice header (pointer/len/cap)
b[0] = 100
fmt.Println(a[0]) // 100 — a cũng đổi!
```

```text
a header: {ptr, len:2, cap:2}
b header: {ptr, len:2, cap:2}   <- hai header khác nhau
              \    /
               v  v
          underlying array: [100, 2]   <- nhưng CHUNG mảng này
```

Chính xác thì Go **không có reference type**: mọi thứ đều pass-by-value. Chỉ là *giá trị* của slice là một struct header chứa pointer, nên copy header xong vẫn trỏ chung mảng. Nói được câu này trong phỏng vấn là điểm cộng lớn.

### Bug production: struct chứa slice

```go
type User struct {
    Name  string
    Roles []string
}

u1 := User{Name: "A", Roles: []string{"admin"}}
u2 := u1          // copy struct — trông như đã "tách" hẳn
u2.Name = "B"     // OK, Name là string -> độc lập
u2.Roles[0] = "guest"

fmt.Println(u1.Roles[0]) // "guest" — u1 BỊ ĐỔI THEO
```

`Name` là value nên độc lập, nhưng `Roles` chỉ copy header → hai user chung một mảng. Trong service, đây là kiểu bug làm "sửa permission user B lại đổi luôn user A".

Cách sửa — deep copy phần slice:

```go
u2 := u1
u2.Roles = slices.Clone(u1.Roles) // Go 1.21+, hoặc make + copy
```

### Interview follow-up

* *`map` truyền vào function, sửa trong function caller có thấy không?* Có. Map value là pointer tới hmap struct → mọi thay đổi key/value đều thấy được. Nhưng gán `m = make(map[string]int)` bên trong function thì caller **không** thấy (chỉ đổi bản copy của biến).
* *Vậy tại sao slice `append` trong function caller lại không thấy?* Vì `append` có thể đổi `len`/pointer, mà đó là field của **header đã bị copy**. Xem mục Slice và array bên dưới.
* *`string` là value hay reference?* Value, nhưng nội dung bytes là immutable và share được — nên copy string rẻ (chỉ copy pointer + len), và không có nguy cơ aliasing như slice.

---

## 4. Concurrency vs parallelism

* **Concurrency:** tổ chức chương trình để xử lý nhiều việc *đan xen* nhau — cấu trúc code.
* **Parallelism:** thực sự chạy nhiều việc *cùng một thời điểm* trên nhiều CPU core — cách thực thi.

Câu nói của Rob Pike: *"Concurrency is about dealing with lots of things at once. Parallelism is about doing lots of things at once."*

### Sơ đồ

1 CPU core, 3 task → **concurrency, không parallelism**:

```text
Core 1: [A][B][C][A][B][C][A][C]...
        chuyển qua lại rất nhanh, không bao giờ có 2 task chạy cùng nanosecond
```

4 CPU core, 4 task → **parallelism**:

```text
Core 1: [ A ================= ]
Core 2: [ B ================= ]
Core 3: [ C ================= ]
Core 4: [ D ================= ]
        thật sự cùng lúc
```

### Trong Go

```go
go downloadImage()
go sendEmail()
go updateInventory()
```

Đoạn này **luôn** là concurrency. Có parallel hay không thì phụ thuộc runtime:

| Điều kiện | Kết quả |
| --- | --- |
| `GOMAXPROCS=1` | 3 goroutine luân phiên trên 1 P → chỉ concurrent |
| `GOMAXPROCS=8`, máy 8 core | có thể chạy song song thật |
| Task là I/O-bound (call API, query DB) | goroutine block ở syscall/network → runtime park nó và chạy goroutine khác; song song hay không gần như không quan trọng |
| Task là CPU-bound (encode ảnh, hash) | chỉ nhanh hơn nếu thật sự có nhiều core |

Từ Go 1.5, `GOMAXPROCS` mặc định = số core. Trong container thì nó đọc số core của **host**, không phải CPU limit của cgroup — nên pod bị limit 1 CPU mà `GOMAXPROCS=64` sẽ gây context switch thừa và CPU throttling. Cách xử lý: dùng `automaxprocs` (uber-go) hoặc set `GOMAXPROCS` theo CPU limit.

### Interview follow-up

* *Goroutine có đồng nghĩa với parallel không?* Không. Goroutine chỉ là concurrency primitive. Có song song hay không phụ thuộc `GOMAXPROCS`, số core và scheduler.
* *Thêm goroutine có luôn làm nhanh hơn không?* Không. Với CPU-bound, số worker > số core chỉ tăng context switch. Với I/O-bound thì tăng worker có ích, nhưng trần thật sự là capacity của dependency (DB pool, rate limit API), không phải CPU.
* *Vì sao service Go trong Kubernetes bị CPU throttling dù CPU usage thấp?* Rất thường là `GOMAXPROCS` lấy theo core của node chứ không theo `resources.limits.cpu`.

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

## 6.5. Select

`select` chờ trên nhiều channel cùng lúc, chạy case nào sẵn sàng trước. Gần như chắc chắn bị hỏi.

```go
select {
case <-ctx.Done():
    return ctx.Err()          // cancel/timeout từ tầng trên
case msg := <-ch:
    handle(msg)               // có message
case <-time.After(time.Second):
    return ErrTimeout         // timeout cục bộ
}
```

Quy tắc cần nhớ:

* Nhiều case cùng sẵn sàng → chọn **ngẫu nhiên** (chống starvation, không phải theo thứ tự viết).
* Không case nào sẵn sàng và **có** `default` → chạy `default` ngay, không block.
* Không case nào sẵn sàng và **không có** `default` → block cho tới khi có.
* `select {}` rỗng → block vĩnh viễn (deadlock).
* Nhận từ **nil channel** thì block mãi mãi — dùng có chủ đích để "tắt" một case động.

### Non-blocking với `default`

```go
select {
case results <- r:
    // gửi được
default:
    // buffer đầy -> không chờ, drop hoặc đếm metric
    metrics.DroppedTotal.Inc()
}
```

Hữu ích cho metric channel, event bus best-effort, và tránh block khi consumer chậm. Nhưng cẩn thận: `default` biến channel đầy thành **im lặng mất dữ liệu**, nên phải luôn đếm số bị drop.

### Production: RabbitMQ consumer

```go
func (c *Consumer) Run(ctx context.Context) error {
    msgs, err := c.ch.Consume(queue, "", false /* autoAck=false */, false, false, false, nil)
    if err != nil {
        return err
    }

    for {
        select {
        case <-ctx.Done():
            // SIGTERM -> dừng nhận message mới, thoát vòng lặp
            return ctx.Err()

        case msg, ok := <-msgs:
            if !ok {
                return ErrChannelClosed // connection rớt -> để supervisor reconnect
            }
            msgCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
            err := c.handle(msgCtx, msg)
            cancel()

            if err != nil {
                msg.Nack(false, shouldRequeue(err))
                continue
            }
            msg.Ack(false) // chỉ ack SAU KHI xử lý xong
        }
    }
}
```

```text
message tới ──┐
              ├──> select ──> handle -> ack
ctx.Done() ───┘              (SIGTERM) -> return -> graceful shutdown
```

### Bẫy `time.After` trong vòng lặp

```go
for {
    select {
    case msg := <-ch:
        handle(msg)
    case <-time.After(time.Minute): // MỖI vòng lặp tạo 1 timer mới,
        checkHealth()               // timer sống tới khi hết 1 phút dù đã không cần
    }
}
```

Loop tần suất cao → hàng nghìn timer tích tụ trong heap. Dùng `time.NewTicker` (nếu cần định kỳ) hoặc `time.NewTimer` + `Reset` (nếu cần idle timeout):

```go
timer := time.NewTimer(time.Minute)
defer timer.Stop()

for {
    select {
    case msg := <-ch:
        handle(msg)
        if !timer.Stop() {
            <-timer.C
        }
        timer.Reset(time.Minute)
    case <-timer.C:
        checkHealth()
        timer.Reset(time.Minute)
    }
}
```

### Interview follow-up

* *`select` với `default` trong `for` loop thì sao?* Thành busy-wait: CPU 100% vì loop quay không nghỉ. Đây là bug hay gặp khi mới học.
* *Làm sao tắt một case trong `select`?* Gán channel của case đó về `nil` — case nil block mãi nên coi như bị loại. Kỹ thuật chuẩn khi merge nhiều stream mà từng stream lần lượt close.
* *`select` có ưu tiên `ctx.Done()` không?* Không. Nếu cả `ctx.Done()` và `ch` cùng sẵn sàng, việc chọn là ngẫu nhiên — nghĩa là sau khi cancel, vẫn có thể xử lý thêm vài message. Muốn ưu tiên tuyệt đối thì phải check `ctx.Err()` riêng ở đầu vòng lặp.

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

### Vì sao? So sánh trên cùng một bài toán

**Bài toán 1 — counter.** State là một biến, thao tác là `count++`.

Mutex, 3 dòng, đọc là hiểu:

```go
mu.Lock()
count++
mu.Unlock()
```

Channel, phải dựng một goroutine owner sống suốt vòng đời service chỉ để giữ một số nguyên:

```go
counterChan <- 1        // ai đóng channel? đọc giá trị hiện tại thế nào?
```

Muốn đọc `count` thì phải thêm một channel reply nữa. Phức tạp hơn mà không được gì. Đây là bảo vệ state → **mutex thắng**. (Thực tế counter thuần thì `atomic.Int64` còn rẻ hơn cả mutex.)

**Bài toán 2 — queue job.** Producer sinh việc, worker tiêu thụ.

```text
Producer ──> channel (buffered) ──> Worker 1..N
```

Channel cho sẵn: hàng đợi, backpressure khi buffer đầy, phân phối job cho N worker, và tín hiệu kết thúc khi close. Nếu dùng mutex thì phải tự viết slice + `sync.Cond` + logic đánh thức worker — chính là viết lại channel bằng tay. Đây là truyền ownership của data → **channel thắng**.

Cách phát biểu trong phỏng vấn:

> Tôi chọn theo bản chất bài toán: *bảo vệ* một vùng nhớ dùng chung thì mutex/atomic; *chuyển giao* dữ liệu giữa các goroutine thì channel. Câu "share memory by communicating" không có nghĩa là cấm mutex.

### Interview follow-up

* *Mutex và RWMutex khác gì?* `RWMutex` cho nhiều reader song song, nhưng chỉ đáng dùng khi tỉ lệ đọc áp đảo và critical section đủ dài; critical section ngắn thì `RWMutex` còn chậm hơn `Mutex` vì overhead bookkeeping lớn hơn.
* *Copy struct có mutex thì sao?* Mutex bị copy theo → hai bản sao khóa hai lock khác nhau, mất hoàn toàn tác dụng. `go vet` bắt lỗi này. Vì vậy struct có mutex phải dùng pointer receiver.
* *Deadlock hay gặp nhất?* Lock hai mutex theo thứ tự ngược nhau ở hai code path, hoặc gọi một hàm cũng `Lock()` chính mutex đó trong lúc đang giữ lock (Go mutex không reentrant).

---

## 8. Worker pool

Worker pool giới hạn số goroutine xử lý đồng thời để bảo vệ DB, Redis, API ngoài hoặc queue consumer.

Đây là câu phân loại Junior/Middle rõ nhất.

Junior trả lời: *"Worker pool để giới hạn goroutine."*

Middle trả lời:

> Trong feature import Excel 200.000 dòng, nếu spawn mỗi dòng một goroutine thì 200k goroutine cùng tranh nhau DB connection pool chỉ có 30 slot. Tôi giới hạn 20 worker, đẩy job qua buffered channel, mỗi worker nhận context có timeout, và dùng WaitGroup để shutdown sạch khi nhận SIGTERM.

### Vì sao không spawn goroutine tự do?

```go
for _, order := range orders { // 100.000 orders
    go process(order)          // 100.000 goroutine
}
```

Mỗi goroutine cần query DB → gọi Redis → gọi API ngoài. Chuỗi sự cố:

```text
100.000 goroutine  (mỗi con ~4-8KB stack ban đầu -> vài trăm MB, và stack còn grow)
        |
        v
DB pool chỉ có 30 connection
        |
        v
99.970 goroutine xếp hàng chờ connection, tất cả đều đang giữ memory
        |
        v
scheduler overhead + GC scan hàng trăm nghìn stack
        |
        v
latency tăng -> API ngoài timeout -> retry -> càng nhiều goroutine
        |
        v
OOMKilled
```

Nghịch lý quan trọng: **spawn nhiều goroutine không làm hệ thống nhanh hơn**, vì trần thật sự là 30 connection DB. Nó chỉ chuyển hàng đợi từ "channel có kiểm soát" sang "hàng đợi vô hình bên trong pool", nơi không đo được, không cancel được, và tốn memory.

Với worker pool:

```text
jobs channel (buffer 100)
        |
        v
   20 workers  -> DB (tối đa 20 connection đồng thời, còn 10 dư cho API request thường)
        |
        v
   results / ack
```

### Skeleton production

```go
func RunPool(ctx context.Context, jobs <-chan Job, n int) error {
    g, ctx := errgroup.WithContext(ctx)

    for i := 0; i < n; i++ {
        g.Go(func() error {
            for {
                select {
                case <-ctx.Done():
                    return ctx.Err()
                case job, ok := <-jobs:
                    if !ok {
                        return nil // producer đã close -> worker thoát bình thường
                    }
                    // timeout cho từng job, không để một job treo cả worker
                    jobCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
                    err := process(jobCtx, job)
                    cancel()
                    if err != nil {
                        return fmt.Errorf("job %s: %w", job.ID, err)
                    }
                }
            }
        })
    }
    return g.Wait()
}
```

Thành phần bắt buộc:

| Thành phần | Vai trò |
| --- | --- |
| `jobs` channel (buffered) | hàng đợi + backpressure |
| N worker cố định | trần concurrency |
| `context.Context` | cancel toàn bộ khi shutdown/lỗi |
| timeout mỗi job | một job chậm không chiếm worker vĩnh viễn |
| `WaitGroup`/`errgroup` | chờ worker kết thúc trước khi close DB |
| retry + DLQ | xử lý lỗi transient vs permanent |
| metric | biết pool đang no hay đói |

### Chọn N bằng bao nhiêu?

Không có số cố định. Cách reasoning trong phỏng vấn:

* **CPU-bound** (encode ảnh, hash, compress): `N ≈ GOMAXPROCS`. Thêm nữa chỉ tăng context switch.
* **I/O-bound** (query DB, call API): N bị chặn bởi **dependency yếu nhất**. DB pool 30 connection và mỗi job cần 1 connection → N ≤ 30, thực tế để 20 và chừa 10 cho HTTP request thường. Nếu gọi API ngoài có rate limit 100 rps và mỗi call mất 200ms → N ≈ 100 × 0.2 = 20.
* Sau đó **đo, không đoán**: tăng dần N, theo dõi `job_latency` p99 và `queue_depth`. Khi tăng N mà throughput không tăng còn latency tăng → đã chạm trần dependency.

### Lỗi thường gặp

* Spawn goroutine vô hạn theo job (như trên).
* Không close `jobs` channel → worker `range` mãi mãi, `g.Wait()` treo.
* Không drain `results` channel → worker block khi gửi kết quả, pool đứng hình.
* Worker gọi dependency ngoài không timeout → một API treo giữ luôn worker slot, dần dần cả 20 worker đều kẹt.
* Buffer channel quá lớn (ví dụ 1.000.000) → che giấu backpressure: producer cứ nhồi, memory phình, và khi crash thì mất sạch job trong buffer.

### Metric nên expose

* `queue_depth` — sâu liên tục nghĩa là thiếu worker hoặc dependency chậm.
* `worker_busy` / `worker_idle` — busy 100% liên tục nghĩa là pool là bottleneck.
* `job_latency` (histogram) — p99 quan trọng hơn trung bình.
* `job_error_total` theo loại lỗi.
* `retry_total`, `dlq_total`.

### Interview follow-up

* *Worker pool khác gì semaphore?* Semaphore (`chan struct{}` buffer N, hoặc `x/sync/semaphore`) vẫn spawn một goroutine mỗi job nhưng chặn số job *chạy* đồng thời — đơn giản hơn, hợp khi số job hữu hạn và nhỏ. Worker pool giữ số goroutine **cố định**, hợp khi job stream vô hạn (queue consumer).
* *Job fail thì sao?* Phân biệt lỗi transient (network, deadlock, 503) → retry có backoff + jitter; và lỗi permanent (payload sai, record không tồn tại) → vào DLQ ngay, retry chỉ tốn thời gian.
* *Shutdown thế nào cho sạch?* Producer close `jobs` → worker xử lý hết job còn trong channel rồi return → `g.Wait()` → lúc đó mới close DB/Redis. Nếu close DB trước khi worker xong thì job cuối cùng fail.

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

### Production: context lan truyền suốt request

```text
Client (browser)
    |
    v
Gin handler        ctx = c.Request.Context()   <- Gin/net-http tạo sẵn, cancel khi client disconnect
    |
    v
OrderService.Create(ctx, req)
    |
    +--> UserRepo.Get(ctx, id)          -> db.QueryContext(ctx, ...)
    +--> PaymentGateway.Charge(ctx,...) -> http.NewRequestWithContext(ctx, ...)
    +--> Cache.Set(ctx, ...)            -> redis client nhận ctx
    |
    v
Postgres
```

Khi user đóng tab hoặc bấm Cancel:

```text
client disconnect
    |
    v
ctx.Done() được đóng
    |
    v
db.QueryContext trả về context.Canceled, Postgres nhận cancel request và bỏ query
    |
    v
HTTP call tới payment bị hủy
    |
    v
handler goroutine return  ->  không leak, không tốn DB connection cho request đã chết
```

Nếu ở đâu đó trong chuỗi bạn dùng `context.Background()`, sợi dây cancel bị **đứt** từ điểm đó trở xuống — query vẫn chạy tiếp dù không còn ai đọc kết quả.

### Ba lỗi context hay gặp nhất

**Lỗi 1 — `context.Background()` ở tầng dưới.**

```go
// SAI: mất toàn bộ deadline/cancel từ request
func (r *Repo) Get(id string) (*User, error) {
    return r.db.QueryContext(context.Background(), ...)
}

// ĐÚNG
func (r *Repo) Get(ctx context.Context, id string) (*User, error) {
    return r.db.QueryContext(ctx, ...)
}
```

`context.Background()` chỉ nên xuất hiện ở **entry point**: `main()`, khởi tạo consumer, cron job.

**Lỗi 2 — store context trong struct.**

```go
// SAI
type Service struct {
    ctx context.Context // context của request NÀO?
}
```

Service sống suốt vòng đời process (hàng tháng). Context sống theo một request (vài giây). Nhét context vào struct nghĩa là request thứ hai dùng lại context đã bị cancel của request thứ nhất → mọi call đều fail với `context.Canceled`.

```go
// ĐÚNG: context đi theo lời gọi, không theo object
func (s *Service) Create(ctx context.Context, req Request) error
```

Ngoại lệ hợp lệ duy nhất: struct đại diện cho **một** đơn vị công việc ngắn hạn (ví dụ `type request struct{ ctx context.Context }` nội bộ trong một pipeline), và ngay cả khi đó cũng nên tránh.

**Lỗi 3 — dùng `context.Value` như optional parameter.**

```go
// SAI: đây là tham số của hàm, không phải request-scoped metadata
ctx = context.WithValue(ctx, "limit", 10)
ctx = context.WithValue(ctx, "sort", "desc")
```

Mất type safety, không thấy được từ signature, không compiler check. Đúng ra là:

```go
func (s *Service) List(ctx context.Context, filter ListFilter) ([]Order, error)
```

`context.Value` chỉ dành cho thứ **xuyên qua mọi tầng mà không tầng nào quan tâm về mặt nghiệp vụ**:

| Nên bỏ vào context.Value | Không bỏ vào context.Value |
| --- | --- |
| `request_id`, `trace_id`, `span` | `page`, `limit`, `sort`, `keyword` |
| `user_id`, JWT claims sau khi auth | filter nghiệp vụ |
| tenant/org id (multi-tenant) | DB connection, logger config |
| locale, feature flag của request | bất cứ thứ gì hàm cần để chạy đúng |

Và luôn dùng **key có type riêng** để tránh va chạm key giữa các package:

```go
type ctxKey struct{} // unexported

func WithRequestID(ctx context.Context, id string) context.Context {
    return context.WithValue(ctx, ctxKey{}, id)
}

func RequestID(ctx context.Context) (string, bool) {
    id, ok := ctx.Value(ctxKey{}).(string)
    return id, ok
}
```

Không dùng string thuần (`context.WithValue(ctx, "user_id", ...)`) vì hai package khác nhau có thể dùng cùng chuỗi và ghi đè nhau — `go vet` cũng cảnh báo.

Interview answer:

> Context giúp tránh goroutine leak và request treo khi client disconnect hoặc dependency chậm. Trong service có DB, Redis, gRPC, queue worker, tôi luôn truyền context xuống các call có thể block.

### Interview follow-up

* *Quên `defer cancel()` thì sao?* Với `WithCancel`/`WithTimeout`, context con được đăng ký vào context cha. Không gọi `cancel()` thì entry đó không được gỡ khỏi cha → memory leak, và timer của `WithTimeout` cũng không được giải phóng sớm. `go vet` bắt được (`lostcancel`).
* *`ctx.Err()` trả về gì?* `nil` khi còn sống, `context.Canceled` khi bị cancel, `context.DeadlineExceeded` khi hết hạn. Phân biệt được hai cái này rất hữu ích: `DeadlineExceeded` là dependency chậm (cần điều tra), `Canceled` thường là client bỏ đi (bình thường, không nên alert).
* *Context có cancel goroutine không?* **Không.** Context chỉ *báo hiệu* qua `ctx.Done()`. Goroutine phải tự chọn lắng nghe và return. Đây là hiểu lầm rất phổ biến.
* *Timeout nên set ở tầng nào?* Set ở entry point theo SLA (ví dụ HTTP handler 3s), rồi mỗi dependency call có timeout nhỏ hơn (DB 2s, Redis 200ms). Context con **không thể** có deadline dài hơn cha — `WithTimeout(ctx, 10*time.Second)` trên một ctx còn 1s vẫn hết hạn sau 1s.

---

## 11. Goroutine leak

Goroutine leak xảy ra khi goroutine không bao giờ kết thúc dù request/job đã xong. Go có GC cho memory, nhưng **không có GC cho goroutine**: một goroutine bị block vĩnh viễn sẽ giữ stack của nó và mọi object nó tham chiếu, mãi mãi.

### Case 1 — receive mãi mãi

```go
func handler() {
    ch := make(chan Result)
    go func() {
        <-ch // không ai send -> block vĩnh viễn
    }()
    return // handler xong, goroutine kia còn sống
}
```

Fix: đảm bảo có sender, hoặc thêm `select` với `ctx.Done()`.

### Case 2 — send mãi mãi (leak kinh điển nhất trong HTTP handler)

```go
func handler(ctx context.Context) (Result, error) {
    ch := make(chan Result) // UNBUFFERED
    go func() {
        ch <- slowCall() // block cho tới khi có người nhận
    }()

    select {
    case r := <-ch:
        return r, nil
    case <-ctx.Done():
        return Result{}, ctx.Err() // timeout -> handler return, KHÔNG AI ĐỌC ch nữa
    }
}
```

Khi timeout, `slowCall()` xong lúc 10s và cố `ch <- result` — nhưng receiver đã bỏ đi. Goroutine block vĩnh viễn. Mỗi request timeout = một goroutine leak. Traffic 100 rps với 10% timeout → 10 goroutine leak mỗi giây.

Fix — buffered channel size 1, để sender luôn gửi được rồi thoát:

```go
ch := make(chan Result, 1) // sender không bao giờ block
```

### Case 3 — không lắng nghe `ctx.Done()`

```go
// SAI
for job := range jobs {
    process(job) // shutdown rồi vẫn chạy tiếp
}

// ĐÚNG
for {
    select {
    case <-ctx.Done():
        return
    case job, ok := <-jobs:
        if !ok {
            return
        }
        process(ctx, job)
    }
}
```

### Case 4 — ticker/timer không `Stop`

```go
// SAI
ticker := time.NewTicker(time.Second) // runtime giữ timer mãi
for { ... }

// ĐÚNG
ticker := time.NewTicker(time.Second)
defer ticker.Stop()
```

Lưu ý: `time.After` trong một `select` chạy trong vòng lặp cũng tạo timer mới mỗi vòng, và timer đó sống tới khi hết hạn dù đã không cần nữa. Loop tần suất cao thì nên dùng `time.NewTimer` + `Reset`.

### Case 5 — worker không shutdown khi service stop

Producer không close `jobs` channel → worker `range jobs` chờ mãi → `wg.Wait()` trong graceful shutdown treo → Kubernetes `SIGKILL` sau grace period → job đang xử lý mất, message RabbitMQ chưa ack bị redeliver.

### Phát hiện leak

Triệu chứng: goroutine count chỉ tăng, không bao giờ giảm.

```text
goroutine count theo thời gian
100 -> 500 -> 2.000 -> 8.000 -> 50.000 ... (memory tăng song song)
```

Service khỏe mạnh thì goroutine count dao động quanh một mức ổn định (baseline + số request đang xử lý).

Cách debug, theo thứ tự:

```go
// 1. Metric — expose liên tục, đây là thứ báo động sớm nhất
runtime.NumGoroutine()
```

```bash
# 2. pprof — xem goroutine đang kẹt ở ĐÂU (debug=2 cho stack trace đầy đủ)
curl 'http://localhost:6060/debug/pprof/goroutine?debug=2' > goroutines.txt

# hoặc phân tích tương tác
go tool pprof http://localhost:6060/debug/pprof/goroutine
(pprof) top
(pprof) traces
```

Đọc output: nếu thấy 40.000 goroutine cùng đứng ở một dòng như `chan send` trong `handler.go:52`, bạn đã tìm ra thủ phạm — đó chính là Case 2.

```go
// 3. Test tự động — bắt leak ngay trong CI
func TestMain(m *testing.M) {
    goleak.VerifyTestMain(m) // go.uber.org/goleak
}
```

### Interview follow-up

* *Goroutine leak có bị GC dọn không?* Không. Goroutine đang block là "reachable" theo định nghĩa của runtime; GC không thể biết nó sẽ không bao giờ chạy tiếp. Chỉ có deadlock **toàn bộ** chương trình mới bị runtime phát hiện (`fatal error: all goroutines are asleep`).
* *Buffered channel size 1 có luôn cứu được không?* Cứu được pattern "một sender, một kết quả". Nếu sender gửi N kết quả thì buffer phải ≥ N, hoặc sender phải `select` với `ctx.Done()` khi gửi.
* *Làm sao biết baseline goroutine bao nhiêu là bình thường?* Chạy load test, xem count ổn định ở mức nào, rồi đặt alert khi vượt ngưỡng đó nhiều lần (ví dụ > 2× baseline trong 10 phút).

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

#### Các case dẫn tới panic (gần như chắc chắn bị hỏi)

| Case | Ví dụ | Ghi chú |
| --- | --- | --- |
| Nil pointer dereference | `var u *User; u.Name` | Phổ biến nhất |
| Index out of range | `s := []int{1,2}; s[5]` | |
| Slice bounds out of range | `s[2:10]` khi `cap(s) < 10` | |
| Type assertion sai | `v := i.(string)` khi `i` là int | Dùng `v, ok := i.(string)` để không panic |
| Send vào closed channel | `close(ch); ch <- 1` | Nhận từ closed channel thì **an toàn** |
| Close channel đã close | `close(ch); close(ch)` | |
| Close nil channel | `var ch chan int; close(ch)` | |
| Concurrent map write | 2 goroutine cùng ghi map | `fatal error`, **không recover được** |
| Chia cho 0 (integer) | `a / b` với `b == 0` | Float chia 0 ra `+Inf`, không panic |
| Ghi vào nil map | `var m map[string]int; m["a"]=1` | Đọc từ nil map thì OK, trả zero value |
| Gọi method trên nil interface | `var s Servicer; s.Do()` | |

Ví dụ nil pointer trong service thật — pattern hay gặp nhất là quên check error trước khi dùng kết quả:

```go
user, err := repo.FindByID(ctx, id) // trả về (nil, ErrNotFound)
// quên check err
fmt.Println(user.Name) // panic: nil pointer dereference
```

Và bẫy nil map trong struct:

```go
type Config struct {
    Tags map[string]string // zero value là nil, KHÔNG phải map rỗng
}

c := Config{}
c.Tags["env"] = "prod" // panic: assignment to entry in nil map

// Fix
c := Config{Tags: make(map[string]string)}
```

#### Recover: vị trí duy nhất đúng

`recover()` chỉ có tác dụng khi gọi **trực tiếp trong một `defer`** của hàm đang panic.

```go
func RecoveryMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if rec := recover(); rec != nil {
                // BẮT BUỘC: log stack trace, nếu không thì mất dấu hoàn toàn
                log.Error("panic recovered",
                    zap.Any("panic", rec),
                    zap.String("request_id", RequestIDFrom(r.Context())),
                    zap.ByteString("stack", debug.Stack()),
                )
                writeError(w, http.StatusInternalServerError, "internal_error")
            }
        }()
        next.ServeHTTP(w, r)
    })
}
```

```text
Có recovery middleware:          Không có:
panic ở 1 request                panic ở 1 request
    |                                |
    v                                v
recover -> log stack             unwind hết stack
    |                                |
    v                                v
trả 500 cho request đó           PROCESS CHẾT
    |                                |
    v                                v
9.999 request khác vẫn chạy      toàn bộ request in-flight mất trắng
```

#### Ba điều rất dễ trả lời sai

1. **Recover không bắt được panic ở goroutine khác.**

```go
go func() {
    panic("boom") // KHÔNG middleware nào cứu được — cả process chết
}()
```

Mỗi goroutine tự chịu trách nhiệm cho panic của mình. Vì vậy mọi goroutine spawn từ handler/worker đều cần `defer recover()` riêng:

```go
go func() {
    defer func() {
        if rec := recover(); rec != nil {
            log.Error("worker panic", zap.Any("panic", rec), zap.ByteString("stack", debug.Stack()))
        }
    }()
    doWork()
}()
```

2. **Một số lỗi là `fatal error`, không phải panic — recover vô dụng.** Concurrent map read/write, out of memory, và deadlock toàn cục đều giết process ngay lập tức.

3. **Đừng dùng panic cho business error.**

```go
panic("user not found")                 // SAI
return nil, fmt.Errorf("find user %s: %w", id, ErrUserNotFound) // ĐÚNG
```

Panic hợp lý ở: `init()` khi config bắt buộc thiếu (fail fast lúc khởi động còn hơn chạy sai), `MustCompile` cho regex hằng, và invariant "không bao giờ xảy ra" trong code nội bộ.

#### Interview follow-up

* *Recover xong có nên tiếp tục xử lý request không?* Không. State đã không xác định — trả 500 và để client retry. Recover là để **process sống sót**, không phải để "sửa" request đó.
* *Panic trong `defer` thì sao?* Nó thay thế panic đang diễn ra; panic gốc bị mất nếu không log lại. Đây là lý do luôn log `debug.Stack()` ngay trong recover.
* *Recovery middleware của Gin đã đủ chưa?* Đủ cho HTTP handler, nhưng **không** phủ goroutine bạn tự spawn, không phủ RabbitMQ consumer, không phủ cron. Mỗi entry point cần recovery riêng.

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

Race detector chỉ phát hiện race **thực sự xảy ra trong lần chạy đó**, nên phải chạy kèm test có concurrency thật. Nó làm chương trình chậm ~10x và tốn memory ~5-10x, nên dùng cho test và staging, không bật ở production.

### Memory model và happens-before: vì sao "logic đúng" vẫn race

Đây là câu hỏi phân loại rõ Middle và Senior.

```go
var done bool
var result int

go func() {
    result = 42   // (1)
    done = true   // (2)
}()

for !done {       // (3)
}
fmt.Println(result) // có thể in ra 0!
```

Về mặt "logic tuần tự", (1) chạy trước (2), nên khi thấy `done == true` thì `result` phải là 42. Nhưng chương trình này **sai**, vì hai lý do:

1. **Reordering.** Compiler và CPU được phép sắp xếp lại (1) và (2) nếu trong phạm vi một goroutine kết quả không đổi. Goroutine kia có thể thấy `done = true` trước khi `result = 42`.
2. **Visibility.** Không có gì bảo đảm goroutine chính *bao giờ* nhìn thấy `done = true` — giá trị có thể nằm trong cache/register của core khác. Vòng `for !done {}` có thể quay mãi mãi.

Go memory model nói: một thao tác ghi chỉ **được bảo đảm** nhìn thấy bởi goroutine khác nếu có quan hệ **happens-before** giữa chúng. Các quan hệ đó do các primitive tạo ra, không tự nhiên mà có:

| Cơ chế | Quan hệ happens-before được bảo đảm |
| --- | --- |
| `mu.Unlock()` → `mu.Lock()` lần sau | mọi ghi trước Unlock đều thấy được sau Lock |
| `ch <- v` → `<-ch` | mọi ghi trước khi send đều thấy được sau khi receive |
| `close(ch)` → receive trả về | tương tự |
| `wg.Done()` → `wg.Wait()` return | |
| `once.Do(f)` return | mọi ghi trong `f` đều thấy được |
| `atomic.Store` → `atomic.Load` | |

Kết luận cần nói ra trong phỏng vấn:

> Race không phải "hiếm khi mới sai". Nó là **hành vi không xác định** — có thể chạy đúng 1 triệu lần rồi sai đúng lúc production peak, hoặc chỉ sai khi đổi compiler/CPU. Không có "race lành tính". Cứ shared mutable state là phải có một trong các primitive trên.

Sửa ví dụ trên:

```go
var done atomic.Bool
var result int64

go func() {
    atomic.StoreInt64(&result, 42)
    done.Store(true) // atomic store tạo happens-before edge
}()

for !done.Load() {
    runtime.Gosched()
}
fmt.Println(atomic.LoadInt64(&result)) // luôn là 42
```

Thực tế thì dùng channel hoặc `WaitGroup` còn rõ hơn nhiều.

Trong phỏng vấn, nên liên hệ với inventory, wallet, session, cache in-memory, request counter.

---

## 13.5. Bộ công cụ trong `sync` và `sync/atomic`

### `sync.Once` — khởi tạo đúng một lần

```go
var (
    once   sync.Once
    client *redis.Client
)

func Redis() *redis.Client {
    once.Do(func() {
        client = redis.NewClient(&redis.Options{Addr: addr})
    })
    return client
}
```

Dù 100 goroutine cùng gọi `Redis()` tại thời điểm khởi động, `redis.NewClient` chỉ chạy **một lần**; 99 goroutine còn lại block cho tới khi hàm đó xong rồi mới nhận được client đã sẵn sàng. Đây là điểm quan trọng: `Once` không chỉ "chạy một lần" mà còn bảo đảm **happens-before** — không ai nhìn thấy client ở trạng thái nửa vời.

Nếu tự viết bằng `if client == nil` thì có race, và tự viết bằng mutex + double-check thì dài dòng và dễ sai.

Dùng ở: lazy init connection pool, load config, đăng ký metric, parse template.

Cảnh báo: `Once` không có cơ chế báo lỗi. Nếu `NewClient` fail, `once.Do` vẫn coi như đã chạy và không bao giờ thử lại. Nếu cần retry thì lưu error ra ngoài và tự xử lý, hoặc dùng `sync.OnceValues` (Go 1.21+).

### `sync.Pool` — tái sử dụng object tạm

```go
var bufPool = sync.Pool{
    New: func() any {
        return new(bytes.Buffer)
    },
}

func encode(v any) ([]byte, error) {
    buf := bufPool.Get().(*bytes.Buffer)
    buf.Reset()                  // BẮT BUỘC: object cũ còn dữ liệu của request trước
    defer bufPool.Put(buf)

    if err := json.NewEncoder(buf).Encode(v); err != nil {
        return nil, err
    }
    return slices.Clone(buf.Bytes()), nil // copy ra, vì buf sắp bị trả lại pool
}
```

Không có pool: mỗi request allocate một `bytes.Buffer` mới → GC phải dọn hàng nghìn buffer mỗi giây. Có pool: buffer được tái dùng, `allocs/op` giảm rõ rệt.

```text
Before sync.Pool:  15 allocs/op,  8.400 B/op
After  sync.Pool:   3 allocs/op,    256 B/op
```

Ba cái bẫy chết người:

1. **Không `Reset()`** → dữ liệu request trước rò rỉ sang request sau. Đây là lỗ hổng bảo mật thật, không phải lý thuyết.
2. **Giữ tham chiếu sau khi `Put`** → trả buffer về pool rồi vẫn dùng `buf.Bytes()`, goroutine khác lấy đúng buffer đó ra ghi đè. Vì vậy phải `Clone` trước khi trả.
3. **Dùng cho object sống lâu** → `sync.Pool` bị **GC dọn sạch sau mỗi chu kỳ GC**. Nó là bộ đệm allocation, không phải cache. Đừng dùng để cache dữ liệu.

Chỉ dùng khi benchmark chứng minh có lợi — với object nhỏ, `sync.Pool` có thể *chậm hơn* allocate mới.

### `sync.Cond` — chờ điều kiện

Hiếm dùng trong backend service (channel thường hợp hơn). Chỉ cần biết nó tồn tại: dùng khi nhiều goroutine phải chờ một điều kiện trên shared state trở thành đúng, và không thể mô hình hóa bằng channel. Ví dụ: bounded queue tự cài, connection pool tự viết. Trong phỏng vấn, nói được *"tôi ưu tiên channel; `sync.Cond` chỉ dùng khi cần đánh thức nhiều waiter theo một điều kiện phức tạp trên state chung"* là đủ.

### `sync/atomic` — counter và flag không cần mutex

```go
var requestCount atomic.Int64  // Go 1.19+ có type an toàn hơn hàm atomic.AddInt64

requestCount.Add(1)
n := requestCount.Load()
```

So với mutex:

| | `atomic` | `Mutex` |
| --- | --- | --- |
| Phạm vi | **một** biến đơn (int, pointer, bool) | nhiều biến, cả một block logic |
| Chi phí | 1 CPU instruction (lock-free) | có thể park goroutine, đắt hơn |
| Khi nào dùng | counter, flag, config pointer swap | mọi invariant liên quan ≥2 biến |

Nguyên tắc: nếu cần **hai** biến nhất quán với nhau thì atomic **không** đủ.

```go
// SAI: hai atomic riêng lẻ không tạo thành một transaction
balance.Add(-100)  // giữa hai dòng này, goroutine khác có thể đọc state không nhất quán
txCount.Add(1)

// ĐÚNG: cần mutex bao cả hai
mu.Lock()
balance -= 100
txCount++
mu.Unlock()
```

`atomic.Value` / `atomic.Pointer` rất hợp để hot-reload config: build config mới hoàn chỉnh rồi swap pointer một phát, reader không bao giờ thấy config nửa vời và không tốn lock khi đọc.

### `WaitGroup` vs `errgroup` (câu hỏi rất phổ biến ở mức Middle)

`sync.WaitGroup` chỉ **đếm**: chờ N goroutine xong. Nó không biết gì về error và không hủy được gì.

```go
var wg sync.WaitGroup
for _, task := range tasks {
    wg.Add(1)
    go func(t Task) {
        defer wg.Done()
        if err := t.Run(); err != nil {
            // rồi sao? log? gán vào biến chung (race)? channel error?
            // và các goroutine KHÁC vẫn chạy tiếp dù task này đã fail
        }
    }(task)
}
wg.Wait()
```

`errgroup` (`golang.org/x/sync/errgroup`) giải quyết đúng hai thiếu sót đó: **thu error đầu tiên** và **cancel toàn bộ phần còn lại**.

```go
g, ctx := errgroup.WithContext(ctx)

g.Go(func() error { return uploadImage(ctx, file) })
g.Go(func() error { return generateThumbnail(ctx, file) })
g.Go(func() error { return saveMetadata(ctx, file) })

if err := g.Wait(); err != nil {
    return err // error đầu tiên; các goroutine còn lại đã nhận ctx.Done()
}
```

```text
generateThumbnail FAIL
        |
        v
errgroup cancel ctx
        |
        +--> uploadImage      thấy ctx.Done() -> dừng, không tốn bandwidth S3
        +--> saveMetadata     thấy ctx.Done() -> dừng, không ghi rác vào DB
        |
        v
g.Wait() trả về error của thumbnail
```

Điều kiện để cơ chế này hoạt động: **goroutine phải thật sự tôn trọng `ctx`**. Nếu bên trong `uploadImage` bạn dùng `context.Background()` thì errgroup cancel cũng vô ích.

`errgroup` còn có `g.SetLimit(20)` — giới hạn số goroutine chạy đồng thời, tức là một worker pool viết trong hai dòng.

| | `WaitGroup` | `errgroup` |
| --- | --- | --- |
| Chờ xong | có | có |
| Thu error | không | có (error đầu tiên) |
| Hủy phần còn lại khi lỗi | không | có (qua `WithContext`) |
| Giới hạn concurrency | không | có (`SetLimit`) |
| Khi nào dùng | fire-and-forget, các task độc lập, lỗi xử lý riêng | task song song mà **một lỗi làm phần còn lại vô nghĩa** |

Lưu ý `WaitGroup`: `wg.Add()` phải gọi **trước** khi spawn goroutine, không được gọi bên trong goroutine (race với `Wait`). Và `WaitGroup` không được copy — luôn truyền `*sync.WaitGroup`.

### `singleflight` — chống cache stampede

```go
var g singleflight.Group

func GetUser(ctx context.Context, id string) (*User, error) {
    v, err, _ := g.Do(id, func() (any, error) {
        return db.QueryUser(ctx, id) // 1000 request cùng miss cache -> chỉ 1 query DB
    })
    if err != nil {
        return nil, err
    }
    return v.(*User), nil
}
```

Khi một hot key hết TTL, 1000 request đồng thời cùng miss cache và cùng lao vào DB — đó là cache stampede, đủ để sập DB. `singleflight` gộp chúng lại: chỉ một request thật sự query, 999 request còn lại chờ và dùng chung kết quả.

---

## 14. Memory management

Go có GC, nhưng backend hiệu năng tốt vẫn cần hiểu allocation.

Khái niệm chính:

* Stack: local data, rẻ (chỉ dịch stack pointer), tự do khi function return, goroutine stack grow/shrink.
* Heap: object sống vượt scope hoặc escape, chịu GC.
* Escape analysis: compiler quyết định biến nằm stack hay heap.
* GC pressure: nhiều allocation nhỏ, object sống lâu, slice giữ reference lớn.

### Escape analysis (interviewer Go rất thích hỏi)

Compiler tự phân tích: *"biến này còn sống sau khi function return không?"* Nếu có → phải lên heap.

**Escape — biến vượt ra ngoài scope:**

```go
func newUser() *User {
    u := User{Name: "A"} // trả pointer ra ngoài -> u phải sống tiếp
    return &u            // "moved to heap: u"
}
```

**Không escape — dù có lấy địa chỉ:**

```go
func sum() int {
    u := User{Age: 30} // chỉ dùng trong function
    p := &u            // lấy địa chỉ nhưng không thoát ra ngoài
    return p.Age       // -> vẫn nằm STACK
}
```

Điểm mấu chốt để nói đúng trong phỏng vấn: **`&` không có nghĩa là heap.** Quyết định nằm ở chỗ pointer đó có *thoát* khỏi function hay không. Ngược lại, Go không có `new` = heap và `{}` = stack như C++ — `new(int)` hoàn toàn có thể nằm trên stack nếu không escape.

Kiểm chứng bằng compiler, không đoán:

```bash
go build -gcflags="-m" ./...
```

```text
./main.go:10:2: moved to heap: u
./main.go:18:2: u does not escape
./main.go:25:12: ... argument does not escape
```

Các nguyên nhân escape phổ biến khác:

| Pattern | Vì sao escape |
| --- | --- |
| `return &localVar` | pointer thoát ra ngoài |
| Gán vào field của struct trên heap | với tới được từ heap |
| Truyền vào `interface{}`/`any` | `fmt.Println(x)` — compiler không biết callee làm gì với nó |
| Gửi pointer qua channel | goroutine khác có thể giữ nó |
| Closure capture biến rồi `go func(){...}` | closure sống lâu hơn function |
| Slice/map có size không xác định lúc compile | `make([]int, n)` với `n` là biến |

`fmt.Println` là thủ phạm thầm lặng trong hot path: mọi argument đều bị box vào `any` → escape → allocation. Đây là lý do logger production dùng structured logging với typed field (`zap.String(...)`) thay vì `fmt.Sprintf`.

Giảm allocation:

* Preallocate slice/map khi biết size gần đúng: `make([]Order, 0, len(rows))`.
* Tránh convert `[]byte` <-> `string` trong hot path (mỗi lần convert là một copy).
* Reuse buffer bằng `sync.Pool` cho object tạm lớn.
* Dùng benchmark `go test -bench -benchmem` để đo `allocs/op`.
* Dùng `pprof` để tìm allocation thật, không đoán.

### Bug phổ biến: sub-slice giữ mảng lớn

```go
big := make([]byte, 10*1024*1024) // 10MB, ví dụ nội dung file upload
small := big[:10]                 // chỉ cần 10 byte đầu (magic header)
return small                      // nhưng GIỮ NGUYÊN cả 10MB không cho GC thu
```

Slice header của `small` vẫn trỏ vào underlying array 10MB. GC không thể thu hồi mảng đó chừng nào `small` còn sống. Cache 1000 header như vậy = 10GB rò rỉ.

Fix — copy sang mảng mới rồi bỏ tham chiếu tới mảng lớn:

```go
small := make([]byte, 10)
copy(small, big[:10])   // hoặc: small := slices.Clone(big[:10])
return small            // giờ big được GC thu bình thường
```

---

## 15. Slice, array và map internals

### Array vs slice

**Array có size cố định và là value type — copy là copy sạch:**

```go
a := [3]int{1, 2, 3} // [3]int — size là một phần của TYPE
b := a               // copy toàn bộ 3 phần tử
b[0] = 100
fmt.Println(a[0])    // 1 — a KHÔNG đổi
```

`[3]int` và `[4]int` là hai type **khác nhau**, không gán cho nhau được. Đây là lý do array rất ít dùng trực tiếp trong code backend.

**Slice là struct 3 field — copy header, chung mảng:**

```go
type sliceHeader struct {
    ptr *T  // trỏ tới phần tử đầu trong underlying array
    len int // số phần tử đang dùng
    cap int // sức chứa từ ptr tới cuối mảng
}
```

```go
a := []int{1, 2, 3}
b := a            // copy 3 field header
b[0] = 100
fmt.Println(a[0]) // 100 — a ĐỔI THEO
```

```text
a header {ptr, len:3, cap:3}  ─┐
b header {ptr, len:3, cap:3}  ─┤ hai header độc lập
                               v
        underlying array: [100][2][3]   <- nhưng chung mảng này
```

### Bug append kinh điển (câu hỏi phỏng vấn phổ biến nhất về slice)

```go
func appendValue(s []int) {
    s = append(s, 10)
}

func main() {
    s := []int{1, 2, 3}
    appendValue(s)
    fmt.Println(s) // [1 2 3] — KHÔNG có 10
}
```

Vì sao? `append` trả về header **mới** (len tăng lên 4), nhưng header đó chỉ được gán vào bản copy `s` bên trong function. Header của caller vẫn có `len = 3`. Ngay cả khi giá trị 10 đã được ghi vào underlying array, caller cũng không "nhìn thấy" nó vì len của nó vẫn là 3.

Đây là lý do `append` **luôn phải gán lại**: `s = append(s, x)`.

Nhưng câu trả lời đầy đủ mà interviewer muốn nghe là: **"có thể thấy hoặc không, tùy capacity"**. Trường hợp nguy hiểm hơn:

```go
func modify(s []int) {
    s = append(s, 99) // ghi vào s[3] của MẢNG CHUNG nếu còn cap
}

func main() {
    base := make([]int, 3, 5) // len=3, cap=5 — còn chỗ trống
    a := base[:3]
    b := base[:3]

    a = append(a, 111) // cap còn -> ghi vào ô index 3 của mảng chung
    b = append(b, 222) // cap còn -> GHI ĐÈ lên đúng ô đó

    fmt.Println(a[3]) // 222 — a bị b ghi đè!
}
```

```text
cap CÒN chỗ:                       cap HẾT chỗ:
append ghi vào mảng cũ             allocate mảng MỚI (~2x), copy sang
    |                                   |
    v                                   v
mọi slice chung mảng đều bị ảnh    slice cũ trỏ mảng cũ, slice mới trỏ
hưởng (aliasing bug)               mảng mới -> tách rời, không aliasing
```

Nghĩa là hành vi **phụ thuộc capacity tại thời điểm chạy** — cùng một đoạn code có thể đúng với input này và sai với input khác. Rất khó debug.

Phòng tránh: khi cắt slice để giao cho nơi khác giữ, dùng **full slice expression** để chặn cap:

```go
part := base[0:3:3] // low:high:max -> cap = 3, append chắc chắn allocate mảng mới
```

Hoặc `slices.Clone(base[:3])` nếu muốn tách hẳn.

### Growth của append

Khi hết cap, runtime allocate mảng mới rồi copy. Chiến lược tăng trưởng (Go hiện đại): slice nhỏ tăng ~2x, slice lớn (>256 phần tử) tăng ~1.25x rồi làm tròn theo size class. Không nên học thuộc con số — điều cần nói là:

> Append vào slice không preallocate là O(1) amortized, nhưng mỗi lần grow là một allocation + copy toàn bộ. Với vòng lặp 100k phần tử, `make([]T, 0, 100000)` loại bỏ khoảng 20 lần grow và 20 lần copy — trong benchmark thường thấy chênh 2-3 lần về thời gian và giảm allocation từ hàng chục xuống 1.

### Map

Điểm cần nắm:

* **Không an toàn cho concurrent read/write** (chi tiết ngay dưới).
* **Iteration order ngẫu nhiên có chủ đích** — runtime cố tình random hóa để lập trình viên không lỡ phụ thuộc vào thứ tự. Muốn deterministic thì lấy keys ra, `sort`, rồi duyệt.
* Nên preallocate nếu biết size: `make(map[string]User, 1000)`.
* **Không lấy được địa chỉ của phần tử map**: `&m["k"]` không compile, và `m["k"].Field = x` cũng không (nếu value là struct). Vì map có thể rehash và dời phần tử. Cách làm: value là pointer (`map[string]*User`), hoặc lấy ra, sửa, gán lại.
* Xóa key không trả memory về OS ngay; map chỉ grow, không shrink. Map từng có hàng triệu key rồi xóa hết vẫn giữ bucket → nếu cần giải phóng thì tạo map mới.

### Map và concurrency

Map **không** thread-safe. Đây không phải race "có thể sai giá trị" mà là **crash cả process**:

```go
m := map[string]int{}

go func() { m["a"] = 1 }()          // write
go func() { fmt.Println(m["a"]) }() // read
```

```text
fatal error: concurrent map read and map write
```

Chú ý chữ **`fatal error`**, không phải `panic` — runtime chủ động giết process và **`recover()` không cứu được**. Đây là điểm khác biệt quan trọng: một handler ghi map không khóa có thể làm sập toàn bộ pod, kể cả khi bạn đã có recovery middleware.

Ba cách sửa:

```go
// 1. Mutex — mặc định, đơn giản, rõ ràng
type SafeCache struct {
    mu sync.Mutex
    m  map[string]int
}

func (c *SafeCache) Get(k string) (int, bool) {
    c.mu.Lock()
    defer c.mu.Unlock()
    v, ok := c.m[k]
    return v, ok
}
```

```go
// 2. RWMutex — khi đọc áp đảo ghi (config cache, feature flag)
mu sync.RWMutex
mu.RLock()   // nhiều reader song song
defer mu.RUnlock()
```

```go
// 3. sync.Map — chỉ cho 2 trường hợp hẹp mà doc nêu rõ:
//    (a) key ghi một lần, đọc nhiều lần
//    (b) các goroutine thao tác trên tập key rời nhau
var m sync.Map
m.Store("a", 1)
v, ok := m.Load("a")
```

`sync.Map` **không** phải "map nhanh hơn". Nó đánh đổi: không có type safety (`any`), không lấy được `len()`, và với workload ghi nhiều thì **chậm hơn** map + mutex. Mặc định nên là map + `RWMutex`; chỉ đổi sang `sync.Map` khi đã benchmark chứng minh có lợi.

Phát hiện: `go test -race ./...` và chạy race detector cả trong staging.

### Interview follow-up

* *Vì sao `append` trong function không ảnh hưởng caller nhưng `s[0] = x` thì có?* Vì `s[0] = x` ghi qua pointer (chung mảng), còn `append` sửa `len` — mà `len` là field của header đã bị copy.
* *Slice có thể nil không? Khác gì slice rỗng?* `var s []int` là nil (ptr = nil, len = cap = 0), `s := []int{}` là rỗng nhưng không nil. Cả hai đều `len == 0`, `range` được, `append` được. Khác biệt duy nhất đáng quan tâm: `json.Marshal(nil slice)` ra `null`, còn slice rỗng ra `[]` — hay gây bug cho frontend.
* *Map iteration order có ổn định trong một lần chạy không?* Không, mỗi lần `range` cùng một map cũng cho thứ tự khác nhau.

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

### Graceful shutdown: thứ tự đúng

Thứ tự sai sẽ mất request hoặc mất message. Đây là flow chuẩn trong Kubernetes:

```text
Kubernetes gửi SIGTERM
        |
        v
1. Readiness probe -> FAIL ngay
        |
        v
2. K8s gỡ Pod khỏi Service endpoints (mất vài giây để lan tới kube-proxy/ingress)
   -> đây là lý do phải SLEEP ~5s trước bước 3, nếu không vẫn có request mới bay vào
        |
        v
3. http.Server.Shutdown(ctx) — đóng listener, KHÔNG nhận request mới,
   nhưng chờ request đang chạy hoàn tất (tối đa shutdownTimeout)
        |
        v
4. Cancel context của worker/consumer -> dừng nhận job/message MỚI
        |
        v
5. Chờ worker xử lý xong job đang dở (wg.Wait / g.Wait)
   -> RabbitMQ: ack xong message cuối cùng
        |
        v
6. Đóng RabbitMQ channel/connection
        |
        v
7. Đóng DB pool, Redis client   <- PHẢI SAU bước 5, nếu không job cuối fail
        |
        v
8. Flush logger/tracer -> exit(0)
```

```go
func main() {
    ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer stop()

    srv := &http.Server{Addr: ":8080", Handler: router}
    go func() {
        if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
            log.Fatal("listen", zap.Error(err))
        }
    }()

    workerCtx, cancelWorkers := context.WithCancel(context.Background())
    var wg sync.WaitGroup
    wg.Add(1)
    go func() { defer wg.Done(); consumer.Run(workerCtx) }()

    <-ctx.Done() // SIGTERM
    log.Info("shutting down")

    health.SetNotReady()           // 1. readiness fail
    time.Sleep(5 * time.Second)    // 2. chờ K8s gỡ endpoint

    shutdownCtx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
    defer cancel()

    if err := srv.Shutdown(shutdownCtx); err != nil { // 3. chờ request in-flight
        log.Error("http shutdown", zap.Error(err))
    }

    cancelWorkers() // 4. worker dừng nhận job mới
    wg.Wait()       // 5. chờ job đang dở xong + ack

    rabbit.Close()  // 6
    db.Close()      // 7
    redis.Close()   // 7
    logger.Sync()   // 8
}
```

Nếu bỏ qua graceful shutdown, với RabbitMQ:

```text
Consumer đang xử lý message (đã trừ kho, chưa ack)
        |
        v
Pod bị SIGKILL
        |
        v
RabbitMQ không nhận được ack -> redeliver message cho consumer khác
        |
        v
Trừ kho LẦN HAI  -> đây chính là lý do consumer BẮT BUỘC phải idempotent
```

`terminationGracePeriodSeconds` của Pod phải **lớn hơn** tổng thời gian shutdown của bạn (ở ví dụ trên: 5s sleep + 25s timeout → đặt ít nhất 40s). Mặc định K8s là 30s; vượt quá là `SIGKILL` không thương tiếc.

Interview answer:

> Điểm quan trọng nhất là thứ tự: fail readiness và chờ load balancer gỡ endpoint trước, rồi mới ngừng nhận request; và luôn đóng DB **sau** khi worker xử lý xong job cuối. Kể cả vậy, consumer vẫn phải idempotent vì SIGKILL luôn có thể xảy ra.

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

## 22. Testing

### Ba tầng test

```text
        /\          E2E:  HTTP -> service -> DB -> Redis -> response
       /  \         ít, chậm, giòn; chỉ cho happy path quan trọng nhất
      /----\
     /      \       Integration: service + Postgres/Redis/RabbitMQ thật (testcontainers)
    /        \      vừa phải; test transaction, query, migration, index
   /----------\
  /            \    Unit: service + fake repository, không I/O
 /              \   nhiều nhất, mili giây, chạy mọi commit
/________________\
```

**Unit test** — dùng fake interface, không chạm DB:

```go
func TestCreateOrder_InsufficientStock(t *testing.T) {
    repo := &FakeInventory{stock: 0}
    svc := NewOrderService(repo, &FakeGateway{})

    _, err := svc.Create(context.Background(), OrderRequest{Qty: 1})

    require.ErrorIs(t, err, ErrOutOfStock)
}
```

**Integration test** — dùng dependency thật, để bắt những thứ mock không bao giờ bắt được: SQL sai cú pháp, thiếu index, transaction không rollback, unique constraint. Dùng `testcontainers-go` để spin Postgres/Redis thật trong Docker, tự dọn sau khi test xong.

Nói được ranh giới này là điểm cộng lớn:

> Mock repository bắt được logic nghiệp vụ, nhưng không bao giờ bắt được lỗi query hay deadlock DB. Vì vậy tôi giữ unit test cho domain logic, và bắt buộc integration test cho mọi thứ chạm DB/queue.

### Table-driven test — chuẩn mực của Go

```go
func TestCalculateDiscount(t *testing.T) {
    tests := []struct {
        name    string
        total   int
        tier    string
        want    int
        wantErr error
    }{
        {"no discount", 100, "basic", 0, nil},
        {"vip 10 percent", 1000, "vip", 100, nil},
        {"negative total", -1, "vip", 0, ErrInvalidTotal},
        {"zero total", 0, "basic", 0, nil}, // edge case
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := CalculateDiscount(tt.total, tt.tier)
            require.ErrorIs(t, err, tt.wantErr)
            require.Equal(t, tt.want, got)
        })
    }
}
```

`t.Run` cho mỗi case một tên riêng → khi fail, output chỉ đúng case đó. Thêm case mới chỉ là thêm một dòng.

### Race test và coverage

```bash
go test -race ./...        # BẮT BUỘC trong CI nếu có concurrency
go test -cover ./...
go test -coverprofile=c.out ./... && go tool cover -html=c.out
```

Coverage là chỉ dấu, không phải mục tiêu. 100% coverage vẫn có thể không assert gì. Điều cần nói: *"tôi ưu tiên cover các nhánh error và edge case, không chạy theo con số"*.

---

## 22.5. Benchmark

```go
func BenchmarkEncodeJSON(b *testing.B) {
    order := makeOrder()
    b.ReportAllocs()
    b.ResetTimer() // loại thời gian setup ra khỏi phép đo

    for i := 0; i < b.N; i++ {
        _, _ = json.Marshal(order)
    }
}
```

```bash
go test -bench=. -benchmem -count=10 ./...
```

Đọc kết quả:

```text
BenchmarkEncodeJSON-8   250000   4821 ns/op   1536 B/op   15 allocs/op
                      │        │           │            └─ số lần allocate mỗi op
                      │        │           └─ số byte allocate mỗi op
                      │        └─ thời gian mỗi op (con số hay bị nhìn nhất)
                      └─ số lần chạy runtime tự chọn (b.N)
```

Trong tối ưu, **`allocs/op` thường quan trọng hơn `ns/op`**: mỗi allocation không chỉ tốn lúc cấp phát mà còn tạo áp lực GC làm chậm *toàn bộ* service về sau — thứ mà benchmark một hàm đơn lẻ không thấy được.

Ví dụ cải thiện thật:

```text
Before (bytes.Buffer mới mỗi request):  4821 ns/op   1536 B/op   15 allocs/op
After  (sync.Pool):                     2140 ns/op    256 B/op    3 allocs/op
```

So sánh có thống kê, đừng so bằng mắt:

```bash
go test -bench=. -count=10 > old.txt
# ... sửa code ...
go test -bench=. -count=10 > new.txt
benchstat old.txt new.txt   # cho biết chênh lệch có ý nghĩa thống kê hay chỉ là nhiễu
```

Bẫy: nếu compiler thấy kết quả không được dùng, nó có thể tối ưu bỏ hẳn code cần đo. Gán kết quả vào biến package-level để chặn:

```go
var sink []byte

func BenchmarkX(b *testing.B) {
    var r []byte
    for i := 0; i < b.N; i++ {
        r, _ = json.Marshal(order)
    }
    sink = r
}
```

---

## 22.6. Profiling với pprof

Bật endpoint (chỉ trên port nội bộ, **không** expose ra internet):

```go
import _ "net/http/pprof"

go func() {
    log.Println(http.ListenAndServe("localhost:6060", nil))
}()
```

### Năm loại profile và câu hỏi chúng trả lời

| Profile | Trả lời câu hỏi | Lệnh |
| --- | --- | --- |
| **CPU** | CPU đang cháy ở hàm nào? | `go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30` |
| **Heap** | Ai đang giữ memory / allocate nhiều? | `go tool pprof http://localhost:6060/debug/pprof/heap` |
| **Goroutine** | Goroutine đang kẹt ở đâu? Có leak không? | `go tool pprof http://localhost:6060/debug/pprof/goroutine` |
| **Block** | Goroutine chờ channel/mutex ở đâu, bao lâu? | cần `runtime.SetBlockProfileRate(1)` |
| **Mutex** | Lock nào bị tranh chấp nhiều nhất? | cần `runtime.SetMutexProfileFraction(1)` |

Block và mutex profile **tắt mặc định** vì có overhead — bật khi cần điều tra, và biết điều này là điểm cộng.

### Các lệnh trong pprof

```text
(pprof) top          # 10 hàm tốn nhiều nhất
(pprof) top -cum     # sắp theo cumulative — thấy được cả call chain, thường hữu ích hơn
(pprof) list Charge  # xem chi phí TỪNG DÒNG trong hàm Charge — đây là lệnh giá trị nhất
(pprof) web          # mở call graph dạng SVG
(pprof) peek Marshal # ai gọi Marshal, Marshal gọi ai
```

Với heap profile, phân biệt hai chế độ — nhầm chỗ này là điều tra sai hướng:

* `-inuse_space` (mặc định): memory **đang** được giữ → dùng khi điều tra **memory leak**.
* `-alloc_space`: tổng memory đã allocate từ đầu, kể cả đã được GC thu → dùng khi điều tra **GC pressure / CPU cao vì GC**.

### Trace — khi pprof không đủ

```bash
curl -o trace.out 'http://localhost:6060/debug/pprof/trace?seconds=5'
go tool trace trace.out
```

Trace cho thấy timeline chi tiết: goroutine nào chạy trên P nào, GC pause bao lâu, syscall block ở đâu, goroutine chờ bao lâu trước khi được schedule. Dùng khi CPU không cao mà latency vẫn tệ — thường là do blocking hoặc scheduler, thứ mà CPU profile không lộ ra.

---

## 22.7. Production debugging playbook

Đây là phần interviewer dùng để kiểm tra bạn có thật sự vận hành service hay không. Nguyên tắc xuyên suốt: **metrics → logs → pprof → trace**. Luôn đi từ tín hiệu rộng tới công cụ hẹp, không nhảy thẳng vào pprof.

### Triệu chứng 1 — CPU 100%

```text
1. Metrics: request rate có tăng đột biến không?
   -> Có: có thể chỉ là traffic thật, cần scale.
   -> Không: CPU cao mà traffic không đổi -> có bug.

2. CPU profile 30s:
   go tool pprof http://.../debug/pprof/profile?seconds=30
   (pprof) top -cum

3. Đọc kết quả:
   - runtime.gcBgMarkWorker chiếm nhiều -> vấn đề là ALLOCATION, không phải logic.
     -> chuyển sang heap profile -alloc_space, tìm chỗ allocate nóng.
   - regexp / json / encoding chiếm nhiều -> hot path chưa tối ưu
     (compile regex mỗi request? marshal object khổng lồ?)
   - hàm của mình chiếm nhiều -> list <func> xem dòng nào tốn
   - runtime.selectgo / chan -> busy-wait loop (select có default trong for)
```

### Triệu chứng 2 — memory tăng liên tục, không bao giờ giảm

```text
1. Metrics: goroutine count có tăng cùng nhịp không?
   -> CÓ  => gần như chắc chắn GOROUTINE LEAK, không phải memory leak.
             curl '.../debug/pprof/goroutine?debug=2' -> xem stack, tìm chỗ block.
   -> KHÔNG => memory leak thật, sang bước 2.

2. Heap profile -inuse_space, lấy 2 lần cách nhau 30 phút rồi so sánh:
   go tool pprof -base heap1.out heap2.out
   -> thấy chính xác cái gì tăng lên giữa hai thời điểm.

3. Nghi phạm quen mặt:
   - cache in-memory không có TTL/eviction (map chỉ lớn dần)
   - sub-slice giữ mảng lớn (big[:10])
   - ticker không Stop
   - append vào slice package-level mãi mãi
   - prepared statement / connection không close
```

### Triệu chứng 3 — latency spike (p99 xấu, p50 vẫn đẹp)

```text
p50 tốt + p99 tệ => KHÔNG phải code chậm đều, mà là có gì đó thỉnh thoảng bị CHỜ.

1. Chia nhỏ latency theo tầng (cần tracing hoặc metric riêng):
   HTTP total = handler + DB + Redis + external API
   -> tầng nào phình ra?

2. DB: connection pool saturation là nghi phạm số một.
   Expose db.Stats(): WaitCount, WaitDuration.
   WaitDuration tăng -> request đang XẾP HÀNG chờ connection, không phải query chậm.
   -> tăng MaxOpenConns (nếu DB chịu được) hoặc giảm concurrency phía app.

3. GC pause: xem GODEBUG=gctrace=1 hoặc metric gc_pause.
   Pause dài -> heap quá lớn hoặc allocate quá nhiều -> quay lại heap profile.

4. Không thấy gì ở trên -> go tool trace:
   goroutine bị chờ schedule (do GOMAXPROCS sai trong container?),
   hay bị block ở mutex (mutex profile)?

5. Lock contention: mutex profile. Một mutex global bảo vệ cache dùng chung
   có thể serialize toàn bộ request dù CPU rảnh.
```

### Triệu chứng 4 — goroutine tăng bất thường

Đã nói ở mục Goroutine leak. Quy trình ngắn: `curl '.../debug/pprof/goroutine?debug=2'` → tìm nhóm goroutine đông nhất → nhìn dòng cuối stack trace, nó đang block ở đâu (`chan send`, `chan receive`, `select`, `sync.Mutex.Lock`) → mở đúng file:line đó ra đọc.

### Cách trình bày trong phỏng vấn

> Tôi luôn bắt đầu từ metrics vì nó cho biết *triệu chứng* và *thời điểm*. Ví dụ khi memory tăng, câu hỏi đầu tiên của tôi là "goroutine count có tăng theo không" — nếu có thì đó là goroutine leak chứ không phải memory leak, và hai thứ đó điều tra hoàn toàn khác nhau. Sau khi khoanh vùng, tôi mới dùng pprof để tìm chính xác dòng code, và dùng `-base` để diff hai heap profile theo thời gian.

---

## 23. Package standard library và x/ hay dùng trong production

Interviewer hay hỏi "package nào em dùng nhiều nhất" để đo kinh nghiệm thật.

| Package | Dùng để làm gì |
| --- | --- |
| `context` | cancel, timeout, request-scoped value — xuất hiện ở mọi signature |
| `sync` | `Mutex`, `RWMutex`, `WaitGroup`, `Once`, `Pool` |
| `sync/atomic` | counter, flag, hot-swap config pointer |
| `golang.org/x/sync/errgroup` | chạy song song có error propagation + cancel |
| `golang.org/x/sync/singleflight` | chống cache stampede |
| `golang.org/x/sync/semaphore` | giới hạn concurrency có trọng số |
| `time` | timeout, ticker, backoff, deadline |
| `net/http` | server, client, middleware, `Shutdown` |
| `database/sql` | pool, `QueryContext`, `Tx` |
| `encoding/json` | serialize; biết cả `json.Decoder` cho stream lớn |
| `errors` | `Is`, `As`, `Join`, wrap với `%w` |
| `os/signal` | `signal.NotifyContext` cho graceful shutdown |
| `net/http/pprof` | profiling endpoint |
| `runtime` | `NumGoroutine`, `GOMAXPROCS`, `ReadMemStats` |
| `testing` | test, benchmark, fuzzing |
| `log/slog` | structured logging (chuẩn từ Go 1.21) |
| `slices`, `maps` | helper generic (Go 1.21+): `Clone`, `Contains`, `SortFunc` |

Trả lời tốt không phải là đọc thuộc danh sách, mà là gắn package với một vấn đề cụ thể:

> Dùng nhiều nhất là `context` và `sync`. `errgroup` thì tôi dùng ở chỗ gọi song song 3-4 downstream service trong một request — nó cho tôi cancel toàn bộ khi một cái fail, thứ mà `WaitGroup` không làm được. `singleflight` thì tôi thêm vào sau một sự cố cache stampede làm DB spike lúc TTL của hot key hết hạn.

---

## 24. Câu hỏi phỏng vấn dễ gặp

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

### Slice truyền vào function, append bên trong, caller có thấy không?

Có thể thấy hoặc không — tùy capacity. Nếu còn cap thì phần tử được ghi vào mảng chung, nhưng caller vẫn không "thấy" vì `len` của header ở caller không đổi. Nếu hết cap thì allocate mảng mới, hai bên tách rời hẳn. Vì vậy `append` luôn phải gán lại: `s = append(s, x)`.

### Biến nào nằm trên stack, biến nào lên heap?

Compiler quyết định bằng escape analysis, không phải bằng `&` hay `new`. Nếu pointer thoát khỏi function (return, gán vào struct trên heap, truyền vào `any`, gửi qua channel, capture bởi goroutine) thì biến lên heap. Kiểm chứng bằng `go build -gcflags="-m"`.

### `WaitGroup` và `errgroup` khác nhau ở đâu?

`WaitGroup` chỉ đếm và chờ, không biết error và không hủy được gì. `errgroup.WithContext` trả về error đầu tiên và cancel context để các goroutine còn lại tự dừng — dùng khi một task fail làm các task còn lại trở nên vô nghĩa (upload ảnh + tạo thumbnail + ghi DB).

### Vì sao concurrent map lại làm chết cả process?

Vì đó là `fatal error`, không phải `panic` — runtime chủ động abort và `recover()` không bắt được. Recovery middleware của Gin cũng vô dụng. Đây là lý do map dùng chung bắt buộc phải có `Mutex`/`RWMutex` hoặc dùng `sync.Map`.

### `context.Value` nên chứa gì?

Chỉ request-scoped metadata xuyên tầng: `request_id`, `trace_id`, `user_id`, JWT claims, tenant id. Không dùng cho tham số nghiệp vụ (`page`, `limit`, `sort`) — những thứ đó phải nằm trong signature để có type safety. Và luôn dùng key có type riêng (unexported struct) để tránh va chạm giữa các package.

### Có "race lành tính" không?

Không. Theo Go memory model, race là hành vi không xác định: compiler và CPU được phép reorder, và không có bảo đảm goroutine khác bao giờ nhìn thấy giá trị mới. Code có race có thể chạy đúng hàng triệu lần rồi sai đúng lúc peak. Cứ shared mutable state là phải có mutex, atomic, hoặc channel.

### `sync.Pool` có phải là cache không?

Không. `sync.Pool` bị GC dọn sạch sau mỗi chu kỳ GC, nên không được dùng để lưu dữ liệu. Nó chỉ để tái sử dụng object tạm (buffer) nhằm giảm allocation. Và bắt buộc `Reset()` object khi lấy ra, nếu không dữ liệu request trước sẽ rò rỉ sang request sau.

### Goroutine có bằng thread không? Có chạy song song không?

Không. Goroutine là concurrency primitive được runtime scheduler ghép lên OS thread (mô hình GMP). Có chạy song song thật hay không phụ thuộc `GOMAXPROCS` và số CPU core. Trong container, `GOMAXPROCS` mặc định lấy theo core của node chứ không theo CPU limit — nên cần `automaxprocs` để tránh throttling.
