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

---

## 20. Vì Sao Protobuf Decode Nhanh Hơn JSON

Câu hỏi hay gặp: "decode protobuf vào struct có nhanh hơn decode JSON không, có phải vì binary và field đánh số không?" Ngắn gọn: **thường nhanh hơn**, và đúng là do binary + field number, nhưng cần hiểu chính xác lý do.

### JSON decode phải làm gì

JSON là text. Để decode `{"order_id":"123","status":"PAID"}` vào struct, runtime phải:

1. **Tokenize text**: quét từng byte tìm `{`, `"`, `:`, `,`, phân biệt string/number/bool.
2. **Parse key là string**: đọc `"order_id"` thành chuỗi.
3. **Match key với field struct**: so sánh string key với tên field. Trong Go, `encoding/json` mặc định dùng **reflection** để map key -> field, khá chậm.
4. **Parse value từ ASCII**: số `123` đang là text `'1' '2' '3'`, phải convert ASCII -> int.
5. **Cấp phát** cho các string key/value trung gian.

Việc so string key và parse số từ text là phần tốn.

### Protobuf decode làm gì

Wire format protobuf là chuỗi các cặp `(tag, value)`. Tag = `(field_number << 3) | wire_type`, mã hoá bằng varint. Ví dụ field `order_id = 1` không gửi chữ `"order_id"`, chỉ gửi số tag.

Khi decode:

1. Đọc **tag (một số nguyên)**, không có string key.
2. `switch` theo field number -> gán thẳng vào field đã biết kiểu trong struct (code generated, **không reflection**).
3. Số được lưu **binary** (varint/fixed), không cần parse từ ASCII.
4. Không cần cấp phát cho tên field.

```text
JSON:      "status":"PAID"      -> tokenize + match "status" + parse
Protobuf:  0x12 0x04 P A I D     -> tag=field 2, đọc 4 byte
```

### Vì sao nhanh hơn

* **Không tokenize text**, không so sánh string key.
* **Field number (integer tag)** thay cho field name -> match bằng số, payload nhỏ hơn.
* **Số ở dạng binary** (varint), không convert ASCII.
* **Generated code** gán trực tiếp vào field có kiểu tĩnh, tránh reflection/map.
* **Payload nhỏ hơn** -> ít byte phải đọc và ít băng thông (đây là lợi ích transport, tách khỏi tốc độ decode).

### Nói cho đúng (đừng overclaim)

* Không phải lúc nào cũng nhanh "gấp nhiều lần". Với payload nhỏ, chênh lệch có thể không đáng kể.
* JSON có thư viện tối ưu mạnh (ví dụ simdjson, hoặc codegen JSON không dùng reflection) thu hẹp khoảng cách.
* Lợi ích lớn nhất thấy rõ khi message nhiều field, nested, số nhiều, hoặc throughput cao.
* Đánh đổi: protobuf khó debug bằng mắt, cần schema và tooling. JSON human-readable, curl được ngay.

Câu trả lời phỏng vấn:

> Protobuf decode thường nhanh hơn JSON vì nó là binary: field được đánh số (integer tag) thay vì tên string nên không phải tokenize text và so sánh key, số lưu dạng varint không cần parse từ ASCII, và code generated gán thẳng vào struct đã biết kiểu nên tránh reflection. Payload cũng nhỏ hơn. Nhưng tôi không nói protobuf luôn nhanh gấp nhiều lần — với payload nhỏ hoặc JSON lib tối ưu thì khoảng cách hẹp lại; đánh đổi là mất tính human-readable và cần schema/tooling.

---

## 21. HTTP/2 Giải Thích Kỹ

gRPC bắt buộc chạy trên HTTP/2. Muốn hiểu gRPC phải hiểu HTTP/2.

### Khái niệm cốt lõi

* **Frame**: đơn vị nhỏ nhất, dạng binary. Có nhiều loại: `HEADERS`, `DATA`, `SETTINGS`, `WINDOW_UPDATE`, `RST_STREAM`, `PING`, `GOAWAY`...
* **Message**: một request hoặc response, gồm nhiều frame (thường 1 `HEADERS` + nhiều `DATA`).
* **Stream**: một luồng hai chiều mang các frame, có `stream id`. Mỗi request/response là một stream riêng trên cùng một connection.
* **Connection**: một TCP connection duy nhất chứa nhiều stream song song.

```text
1 TCP connection
├── stream 1: HEADERS + DATA  (request A)  ┐ interleave
├── stream 3: HEADERS + DATA  (request B)  │ các frame
└── stream 5: HEADERS + DATA  (request C)  ┘ đan xen nhau
```

### Các tính năng chính

1. **Binary framing**: giao thức nhị phân, không phải text như HTTP/1.1. Parse nhanh và ít lỗi hơn.
2. **Multiplexing**: nhiều request/response cùng lúc trên **một** connection, các frame đan xen theo stream id. Không cần mở nhiều TCP connection.
3. **Header compression (HPACK)**: header lặp lại (host, user-agent, cookie...) được nén và dùng bảng index, giảm byte thừa mỗi request.
4. **Stream prioritization**: client có thể gợi ý stream nào ưu tiên.
5. **Flow control**: kiểm soát lưu lượng theo từng stream và cả connection bằng `WINDOW_UPDATE` (đây là nền tảng cho backpressure của gRPC streaming).
6. **Server push**: server chủ động đẩy resource (thực tế nay gần như bỏ, browser đã deprecate).

### Head-of-line blocking

* **HTTP/1.1**: mỗi connection xử lý tuần tự; một response chậm chặn các response sau (HOL blocking ở tầng HTTP). Browser lách bằng cách mở ~6 TCP connection/host.
* **HTTP/2**: giải quyết HOL ở **tầng HTTP** nhờ multiplex nhiều stream. Nhưng **vẫn còn HOL ở tầng TCP**: TCP là ordered stream, mất một gói khiến mọi stream phía trên phải chờ retransmit.
* **HTTP/3 (QUIC trên UDP)**: giải quyết luôn HOL tầng transport vì mỗi stream độc lập. (Ngoài phạm vi gRPC cơ bản nhưng nên biết.)

---

## 22. HTTP/1.1 vs HTTP/2 (Và Vì Sao gRPC Cần HTTP/2)

| Tiêu chí | HTTP/1.1 | HTTP/2 |
|---|---|---|
| Định dạng | Text | Binary framing |
| Số request đồng thời/1 connection | 1 tại một thời điểm (pipelining kém) | Nhiều stream multiplex |
| Cách parallel | Mở nhiều TCP connection (~6/host) | 1 connection, nhiều stream |
| Header | Text, lặp lại mỗi request | Nén bằng HPACK |
| HOL blocking tầng HTTP | Có | Không |
| HOL blocking tầng TCP | Có | Vẫn còn (HTTP/3 mới hết) |
| Flow control theo stream | Không | Có (`WINDOW_UPDATE`) |
| Trailer sau body | Hạn chế | Hỗ trợ tốt |

### Vì sao gRPC cần HTTP/2

* **Streaming**: 4 kiểu RPC (unary, server/client/bidi streaming) map tự nhiên vào stream và nhiều `DATA` frame của HTTP/2.
* **Multiplexing**: nhiều RPC chạy song song trên một connection dài, hợp mô hình service-to-service.
* **Trailer cho status**: gRPC trả `grpc-status`/`grpc-message` trong **trailer** (header gửi *sau* body). HTTP/1.1 không truyền trailer tiện lợi; HTTP/2 làm được.
* **Flow control**: backpressure của streaming dựa trực tiếp trên flow control của HTTP/2.

Chính vì phụ thuộc những đặc tính này (nhất là trailer và kiểm soát frame) mà gRPC "thuần" không chạy được ở nơi không cho thao tác HTTP/2 mức thấp — như browser.

---

## 23. Browser/FE Có Gọi gRPC Trực Tiếp Được Không?

Câu hỏi: ReactJS có call gRPC được không, vì sao browser không hiểu gRPC thuần?

### Vì sao browser không dùng được gRPC thuần

gRPC cần thao tác ở tầng HTTP/2 mà **JavaScript trong browser không được phép chạm tới**:

* Browser (fetch/XHR) **không expose HTTP/2 frame thô**, không cho đọc/ghi trailer, không kiểm soát framing.
* gRPC dựa vào **trailer** để trả status -> browser API không đọc được trailer đúng cách.
* JS không ép được kết nối phải là HTTP/2 hay điều khiển stream.

Nên dù browser có thể *chạy trên* HTTP/2, nó vẫn thiếu API tầng thấp mà gRPC cần.

### Giải pháp: gRPC-Web

* **gRPC-Web** là biến thể của gRPC dành cho browser, dùng được với `fetch`/XHR.
* Cần **proxy dịch** ở giữa (thường là **Envoy** hoặc `grpc-web` proxy) để chuyển gRPC-Web <-> gRPC thật cho backend.
* Hạn chế: hỗ trợ tốt **unary** và **server streaming**; **client streaming** và **bidirectional streaming** không hỗ trợ (hoặc rất hạn chế).

```text
React (gRPC-Web client)
  -> HTTP/1.1 hoặc HTTP/2 (fetch)
  -> Envoy / grpc-web proxy   (dịch giao thức)
  -> gRPC service (HTTP/2 thuần)
```

### Lựa chọn khác: Connect (connect-web / Connect protocol)

* **Connect** (của Buf) là giao thức thân thiện browser hơn: chạy trên HTTP/1.1 lẫn HTTP/2, unary có thể là POST thường với body protobuf **hoặc JSON**.
* Ưu điểm: gọi được bằng `fetch`/`curl` cho unary, vẫn tương thích gRPC, ít cần proxy đặc biệt.

### Chốt cho phỏng vấn

> Browser không gọi được gRPC thuần vì fetch/XHR không cho thao tác HTTP/2 mức thấp — đặc biệt là đọc trailer nơi gRPC nhét status code. Muốn FE (React) gọi thì dùng gRPC-Web qua một proxy như Envoy để dịch, và chấp nhận chỉ có unary + server streaming; hoặc dùng Connect protocol thân thiện browser hơn. Vì vậy public API cho browser tôi thường vẫn ưu tiên REST/JSON hoặc gRPC-Web/Connect có chủ đích, không expose gRPC thuần.

---

## 24. So Sánh HTTP (REST/JSON) vs gRPC — Bổ Sung

Bảng ở mục 2 so sánh nhanh REST vs gRPC. Đây là góc nhìn sâu hơn ở tầng giao thức/dữ liệu.

| Khía cạnh | HTTP + REST/JSON | gRPC |
|---|---|---|
| Transport | HTTP/1.1 hoặc HTTP/2 | Bắt buộc HTTP/2 |
| Message format | JSON text (thường) | Protobuf binary |
| Contract | OpenAPI/Swagger (tùy chọn) | `.proto` bắt buộc, sinh code |
| Kích thước payload | Lớn hơn (text, key lặp) | Nhỏ hơn (binary, tag số) |
| Tốc độ serialize/decode | Chậm hơn (parse text, reflection) | Nhanh hơn (binary, codegen) |
| Streaming | Không chuẩn (SSE/WebSocket riêng) | 4 kiểu built-in |
| Status | HTTP status code | gRPC status + trailer |
| Browser | Native | Cần gRPC-Web/Connect + proxy |
| Debug bằng mắt/curl | Dễ | Khó, cần grpcurl/tooling |
| Human-readable | Có | Không |
| Đổi field an toàn | Tùy discipline JSON | Ràng buộc field number rõ ràng |

Khi nào chọn gì:

* **REST/JSON**: public API, third-party integration, cần debug/curl dễ, browser gọi trực tiếp, payload cần human-readable.
* **gRPC**: internal service-to-service, cần typed contract + codegen, latency/throughput tốt, streaming, nhiều ngôn ngữ.

Câu chốt:

> HTTP/REST/JSON thắng ở tính phổ biến, dễ debug và tương thích browser. gRPC thắng ở contract chặt, payload nhỏ, decode nhanh, streaming và multiplex trên HTTP/2. Tôi không chọn theo "cái nào nhanh hơn" mà theo ngữ cảnh: biên public dễ tích hợp thì REST, giao tiếp nội bộ cần typed contract và hiệu năng thì gRPC.

### Con số "nhẹ hơn 7–10 lần" nói sao cho đúng

Hay gặp claim "gRPC nhẹ hơn REST/JSON tới 7–10 lần". Nói được, nhưng phải hiểu bản chất và không đọc như hằng số:

* Con số đến từ **kết hợp Protobuf (payload binary nhỏ) + HTTP/2 (multiplex, HPACK nén header)** so với REST/JSON trên HTTP/1.1.
* Đây là **ước lượng theo benchmark cụ thể**, phụ thuộc payload, số field, mức nested, throughput. Message nhiều field/nested/throughput cao thì chênh rõ; payload nhỏ thì chênh ít.
* "Nhẹ hơn" (kích thước byte) và "nhanh hơn" (latency/CPU decode) là hai thứ khác nhau — cả hai đều cải thiện nhưng do nguyên nhân khác (payload nhỏ vs tránh parse text/reflection).
* So sánh sòng phẳng phải cùng điều kiện: nếu REST cũng chạy HTTP/2 và JSON dùng lib tối ưu, khoảng cách hẹp lại.

Câu chốt an toàn khi phỏng vấn:

> Con số 7–10 lần là ước lượng từ benchmark, đến từ việc Protobuf làm payload nhỏ và HTTP/2 multiplex + nén header. Tôi coi đây là "gRPC thường nhẹ và nhanh hơn đáng kể" chứ không phải hằng số cố định — mức lợi phụ thuộc payload và tải, và phải benchmark trên chính hệ thống của mình.
