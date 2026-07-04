# gRPC Middle Backend Notes

File này gom kiến thức gRPC ở mức Middle Backend: đủ để giải thích trong phỏng vấn, thiết kế internal service communication, và tránh lỗi compatibility khi dùng Protocol Buffers.

gRPC không chỉ là "REST nhanh hơn". Điểm chính là contract bằng protobuf, HTTP/2, deadline/cancellation, status code rõ, streaming và code generation.

---

## 1. gRPC là gì?

gRPC là framework RPC dùng HTTP/2 và Protocol Buffers.

Luồng cơ bản:

```text
client gọi method trên generated client
-> gRPC serialize request bằng protobuf
-> gửi qua HTTP/2
-> server handler xử lý
-> trả response/status/trailer
```

Hợp với:

* internal microservices;
* service-to-service communication;
* contract rõ giữa team;
* low-latency binary serialization;
* streaming;
* code generation cho nhiều ngôn ngữ.

Không luôn hợp với:

* public API cho browser;
* third-party integration muốn curl/debug đơn giản;
* team chưa có kỷ luật schema compatibility;
* payload cần human-readable/debug trực tiếp.

---

## 2. gRPC vs REST

| Tiêu chí | REST | gRPC |
|---|---|---|
| Protocol | HTTP/1.1 hoặc HTTP/2 | HTTP/2 |
| Payload | JSON thường gặp | Protobuf binary |
| Contract | OpenAPI nếu có | `.proto` là contract chính |
| Browser support | Tự nhiên | Cần gRPC-Web/proxy nếu browser gọi trực tiếp |
| Debug | curl/Postman dễ | Cần grpcurl/BloomRPC/Evans/tooling |
| Streaming | Có thể nhưng không chuẩn bằng | Built-in |
| Internal service | Dùng được | Rất hợp |
| Public API | Rất hợp | Cần cân nhắc |

Câu trả lời ngắn:

> REST hợp public API và integration dễ debug. gRPC hợp internal microservices cần contract rõ, latency tốt, codegen và streaming. Tôi không chọn gRPC chỉ vì "nhanh hơn"; tôi chọn khi team cần typed contract và service-to-service communication ổn định.

---

## 3. Protobuf Cơ Bản

Ví dụ:

```proto
syntax = "proto3";

package order.v1;

service OrderService {
  rpc GetOrder(GetOrderRequest) returns (GetOrderResponse);
}

message GetOrderRequest {
  string order_id = 1;
}

message GetOrderResponse {
  string order_id = 1;
  string status = 2;
}
```

Điểm quan trọng:

* Field number quan trọng hơn field name trong binary format.
* Không được đổi field number tùy tiện.
* Generated code tạo client/server stub.
* `.proto` nên được review như API contract.

---

## 4. 4 Kiểu RPC

### Unary

Request/response bình thường.

```proto
rpc GetOrder(GetOrderRequest) returns (GetOrderResponse);
```

Hợp với đa số internal API.

### Server streaming

Client gửi 1 request, server trả nhiều response.

```proto
rpc WatchOrder(WatchOrderRequest) returns (stream OrderEvent);
```

Hợp với:

* realtime update;
* log/progress stream;
* result lớn trả dần.

### Client streaming

Client gửi nhiều request, server trả 1 response.

```proto
rpc UploadChunks(stream UploadChunkRequest) returns (UploadResult);
```

Hợp với upload/chunking hoặc batch gửi dần.

### Bidirectional streaming

Hai bên gửi stream đồng thời.

```proto
rpc Chat(stream ChatMessage) returns (stream ChatMessage);
```

Hợp với realtime interaction, nhưng complexity cao hơn: backpressure, ordering, cancellation, reconnect.

---

## 5. Deadline, Timeout Và Cancellation

gRPC deadline là thời điểm cuối cùng request được phép chạy. Client set deadline, server nhận được qua context.

```text
client deadline = now + 300ms
-> server xử lý quá 300ms
-> client nhận DeadlineExceeded
-> server context bị cancel
```

Trong Go, handler nên truyền `ctx` xuống DB/Redis/HTTP call:

```go
func (s *Server) GetOrder(ctx context.Context, req *pb.GetOrderRequest) (*pb.GetOrderResponse, error) {
    order, err := s.repo.FindOrder(ctx, req.OrderId)
    if err != nil {
        return nil, err
    }
    return &pb.GetOrderResponse{OrderId: order.ID}, nil
}
```

Điểm phỏng vấn:

* Client phải set deadline, không để call treo vô hạn.
* Server phải tôn trọng `ctx.Done()`.
* Deadline nên nằm trong timeout budget toàn request.
* Retry phải xét deadline còn lại.

Deadline khác timeout thế nào?

* Timeout thường là duration, ví dụ 300ms.
* Deadline là thời điểm cụ thể, ví dụ 10:00:00.300.
* gRPC truyền deadline qua service boundary tốt hơn timeout local rời rạc.

---

## 6. Status Code

gRPC dùng status code riêng, không dùng HTTP status làm business code chính.

| Code | Khi dùng |
|---|---|
| `InvalidArgument` | Input sai format/rule |
| `NotFound` | Resource không tồn tại |
| `AlreadyExists` | Duplicate unique resource |
| `FailedPrecondition` | State hiện tại không cho phép action |
| `PermissionDenied` | Đã xác thực nhưng không có quyền |
| `Unauthenticated` | Chưa xác thực/token sai |
| `ResourceExhausted` | Quota/rate limit/concurrency limit |
| `Unavailable` | Service/dependency tạm không sẵn sàng |
| `DeadlineExceeded` | Quá deadline |
| `Canceled` | Client/server cancel |
| `Internal` | Lỗi không mong muốn |

Mapping nên nhất quán:

* validation error -> `InvalidArgument`;
* missing resource -> `NotFound`;
* duplicate create -> `AlreadyExists`;
* business state sai -> `FailedPrecondition`;
* dependency down tạm thời -> `Unavailable`;
* unknown bug -> `Internal`.

---

## 7. Error Detail

Không nên chỉ trả string lỗi tùy tiện. Với lỗi cần machine-readable detail, có thể dùng structured error detail.

Ví dụ logic:

```text
code = InvalidArgument
message = validation failed
details = field violations
```

Best practice:

* message không lộ secret/internal SQL;
* code ổn định để client handle;
* log nội bộ có request id/correlation id;
* lỗi domain map rõ sang status code.

---

## 8. Interceptor

Interceptor tương tự middleware.

Dùng cho:

* auth;
* logging;
* tracing;
* metrics;
* panic recovery;
* rate limit;
* deadline/timeout guard;
* validation;
* retry ở client side.

Có 2 loại chính:

* Unary interceptor.
* Stream interceptor.

Thứ tự interceptor quan trọng. Ví dụ auth nên chạy trước business handler, recovery nên bọc ngoài để bắt panic.

---

## 9. Metadata

Metadata giống headers trong HTTP.

Dùng cho:

* authorization token;
* request id/correlation id;
* tenant id;
* locale;
* feature flag context.

Lưu ý:

* Không nhét business payload lớn vào metadata.
* Không tin metadata từ client nếu chưa auth/validate.
* Cần propagate request id qua service boundary.

---

## 10. Protobuf Compatibility

Đây là phần rất hay hỏi.

Rules quan trọng:

* Không đổi field number.
* Không đổi meaning của field cũ.
* Không reuse field number đã xóa.
* Không reuse field name đã xóa nếu có thể gây hiểu nhầm.
* Dùng `reserved` cho field/name đã xóa.
* Thêm field mới theo hướng optional/backward-compatible.
* Consumer phải chịu được unknown field/enum value.

Ví dụ đúng:

```proto
message Order {
  string id = 1;
  string status = 2;

  reserved 3;
  reserved "old_total";

  string currency = 4;
}
```

Không làm:

```proto
message Order {
  string id = 1;
  int64 status = 2; // sai nếu trước đây status là string
  string new_field = 3; // sai nếu 3 từng là field cũ đã xóa
}
```

Vì binary payload cũ có thể bị decode sai.

---

## 11. Enum Compatibility

Enum dễ làm client cũ lỗi nếu không xử lý unknown.

Pattern:

```proto
enum OrderStatus {
  ORDER_STATUS_UNSPECIFIED = 0;
  ORDER_STATUS_CREATED = 1;
  ORDER_STATUS_PAID = 2;
  ORDER_STATUS_CANCELLED = 3;
}
```

Best practice:

* Giá trị đầu tiên nên là `UNSPECIFIED = 0`.
* Client nên có default handler cho unknown value.
* Không đổi meaning của enum number cũ.
* Không reuse enum number đã xóa.

---

## 12. Pagination Trong gRPC

Không nên trả list không giới hạn.

```proto
message ListOrdersRequest {
  int32 page_size = 1;
  string page_token = 2;
}

message ListOrdersResponse {
  repeated Order orders = 1;
  string next_page_token = 2;
}
```

`page_token` có thể encode cursor như `(created_at, id)`.

Middle backend nên nói được:

* page size limit;
* cursor pagination thay vì offset lớn;
* stable ordering;
* token không nên lộ quá nhiều internal detail nếu public.

---

## 13. Streaming Production Concerns

Streaming mạnh nhưng dễ lỗi hơn unary.

Cần nghĩ:

* flow control/backpressure;
* client disconnect;
* server shutdown;
* message size limit;
* heartbeat/keepalive;
* auth/token expiry trong long-lived stream;
* retry/reconnect;
* ordering;
* memory nếu stream chậm.

Với Middle Backend, nên nói:

> Tôi ưu tiên unary nếu workflow đơn giản. Streaming chỉ dùng khi cần dữ liệu trả dần, realtime hoặc upload nhiều chunk. Khi dùng streaming, tôi phải xử lý cancellation, backpressure, message size và reconnect.

---

## 14. Retry Trong gRPC

Retry chỉ nên làm cho operation idempotent hoặc read-only.

Có thể retry:

* read query;
* idempotent create/update có idempotency key;
* dependency transient `Unavailable`.

Không nên retry tự động:

* payment charge không có idempotency key;
* operation tạo side effect không idempotent;
* validation/auth error.

Retry cần:

* max attempts;
* exponential backoff;
* jitter;
* tôn trọng deadline còn lại;
* phân biệt status code retryable.

---

## 15. Load Balancing Và Service Discovery

gRPC dùng HTTP/2 connection dài. Load balancing có thể khác REST vì nhiều request multiplex trên một connection.

Cần nghĩ:

* client-side load balancing hoặc proxy như Envoy;
* service discovery;
* connection pooling;
* health check;
* keepalive;
* graceful shutdown để không cắt stream/request đang chạy.

Nếu dùng Kubernetes, thường cần hiểu load balancing qua service/proxy có thể không chia đều như mong đợi với long-lived HTTP/2 connection.

---

## 16. Observability

Metrics nên có:

* request count theo method;
* latency p50/p95/p99 theo method;
* status code count;
* deadline exceeded count;
* payload size;
* in-flight requests/streams;
* retry count;
* dependency latency bên trong handler.

Log nên có:

* method name;
* request id/correlation id;
* user/tenant nếu phù hợp;
* status code;
* duration;
* error reason đã sanitize.

Tracing giúp thấy call chain qua nhiều service.

---

## 17. Security

gRPC production vẫn cần đầy đủ security:

* TLS/mTLS giữa services nếu cần;
* auth qua metadata;
* authorization trong handler;
* input validation;
* max message size;
* rate/concurrency limit;
* không log payload chứa secret;
* reflection chỉ bật có kiểm soát.

---

## 18. Lỗi Thiết Kế Thường Gặp

* Không set deadline, call treo vô hạn.
* Không truyền context xuống DB/HTTP client.
* Trả mọi lỗi là `Internal`.
* Đổi protobuf field number.
* Reuse field number đã xóa.
* Không xử lý unknown enum.
* Dùng streaming khi unary đủ.
* Không có message size limit.
* Không có observability theo method/status.
* Retry operation non-idempotent.
* Expose gRPC public mà không tính browser/tooling compatibility.

---

## 19. Câu Trả Lời Phỏng Vấn Mẫu

### gRPC là gì và dùng khi nào?

> gRPC là RPC framework dùng HTTP/2 và Protocol Buffers. Tôi dùng gRPC chủ yếu cho internal service-to-service communication khi cần contract rõ, code generation, latency tốt hoặc streaming. Với public API/browser/third-party integration, REST thường dễ dùng và debug hơn.

### Deadline trong gRPC quan trọng thế nào?

> Deadline giúp request không treo vô hạn và được truyền qua service boundary. Client set deadline, server nhận qua context và phải truyền context xuống DB/HTTP/gRPC call khác. Khi hết deadline, client nhận `DeadlineExceeded`, server nên dừng xử lý nếu context bị cancel.

### Protobuf backward compatibility gồm gì?

> Không đổi field number, không reuse field number đã xóa, dùng `reserved` cho field cũ, thêm field mới theo hướng optional/backward-compatible, và client phải chịu được unknown enum/field. Field number là contract binary nên đổi bừa có thể làm payload cũ bị decode sai.

### Interceptor dùng để làm gì?

> Interceptor giống middleware trong gRPC. Tôi dùng cho auth, logging, metrics, tracing, recovery, validation, rate limit hoặc client-side retry. Cần tách unary và stream interceptor, và chú ý thứ tự chạy.

### gRPC streaming cần cẩn thận gì?

> Streaming cần xử lý cancellation, backpressure, message size, reconnect, auth/token expiry và graceful shutdown. Nếu workflow chỉ request/response đơn giản, tôi ưu tiên unary để giảm complexity.
