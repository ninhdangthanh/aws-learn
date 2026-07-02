# Rate Limit

Rate limit là cơ chế giới hạn số request/action trong một khoảng thời gian. Mục tiêu không chỉ là chặn traffic xấu, mà còn bảo vệ tài nguyên backend, chia quota công bằng và kiểm soát chi phí.

---

## Concept cốt lõi

Một rate limiter thường cần trả lời 5 câu hỏi:

1. **Limit cái gì?** Request, login attempt, upload, export job, token AI, message gửi đi.
2. **Limit ai?** IP, user, tenant, API key, device, endpoint hoặc kết hợp nhiều dimension.
3. **Limit bao nhiêu?** Ví dụ `100 requests/minute`, `10 uploads/hour`, `1 AI export/day`.
4. **Khi vượt limit thì làm gì?** Reject `429`, delay, queue, CAPTCHA, step-up verification hoặc downgrade priority.
5. **State nằm ở đâu?** In-memory, Redis, API gateway, service mesh hoặc third-party gateway.

Rate limit khác timeout/circuit breaker:

* **Rate limit:** chặn caller gửi quá nhiều request.
* **Timeout:** không chờ một request quá lâu.
* **Circuit breaker:** tạm ngưng gọi dependency đang lỗi/chậm.
* **Bulkhead/concurrency limit:** giới hạn số request đang xử lý đồng thời.

Trong hệ thống production, các cơ chế này thường đi cùng nhau.

---

## Rate limit dùng để làm gì?

### Bảo vệ hệ thống khỏi overload

Khi traffic tăng đột biến, backend có thể hết worker, connection pool, CPU, memory hoặc làm database/cache/queue quá tải. Rate limit giúp reject sớm request vượt ngưỡng thay vì để request đi sâu vào service rồi mới fail.

Ví dụ:

* giới hạn `POST /search` vì query nặng;
* giới hạn upload file lớn;
* giới hạn số job export/report chạy đồng thời;
* giới hạn request vào dependency chậm.

### Ngăn abuse/spam

Rate limit thường dùng để giảm:

* brute force login;
* credential stuffing;
* scraping;
* spam register/forgot password;
* DDoS nhẹ hoặc app-level DDoS;
* abuse promotion/referral/free trial.

Với abuse nghiêm trọng, rate limit nên đi cùng CAPTCHA, device fingerprinting, WAF, audit log và rule theo pattern hành vi.

### Đảm bảo fair usage giữa user/tenant

Nếu một user hoặc tenant gọi API quá nhiều, họ có thể làm chậm tenant khác. Rate limit giúp tenant isolation tốt hơn.

Ví dụ:

```text
free tenant:      60 requests/minute
premium tenant:   600 requests/minute
internal service: 3000 requests/minute
```

### Kiểm soát chi phí

Một số API có chi phí theo call hoặc theo dữ liệu xử lý:

* OpenAI/LLM API;
* AWS Lambda, Rekognition, Textract, S3 request;
* third-party payment/shipping/geocoding API;
* SMS/Email/OTP provider.

Rate limit giúp tránh bug, retry storm hoặc user abuse làm chi phí tăng bất ngờ.

### SLA theo gói dịch vụ

Rate limit có thể thể hiện chính sách sản phẩm:

* free tier: quota thấp, burst nhỏ;
* pro tier: quota cao hơn;
* enterprise: quota riêng theo contract;
* internal/admin: rule riêng nhưng vẫn cần limit để tránh bug.

### Bảo vệ dependency và downstream service

Ngay cả khi service hiện tại còn khỏe, downstream như database, Redis, RabbitMQ, gRPC service hoặc third-party API có thể yếu hơn. Rate limit giúp tạo backpressure.

---

## Nên limit theo dimension nào?

Không nên chỉ limit theo IP. Tùy endpoint, có thể kết hợp:

* IP hoặc subnet;
* user ID;
* tenant ID;
* API key/client ID;
* endpoint;
* device/session fingerprint;
* email/phone/domain;
* plan/tier;
* region;
* expensive action, ví dụ `upload`, `export`, `generate-ai-answer`.

Ví dụ key:

```text
rl:login:ip:1.2.3.4
rl:login:account:user_123
rl:api:tenant:t_456:/v1/orders
rl:openai:user:user_123:gpt-4.1
rl:upload:tenant:t_456
```

---

## Đặt rate limit ở đâu?

### Edge/CDN/WAF

Phù hợp để chặn traffic rất sớm:

* bot traffic;
* IP reputation xấu;
* DDoS nhẹ;
* request quá nhiều vào public endpoint;
* geo/ASN/datacenter IP rule.

Ưu điểm là request chưa vào backend. Nhược điểm là khó biết sâu business context như tenant plan, role, quota theo feature.

### API Gateway hoặc reverse proxy

Phù hợp cho:

* limit theo API key/client ID;
* limit theo route;
* global quota;
* trả header chuẩn như `Retry-After`.

Đây là vị trí tốt cho rate limit chung trước khi request vào service.

### Application service

Phù hợp khi cần business context:

* tenant free/premium;
* user role;
* action cost khác nhau;
* quota theo feature;
* limit theo trạng thái account mới/cũ.

Nhược điểm là request đã vào app, nên vẫn tốn một phần tài nguyên.

### Worker/consumer layer

Phù hợp để bảo vệ downstream:

* giới hạn tốc độ gửi email/SMS;
* giới hạn gọi OpenAI/AWS;
* giới hạn resize image;
* giới hạn export/report job.

Ở layer này, leaky bucket, token bucket hoặc concurrency limit thường hữu ích hơn chỉ đếm request HTTP.

---

## 5 thuật toán phổ biến

| Thuật toán | Ý tưởng | Hợp với |
|---|---|---|
| [Fixed Window Counter](fixed-window-counter) | Đếm request trong từng cửa sổ cố định | Simple API quota, free tier đơn giản |
| [Sliding Window Log](sliding-window-log) | Lưu timestamp từng request và đếm trong khoảng trượt | Login/security endpoint cần chính xác |
| [Sliding Window Counter](sliding-window-counter) | Ước lượng cửa sổ trượt bằng current + previous window | API public cần cân bằng giữa chính xác và nhẹ |
| [Token Bucket](token-bucket) | Token refill theo tốc độ, request tiêu thụ token | Cho phép burst có kiểm soát, API gateway |
| [Leaky Bucket](leaky-bucket) | Request vào hàng đợi và được xử lý/chảy ra đều | Làm mượt traffic, bảo vệ worker/downstream |

---

## Chọn thuật toán nhanh

* Muốn đơn giản, dễ implement: **Fixed Window Counter**.
* Muốn chính xác cho login/brute force: **Sliding Window Log**.
* Muốn gần chính xác nhưng tiết kiệm memory: **Sliding Window Counter**.
* Muốn cho burst ngắn nhưng giữ average rate: **Token Bucket**.
* Muốn làm mượt tốc độ xử lý downstream: **Leaky Bucket**.

Trong production, rate limiter phân tán thường đặt state trong Redis và dùng Lua script để atomic. Với single process hoặc service nhỏ, có thể dùng in-memory limiter, nhưng khi scale ngang nhiều instance thì in-memory không còn global chính xác.

---

## Response khi bị limit

HTTP status thường dùng là `429 Too Many Requests`.

Nên trả thêm header:

```http
HTTP/1.1 429 Too Many Requests
Retry-After: 30
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 0
X-RateLimit-Reset: 1735689600
```

Response body nên rõ nhưng không lộ rule quá chi tiết:

```json
{
  "error": "rate_limited",
  "message": "Too many requests. Please retry later.",
  "retry_after_seconds": 30
}
```

Với endpoint security như login, không nên trả thông tin quá cụ thể kiểu "account này bị limit vì sai password 5 lần", vì có thể hỗ trợ attacker dò account.

---

## Issue và edge case thường gặp

### 1. Scale ngang làm in-memory limiter không còn chính xác

Nếu có 5 app instances và mỗi instance cho `100 requests/minute`, user có thể gửi tổng cộng gần `500 requests/minute` nếu load balancer phân tán đều.

Cách xử lý:

* dùng Redis/shared storage;
* dùng API gateway rate limit;
* sticky session chỉ là giảm sai số, không phải giải pháp chắc chắn;
* với distributed limiter, dùng Lua script/transaction để atomic.

### 2. Race condition khi tăng counter

Nếu check limit và increment counter là 2 thao tác rời, nhiều request song song có thể cùng pass.

Sai về mặt logic:

```text
GET counter
if counter < limit:
  INCR counter
```

Production nên dùng thao tác atomic:

* Redis Lua script;
* Redis `INCR` và set TTL cẩn thận;
* database row lock nếu traffic thấp;
* gateway built-in limiter.

### 3. TTL bị set sai gây key sống mãi hoặc reset sai

Với Redis fixed window, nếu `INCR` xong nhưng quên `EXPIRE`, key có thể sống mãi. Nếu request nào cũng reset TTL, user có thể bị limit lâu hơn dự kiến.

Pattern tốt:

* chỉ set TTL khi key mới tạo;
* TTL dài hơn window một chút để tránh lệch thời gian;
* monitor số lượng key rate limit.

### 4. Clock skew giữa nhiều server

Sliding window/token bucket dùng thời gian. Nếu server lệch clock, kết quả limit có thể sai.

Cách giảm rủi ro:

* dùng time từ Redis/server trung tâm nếu cần chính xác;
* đồng bộ NTP;
* tránh logic phụ thuộc vào millisecond quá chặt nếu không cần.

### 5. Boundary burst của fixed window

Fixed window cho phép burst ở ranh giới window. Nếu endpoint nhạy cảm, dùng sliding window hoặc token bucket.

Ví dụ:

```text
100 requests lúc 12:00:59
100 requests lúc 12:01:00
```

Về mặt window thì hợp lệ, nhưng backend nhận 200 request trong 2 giây.

### 6. IP-based limit dễ chặn nhầm user thật

Nhiều user có thể chung IP:

* công ty;
* trường học;
* quán cafe;
* mobile carrier NAT.

Nếu limit theo IP quá chặt, user thật bị ảnh hưởng. Nên kết hợp user ID, session, tenant, endpoint và risk score.

### 7. User né limit bằng nhiều IP/account

Attacker có thể dùng proxy/VPN hoặc tạo nhiều account mới.

Cách giảm:

* limit theo nhiều dimension;
* device/session fingerprint;
* CAPTCHA/challenge khi có tín hiệu bất thường;
* velocity rule cho account mới;
* trust level/quota thấp cho account mới;
* detect pattern hành vi như scan ID tuần tự, request interval đều bất thường.

### 8. Retry storm sau khi bị 429

Client thấy `429` rồi retry ngay lập tức có thể làm hệ thống tệ hơn.

Cách xử lý:

* trả `Retry-After`;
* client dùng exponential backoff và jitter;
* server không nên queue vô hạn;
* SDK/internal client nên tôn trọng rate-limit header.

### 9. Fail-open hay fail-closed khi Redis/gateway lỗi?

Nếu rate limiter phụ thuộc Redis, khi Redis lỗi phải quyết định:

* **Fail-open:** cho request đi qua, ưu tiên availability nhưng có rủi ro abuse/overload.
* **Fail-closed:** chặn request, an toàn hơn nhưng có thể làm downtime.

Gợi ý:

* endpoint public/read nhẹ: có thể fail-open ngắn hạn;
* login/payment/expensive AI API: cân nhắc fail-closed hoặc fallback limit cục bộ;
* luôn alert khi limiter dependency lỗi.

### 10. Rate limit không thay thế authorization

Rate limit chỉ giới hạn tần suất, không quyết định user có quyền hay không. Endpoint vẫn phải kiểm tra auth/authorization, ownership và tenant isolation.

### 11. Limit quá thấp làm hỏng UX

Rate limit cần quan sát traffic thật. Nếu limit đặt cảm tính:

* frontend có thể bị chặn khi load nhiều resource;
* mobile network retry làm user bị limit oan;
* user premium thấy trải nghiệm không khác free;
* background sync/POS offline sync có thể bị nghẽn.

Nên đo p95/p99 request rate theo user/tenant trước, rồi đặt limit có buffer.

### 12. Endpoint khác nhau cần rule khác nhau

Không nên dùng một rule cho mọi endpoint.

Ví dụ:

```text
GET /products          -> limit cao, cache được
POST /login            -> limit thấp, security-sensitive
POST /upload           -> limit theo size/quota
POST /ai/generate      -> limit theo cost/token
POST /reports/export   -> limit theo job/concurrency
```

### 13. Multi-region và consistency

Nếu hệ thống chạy nhiều region, rate limit global chính xác sẽ khó hơn vì state phân tán.

Các hướng xử lý:

* limit local per-region, chấp nhận sai số;
* route một tenant/user về một home region;
* dùng global datastore nhưng chấp nhận latency;
* chia quota theo region, ví dụ mỗi region 50% quota.

### 14. Observability thiếu thì không biết limit đúng hay sai

Cần metric/log:

* allowed vs rejected count;
* reject theo endpoint/user/tenant/API key;
* top limited keys;
* Redis/gateway latency;
* false positive report từ user;
* chi phí third-party trước/sau khi limit.

Alert nên tập trung vào spike bất thường, không alert từng request bị limit.

---

## Checklist thiết kế rate limit

1. Xác định mục tiêu: overload, abuse, fair usage, cost hay SLA.
2. Xác định dimension: IP, user, tenant, API key, endpoint, device.
3. Chọn thuật toán: fixed/sliding/token/leaky.
4. Chọn nơi enforce: edge, gateway, app, worker.
5. Chọn state store: in-memory, Redis, gateway built-in.
6. Thiết kế response `429`, `Retry-After`, message.
7. Quyết định fail-open/fail-closed.
8. Thêm metric, log, alert.
9. Test concurrency và boundary window.
10. Review UX để tránh chặn nhầm user thật.

---

## Câu trả lời phỏng vấn

> Rate limit dùng để bảo vệ backend khỏi overload, chống abuse như brute-force/scraping/DDoS nhẹ, đảm bảo fair usage giữa tenant và kiểm soát chi phí với API tính tiền theo call. Tôi sẽ chọn thuật toán theo use case: token bucket cho API gateway vì hỗ trợ burst, sliding window log cho login vì cần chính xác, sliding window counter cho public API vì cân bằng giữa độ chính xác và memory. Production nhiều instance thường cần Redis/Lua để đảm bảo counter atomic và shared giữa các app instance.
