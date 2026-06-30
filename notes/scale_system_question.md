## câu 1: trả lời câu hỏi, hệ thống đang phục vụ CCU 100, bỗng 1 ngày lên 100m
## câu 2: giải thích từ đầu network, API gateway..... cho đến layer cuối cùng như database


Hãy tưởng tượng hệ thống của bạn đang chạy bình thường với **100 CCU** (concurrent users), rồi một ngày đẹp trời lên **100 triệu CCU**.

Đây không còn là “scale server” nữa, mà là:

* distributed systems
* networking
* load distribution
* async architecture
* database sharding
* queueing
* consistency tradeoff
* regional infra
* observability
* failure isolation

Tôi sẽ đi từ ngoài Internet → tận database cuối cùng.

---

# 1. User Request Đi Từ Đâu?

Flow thực tế:

```text
User Device
   ↓
DNS
   ↓
CDN / Edge
   ↓
WAF / DDoS Protection
   ↓
Load Balancer
   ↓
API Gateway
   ↓
Service Mesh / Internal LB
   ↓
Microservices
   ↓
Cache / Queue / DB
```

---

# 2. DNS Layer

100 user:

* bình thường dùng Cloudflare/AWS Route53 là đủ

100M CCU:

* DNS itself becomes critical infra

Vấn đề:

* DNS query volume cực lớn
* regional routing
* failover
* latency routing

Lúc này dùng:

* Geo DNS
* Anycast DNS
* multi-region resolution

Ví dụ:

* user VN → Singapore region
* user US → Oregon
* user EU → Frankfurt

Mục tiêu:

* không cho traffic toàn thế giới đập vào 1 datacenter

---

# 3. CDN / Edge Layer

Nếu frontend/static assets:

* JS bundle
* images
* videos
* configs

mà vẫn từ origin server:

=> chết ngay.

100M CCU:

* mọi static content phải push ra edge.

Dùng:

* Cloudflare
* Akamai
* Amazon Web Services CloudFront

CDN giúp:

* cache gần user
* giảm bandwidth origin
* absorb traffic spikes

Ví dụ:

* 1 image được cache tại Tokyo edge node
* 10 triệu request không cần về origin

---

# 4. DDoS Protection / WAF

100M CCU thật sự rất khó phân biệt với DDoS.

Lúc này cần:

* bot detection
* rate limiting
* fingerprinting
* challenge system
* IP reputation

Nếu không:

* crawler thôi cũng giết chết hệ thống.

---

# 5. Load Balancer Layer

Tại sao không cho user hit app server luôn?

Vì:

* app server có thể chết
* cần distribute traffic
* health check
* sticky session
* SSL termination

Flow:

```text
User
  ↓
Global LB
  ↓
Regional LB
  ↓
Internal LB
```

100M CCU:

* load balancer cũng phải distributed

Thường:

* L4 LB (TCP)
* L7 LB (HTTP)

Ví dụ:

* NGINX
* Envoy
* HAProxy
* AWS ALB/NLB

---

# 6. API Gateway

Đây là “cổng thành”.

Nó xử lý:

* auth
* rate limit
* routing
* JWT validation
* API aggregation
* metrics
* tracing

Không thể:

```text
User → microservice trực tiếp
```

vì:

* security nightmare
* impossible observability

Gateway thường:

* stateless
* horizontal scale

100M CCU:

* gateway layer có thể lên hàng nghìn instances.

---

# 7. Stateless Application Layer

Đây là mindset lớn nhất.

100 user:

```text
server lưu session trong RAM
```

100M:
=> impossible.

Mọi app server phải:

* stateless
* ephemeral
* disposable

Session:

* Redis
* JWT
* distributed cache

Lúc này:

```text
any request can hit any server
```

Mới scale được.

---

# 8. Auto Scaling

100 user:

* 2 servers đủ

100M:
traffic thay đổi từng giây.

Phải có:

* HPA (Horizontal Pod Autoscaler)
* queue depth scaling
* CPU scaling
* predictive scaling

Ví dụ:

* concert ticket mở bán
* spike từ 100k → 20M trong 2 phút

Nếu scale chậm:
=> cascade failure.

---

# 9. Cache Layer — Thứ Sống Còn

Không có cache:

* database chết đầu tiên.

100M CCU:
cache hit rate là thứ quyết định sống/chết.

Thường dùng:

* Redis
* Memcached

Cache:

* session
* profile
* hot feed
* product info
* ranking
* permissions

Ví dụ:

DB:

```sql
SELECT * FROM users WHERE id=1
```

Nếu 20M req/s:
=> impossible.

Cache:

```text
Redis GET user:1
```

RAM access nhanh hơn DB hàng trăm lần.

---

# 10. Nhưng Redis Có Bottleneck Không?

Có.

Rất nhiều người nghĩ:

> “dùng Redis là xong”

Sai.

100M CCU:
Redis cũng:

* overload
* network saturation
* memory fragmentation
* hot key problem

Ví dụ hot key:

```text
GET celebrity:taylorswift
```

10 triệu req/s vào 1 key.

Redis single-thread:
=> chết.

Giải pháp:

* Redis Cluster
* key partition
* replication
* local cache
* edge cache

---

# 11. Queue Layer — Cứu Hệ Thống Khỏi Spike

Sai lầm lớn:

```text
request → xử lý sync toàn bộ
```

100M:
=> app chết vì burst.

Phải chuyển sang async.

Ví dụ:

* send email
* analytics
* recommendation
* notification

Thay vì:

```text
API → email service
```

Làm:

```text
API → Kafka → worker xử lý
```

Dùng:

* Apache Kafka
* RabbitMQ
* Pulsar

Queue giúp:

* absorb spike
* retry
* backpressure
* decouple services

---

# 12. Database Layer — Nơi Mọi Người Chết

100 user:

```text
1 MySQL server
```

100M:
=> impossible.

---

# 13. Vertical Scaling Không Cứu Được

## Scale dọc vs scale ngang trong backend

**Scale dọc (vertical scaling)** là tăng tài nguyên cho một instance/máy hiện tại:

```text
2 CPU / 4GB RAM
-> 8 CPU / 32GB RAM
-> 32 CPU / 128GB RAM
```

Ưu điểm:

* đơn giản.
* không cần đổi kiến trúc nhiều.
* phù hợp giai đoạn đầu hoặc bottleneck tạm thời.

Nhược điểm:

* có giới hạn vật lý.
* máy lớn rất đắt.
* vẫn có single point of failure nếu chỉ có một instance.
* không giải quyết tốt traffic spike quá lớn.

**Scale ngang (horizontal scaling)** là tăng số lượng instances:

```text
1 app instance
-> 3 app instances
-> 20 app instances sau load balancer
```

Ưu điểm:

* tăng throughput bằng cách thêm instance.
* tăng high availability nếu chạy nhiều AZ/node.
* instance chết vẫn còn instance khác xử lý.
* phù hợp stateless backend/API.

Nhược điểm:

* app nên stateless.
* session phải đưa ra Redis/JWT/database.
* cần load balancer/service discovery.
* cần xử lý race condition ở DB/cache/queue.
* background jobs phải tránh chạy trùng.

Trong backend API, scale ngang thường là hướng chính:

```text
Client
-> Load Balancer / API Gateway
-> App instance 1
-> App instance 2
-> App instance 3
```

Muốn scale ngang tốt, backend cần:

* stateless application layer.
* shared database/cache/queue.
* idempotency cho retry.
* distributed lock hoặc leader election cho job đặc biệt.
* health check/readiness check.
* autoscaling theo CPU/RPS/queue backlog/latency.
* observability để biết scale có thật sự giúp không.

Scale ngang không tự động giải quyết database bottleneck. Nếu mọi app instance cùng đập vào một DB, bottleneck chuyển xuống database. Khi đó cần index, cache, read replica, partitioning, sharding hoặc async queue tùy bài toán.

Không thể:

```text
CPU x1000
RAM x1000
```

Một máy có giới hạn vật lý.

Eventually:

* IO bottleneck
* network bottleneck
* replication lag

---

# 14. Read Replica

Bước đầu tiên:

```text
Primary DB
   ↓
Read Replicas
```

Read:

* replicas

Write:

* primary

Ví dụ:

* 95% traffic là read

---

# 15. Nhưng Eventually Replica Cũng Chết

Vì:

* write throughput quá lớn
* replication lag
* storage IO

=> phải shard.

---

# 16. Database Sharding

Chia data.

Ví dụ:

```text
User 1-10M → shard A
User 10M-20M → shard B
```

hoặc:

```text
hash(user_id)
```

Mỗi shard:

* DB riêng
* storage riêng
* replica riêng

---

# 17. Vấn Đề Kinh Hoàng Của Sharding

Sharding không miễn phí.

Nó phá:

* JOIN
* transaction
* consistency

Ví dụ:

```sql
JOIN orders WITH users
```

Nếu khác shard:
=> cực khó.

Lúc này:

* denormalization
* event-driven sync
* CQRS
* materialized view

xuất hiện.

---

# 18. SQL vs NoSQL

100M CCU thường:

* polyglot persistence

Ví dụ:

* MySQL → transaction
* MongoDB → flexible docs
* Cassandra → massive write
* Elasticsearch → search
* Redis → cache

Không ai dùng 1 DB cho tất cả.

---

# 19. Hot Partition Problem

Ví dụ:

```text
chatroom:worldcup
```

100 triệu người cùng ghi.

Nếu shard bằng room_id:
=> 1 shard chết.

Đây là bài toán cực khó.

Giải pháp:

* partition by time
* partition by user
* append-only log
* stream processing

---

# 20. Distributed Lock

100M:
race condition everywhere.

Ví dụ:

* chỉ còn 1 vé concert
* 10M người click mua

Không thể:

```text
SELECT stock
UPDATE stock
```

naive.

Cần:

* optimistic lock
* distributed lock
* atomic operation

---

# 21. Network Là Bottleneck Thật Sự

Nhiều backend dev nghĩ CPU là bottleneck.

Sai.

100M CCU:
network thường chết trước.

Ví dụ:

* east-west traffic
* service-to-service calls

Nếu microservice gọi nhau lung tung:
=> latency explosion.

---

# 22. Service Mesh

Khi microservice quá nhiều:

* auth
* retry
* tracing
* mTLS
* discovery

không thể code thủ công.

Dùng:

* Istio
* Envoy

---

# 23. Observability

100 user:

```text
console.log
```

100M:
=> impossible.

Cần:

* centralized logging
* tracing
* metrics
* alerting

Stack:

* Prometheus
* Grafana
* ELK
* OpenTelemetry

---

# 24. Cascade Failure — Thứ Giết Hệ Thống

Một service chậm:
→ thread pool full
→ retry storm
→ queue full
→ DB chết
→ toàn hệ thống chết

Đây là thứ xảy ra thật.

Giải pháp:

* timeout
* circuit breaker
* bulkhead isolation
* backpressure
* load shedding

---

# 25. Cuối Cùng: Big Tech Không “Scale Một Hệ Thống”

Họ scale bằng:

* partitioning users
* partitioning geography
* partitioning workloads

Ví dụ:

* TikTok feed infra riêng
* chat infra riêng
* recommendation infra riêng

Không có:

```text
1 mega backend
```

---

# 26. Điều Quan Trọng Nhất

Từ 100 → 100M CCU không phải:

```text
viết code tốt hơn
```

Mà là:

```text
giảm shared state
giảm synchronization
giảm coupling
giảm blocking
```

Toàn bộ distributed systems đều xoay quanh:

> “làm sao để càng ít thứ phải đồng bộ realtime càng tốt”
