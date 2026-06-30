# Production Scale Metrics
## Cách nói về users, CCU, RPS/TPS, latency, data volume và throughput theo CV

File này dùng để chuẩn bị câu trả lời phỏng vấn khi nhà tuyển dụng hỏi:

> Hệ thống bạn từng làm phục vụ bao nhiêu user? Concurrent users bao nhiêu? RPS/TPS? Latency? Data volume? Bạn trực tiếp chạm vào phần nào?

Nguyên tắc quan trọng: không bịa số. Nếu không sở hữu metric, nói rõ phạm vi mình chạm vào và cách mình sẽ đo/ước lượng.

---

## 1. Phân loại mức độ "đã chạm"

Khi nói production scale, nên phân biệt 3 mức:

| Mức | Ý nghĩa | Cách nói |
|---|---|---|
| Trực tiếp chạm | Bạn tự debug/optimize/monitor phần đó | "Tôi trực tiếp xử lý query/API/worker này..." |
| Chạm gián tiếp | Hệ thống có metric đó, bạn làm feature nằm trong hệ thống nhưng không owner metric | "Hệ thống phục vụ khoảng..., phần tôi làm nằm trong flow..." |
| Chưa chạm rõ | Bạn hiểu concept nhưng chưa có số thật | "Tôi chưa trực tiếp sở hữu metric này, nhưng tôi hiểu cần đo bằng..." |

Câu trả lời tốt không phải lúc nào cũng có số thật. Điểm quan trọng là thành thật, biết phạm vi trách nhiệm, và biết metric nào cần đo.

---

## 2. Các metric nghĩa là gì?

### Registered users / total users

Tổng số tài khoản/người dùng đã đăng ký hoặc từng dùng hệ thống.

Ví dụ từ CV:

* U2U Network: **50,000+ users**.

Nên nói:

> Hệ thống proxy platform ở U2U phục vụ hơn 50,000 users. Tôi không phải người duy nhất owner toàn bộ platform, nhưng tôi trực tiếp phát triển backend service/API và một số phần concurrency quanh core proxy service.

### CCU / concurrent users

Số user hoạt động cùng thời điểm. Đây không phải tổng user.

Ví dụ từ CV:

* U2U Network: khoảng **2,000 concurrent users**.

Nên nói:

> Điểm scale rõ nhất trong CV của tôi là U2U, khoảng 50,000+ users và 2,000 concurrent users. Tôi có chạm vào backend service, API, Redis/PostgreSQL/RabbitMQ integration và phần async/concurrent processing.

### RPS

Requests per second: số HTTP/gRPC requests mỗi giây.

Bạn có thể đã chạm nếu:

* debug API chậm.
* xem Grafana/ALB/Nginx/API Gateway metrics.
* tối ưu endpoint/query.
* cấu hình rate limit hoặc autoscaling.

Nếu chưa có số:

> Tôi chưa muốn bịa RPS vì ở team trước tôi không phải người owner toàn bộ traffic dashboard. Nhưng với endpoint tôi xử lý, tôi quan tâm p95 latency, DB query latency, error rate và số request theo endpoint để biết bottleneck nằm ở app hay database.

### TPS

Transactions per second. Có hai nghĩa tùy ngữ cảnh:

* Business TPS: số order/payment/sync transaction mỗi giây.
* Database TPS: số database transactions mỗi giây.

Trong phỏng vấn backend, cần hỏi lại:

> Anh/chị đang hỏi TPS theo business transaction như order/payment, hay database transaction throughput?

Bạn có thể chạm TPS nếu làm:

* order/POS sync.
* payment.
* inventory update.
* blockchain-related operation.
* DB transaction/concurrency.

### Latency

Thời gian xử lý request/job.

Các mốc hay dùng:

* average latency: dễ bị che bởi outlier.
* p95 latency: 95% request nhanh hơn giá trị này.
* p99 latency: 99% request nhanh hơn giá trị này, quan trọng với tail latency.

Nên nói:

> Khi debug latency, tôi không chỉ nhìn average. Tôi ưu tiên p95/p99, DB query latency, Redis latency, downstream timeout và connection pool wait.

### Data volume

Quy mô dữ liệu hệ thống xử lý/lưu trữ:

* số rows/documents.
* GB/TB data.
* số events/messages.
* số files/PDFs.
* số chunks/vectors trong RAG.

Bạn có thể chạm nếu:

* thiết kế schema/index.
* debug slow query.
* viết ETL.
* migrate data.
* xử lý MongoDB aggregation.
* xây ingestion/vector indexing.

### Throughput

Số lượng công việc hệ thống xử lý trong một khoảng thời gian.

Ví dụ:

* requests/second.
* messages/second.
* jobs/minute.
* documents/hour.
* chunks embedded/minute.
* rows processed/minute trong ETL.

Throughput khác latency:

* Latency: một request mất bao lâu.
* Throughput: hệ thống xử lý được bao nhiêu request/job trong một đơn vị thời gian.

---

## 3. Map theo CV của bạn

### U2U Network - proxy networking platform

CV facts có thể nói:

* 50,000+ users.
* khoảng 2,000 concurrent users.
* backend systems bằng Rust, microservices quanh core proxy server.
* async/concurrent processing.
* PostgreSQL, Redis, RabbitMQ.
* AWS EC2.

Bạn có thể nói chắc:

> Đây là hệ thống có số production rõ nhất của tôi: hơn 50,000 users và khoảng 2,000 concurrent users. Tôi trực tiếp làm backend service/API, tích hợp PostgreSQL, Redis, RabbitMQ và có tham gia phần async/concurrent processing quanh proxy platform.

Nên cẩn thận:

* Không nói bạn một mình scale toàn bộ platform.
* Không bịa RPS/TPS nếu không có dashboard/log.
* Nếu hỏi latency, nói theo cách debug/metric bạn dùng nếu không nhớ số.

Câu trả lời mẫu:

> Ở U2U, hệ thống phục vụ hơn 50,000 users và khoảng 2,000 concurrent users. Phần tôi trực tiếp chạm là backend service/API, Redis/PostgreSQL/RabbitMQ integration và async/concurrent processing. Tôi không nhớ chính xác RPS toàn hệ thống nên sẽ không bịa, nhưng khi debug performance tôi nhìn vào request latency, DB query latency, Redis/cache behavior, queue backlog và resource usage của service.

### DIQIT - F&B e-commerce/POS/offline-first

CV facts có thể nói:

* F&B e-commerce và POS cho Japanese clients.
* online ordering, store operations, reporting, offline-first systems.
* Golang + Node.js TypeScript.
* MongoDB, gRPC, RabbitMQ, AWS/Kubernetes/Lambda/ECS/S3.
* Grafana.
* Dart sync engine cho offline POS.

Bạn có thể đã chạm:

* DB query/API performance.
* sync throughput.
* RabbitMQ async job flow.
* MongoDB schema/index/aggregation.
* Grafana metrics.
* offline sync conflict/retry/idempotency.

Cách nói khi chưa có số:

> Với DIQIT, tôi chạm nhiều vào flow POS/e-commerce thực tế như API, MongoDB query, gRPC/RabbitMQ và offline sync. Tôi có làm việc trong môi trường có Grafana, nhưng nếu không có số RPS/TPS chính xác trong đầu, tôi sẽ nói rõ phần tôi trực tiếp optimize là query/API/sync flow thay vì claim traffic toàn hệ thống.

Metric nên tự điền nếu có:

```text
Số store/client:
Số POS devices:
Orders/day:
Peak orders/minute:
Sync jobs/minute:
API p95 latency:
MongoDB collection size:
RabbitMQ backlog peak:
```

### The Growhub - AgriTech supply chain/traceability

CV facts có thể nói:

* AgriTech supply chain và traceability.
* Golang microservices.
* migrate Python sang Golang.
* MySQL, Elasticsearch, Temporal.
* ETL, DuckDB, Parquet.
* analytics dashboards.

Bạn có thể đã chạm:

* ETL throughput.
* data volume theo files/rows/Parquet.
* Elasticsearch query/indexing.
* backend service migration performance.
* business analytics data pipeline.

Cách nói:

> Ở Growhub, phần scale tôi chạm nhiều hơn là data pipeline/analytics và backend migration. Tôi có làm ETL với DuckDB/Parquet và service Golang, nên nếu nói metric tôi sẽ tách rõ API traffic với data throughput như số rows/files xử lý mỗi batch.

Metric nên tự điền nếu có:

```text
Rows processed/job:
Files processed/day:
Parquet data size:
ETL duration before/after:
Elasticsearch index size:
API latency before/after migration:
```

### IDS Software - e-commerce/Zalo Mini App

CV facts có thể nói:

* Backend intern.
* Java Spring Boot.
* MySQL.
* payment integrations Momo/ZaloPay.

Nên nói ở mức vừa phải:

> Đây là internship nên tôi không dùng nó làm ví dụ scale chính. Tôi có chạm API, payment integration và MySQL, nhưng production-scale story chính của tôi nên là U2U/DIQIT/Growhub.

### Redis clone

Không phải production users thật, nhưng là project deep dive kỹ thuật.

Có thể nói về:

* concurrency.
* RESP parser.
* command dispatch.
* TTL.
* AOF/PubSub concept.
* race test/benchmark nếu có.

Không nên dùng để claim production scale.

### RAG chatbot/backend-AI

Không phải production users thật, nhưng có thể nói về system design và throughput giả định.

Có thể nói về:

* document ingestion throughput.
* chunks/document.
* embeddings latency/cost.
* Qdrant search latency.
* chat latency/token usage.
* worker queue backlog.

Metric nên tự đo nếu muốn:

```text
PDF pages/document:
Chunks/document:
Embedding latency/chunk batch:
Search topK latency:
Chat p95 latency:
Tokens/request:
Worker jobs/minute:
```

---

## 4. Những phần bạn nói là đã chạm trực tiếp

Dựa trên CV và cách bạn mô tả, có thể tự tin nói đã chạm:

* DB query/API backend.
* PostgreSQL/MySQL/MongoDB integration.
* Redis caching hoặc Redis-backed flow.
* RabbitMQ async messaging.
* gRPC service communication.
* concurrent/async processing.
* ETL/data pipeline.
* production system có real users.
* U2U scale: 50,000+ users, khoảng 2,000 concurrent users.

Cách nói:

> Tôi đã trực tiếp chạm vào API/backend service, DB query, Redis/RabbitMQ integration và concurrent processing. Về số production rõ nhất, U2U có hơn 50,000 users và khoảng 2,000 concurrent users. Các metric như RPS/TPS toàn hệ thống thì tôi không claim nếu không có dashboard trước mặt, nhưng tôi biết cách đo và tôi thường debug qua latency, query time, queue backlog, error rate và resource usage.

---

## 5. Những phần có thể đã chạm gián tiếp

Các phần này có thể bạn đã gặp qua nhưng nên nói thận trọng:

| Concept | Có thể đã chạm qua | Cách nói an toàn |
|---|---|---|
| RPS | API services, Grafana, gateway/load balancer metrics | "Tôi từng xem/quan tâm request rate theo endpoint, nhưng không owner traffic toàn platform." |
| TPS | order/POS sync, payment, DB transaction, ETL transaction | "Tôi cần phân biệt business TPS và DB TPS; phần tôi chạm là transaction/query/sync flow." |
| p95/p99 latency | API latency, query latency, Grafana | "Tôi ưu tiên p95/p99 hơn average khi debug." |
| Data volume | ETL, Mongo collections, RAG chunks/vectors | "Tôi đo bằng rows/files/documents/GB tùy hệ thống." |
| Transaction throughput | DB updates, inventory/payment/sync flow | "Tôi quan tâm lock, isolation, connection pool và retry." |
| Observability | Grafana/logging/debugging | "Tôi có dùng metrics/logs để debug, nhưng chưa claim mình setup toàn bộ observability platform nếu không trực tiếp làm." |

---

## 6. Những phần chưa nên claim quá mạnh

Nếu chưa có bằng chứng rõ, tránh nói:

* "Tôi scale hệ thống lên X RPS" nếu bạn không trực tiếp owner.
* "Tôi đảm bảo p99 dưới X ms" nếu không có số.
* "Tôi thiết kế HA toàn hệ thống" nếu bạn chỉ deploy/maintain một service.
* "Tôi tối ưu database throughput X lần" nếu không có before/after.

Thay vào đó:

> Tôi không trực tiếp owner toàn bộ metric đó, nhưng phần tôi làm nằm trong hệ thống production có real users. Tôi có kinh nghiệm debug query/API/queue và hiểu cần đo RPS, p95/p99 latency, error rate, queue backlog, DB connection pool và resource usage để đánh giá scale.

---

## 7. Câu trả lời mẫu khi bị hỏi về số

### Câu hỏi: Hệ thống lớn nhất bạn từng làm phục vụ bao nhiêu user?

Trả lời:

> Hệ thống có số rõ nhất là U2U proxy platform, phục vụ hơn 50,000 users và khoảng 2,000 concurrent users. Tôi tham gia backend service/API, async/concurrent processing và tích hợp PostgreSQL, Redis, RabbitMQ. Tôi không claim một mình scale toàn bộ platform, nhưng tôi trực tiếp làm trên các service chạy trong hệ thống production đó.

### Câu hỏi: RPS/TPS hệ thống là bao nhiêu?

Trả lời nếu chưa nhớ số:

> Tôi không muốn bịa số RPS/TPS nếu không có dashboard. Ở các team trước, tôi thường chạm vào endpoint/query/worker cụ thể hơn là owner traffic toàn platform. Khi đánh giá một flow, tôi nhìn request rate, p95/p99 latency, error rate, DB query time, connection pool wait và queue backlog. Với business TPS, tôi sẽ tách order/payment/sync transaction khỏi database TPS.

### Câu hỏi: Bạn từng optimize DB query chưa?

Trả lời:

> Có. Phần tôi chạm trực tiếp là DB query/API backend. Khi query chậm, tôi sẽ xem query pattern, index hiện có, `EXPLAIN/EXPLAIN ANALYZE`, số rows scan, sort/join, connection pool và transaction duration. Với bảng lớn, tôi cân nhắc composite index, cursor pagination, tránh N+1, và migration/index creation an toàn.

### Câu hỏi: Bạn biết latency của service không?

Trả lời:

> Nếu có dashboard thì tôi sẽ nói theo p95/p99 thay vì average. Nếu không có số cụ thể trong đầu, tôi sẽ nói cách tôi đo: latency theo endpoint, DB/Redis/downstream latency, timeout rate, queue wait time và resource saturation. Với RAG project, latency còn gồm embedding latency, Qdrant search latency và chat model latency.

### Câu hỏi: Bạn từng xử lý concurrency ở production chưa?

Trả lời:

> Có. Ở U2U tôi làm trong hệ thống có khoảng 2,000 concurrent users và có phần async/concurrent processing quanh backend/proxy platform. Ở Go/RabbitMQ/worker flow, tôi quan tâm giới hạn worker, context cancellation, timeout, idempotency và queue backlog để tránh overload downstream.

---

## 8. Checklist tự điền số thật

Trước khi phỏng vấn, nếu có thể mở lại Grafana/log/DB stats, điền các số này:

### U2U

```text
Total users: 50,000+
Peak concurrent users: ~2,000
Peak RPS:
Average/p95/p99 API latency:
PostgreSQL data size / biggest table:
Redis usage:
RabbitMQ message rate / backlog:
Incident/performance story:
```

### DIQIT

```text
Stores/tenants:
POS devices:
Orders/day:
Peak order rate:
Sync jobs/minute:
MongoDB biggest collection:
API p95 latency:
RabbitMQ backlog:
Grafana dashboard you used:
```

### Growhub

```text
Rows processed per ETL job:
Data size per batch:
ETL duration before/after:
Elasticsearch index size:
API latency before/after Python -> Go migration:
Temporal workflows/day:
```

### RAG project

```text
Documents:
Average pages/document:
Chunks/document:
Embedding batch latency:
Qdrant search latency:
Chat latency:
Token usage/request:
Worker jobs/minute:
```

---

## 9. Map sang notes khác

* Go concurrency: [golang-core-interview.md](golang-core-interview.md)
* Database optimization: [database-middle-roadmap.md](database-middle-roadmap.md)
* RabbitMQ/gRPC/retry/circuit breaker: [backend-communication-roadmap.md](backend-communication-roadmap.md)
* Production failure modes: [production-backend-concepts.md](production-backend-concepts.md)
* System design scale path: [scale_system_question.md](scale_system_question.md)

