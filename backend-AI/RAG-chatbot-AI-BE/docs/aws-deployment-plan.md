# AWS Deployment Plan Cho RAG Chatbot AI Backend

## Mục Tiêu

Deploy backend RAG lên AWS để client có thể:

- Upload PDF.
- Theo dõi trạng thái xử lý document.
- Search semantic qua Qdrant.
- Chat hỏi đáp với citations.
- Nhận response thường hoặc SSE streaming.

Project này khác `video-call` ở chỗ:

- Không có WebRTC media.
- Không cần TURN/STUN.
- Không cần WebSocket signaling.
- API chủ yếu là HTTP/SSE.
- Có nhiều dependency stateful hơn: PostgreSQL, Redis, Qdrant.

## Có Cần HTTPS Không?

Nếu chỉ demo nội bộ bằng `curl` hoặc Postman:

```text
Client -> http://<public-ip>:8080
```

thì có thể chưa cần HTTPS.

Nhưng nếu đưa cho người khác dùng qua browser hoặc upload tài liệu thật, nên dùng HTTPS vì:

- Request có thể chứa nội dung tài liệu nội bộ.
- Chat history và citations có thể nhạy cảm.
- Sau này có auth/token thì HTTP sẽ làm lộ token.
- Browser/API client production thường kỳ vọng HTTPS.
- SSE chạy tốt qua HTTPS khi có proxy/load balancer cấu hình đúng.

Kết luận:

- Demo nhanh, ít người dùng: có thể public trực tiếp ECS task bằng HTTP.
- Production hoặc tài liệu thật: nên dùng domain + HTTPS, thường qua ALB hoặc API Gateway/CloudFront.

## Architecture Hiện Tại Theo README

```text
Client
  -> HTTP / SSE
  -> Go API Server :8080

Go API Server
  -> PostgreSQL: metadata, documents, chunks, chat history
  -> Redis: Asynq queue/cache
  -> Qdrant: vector database
  -> OpenAI API: embeddings + LLM

Background Worker
  -> Redis: dequeue jobs
  -> PostgreSQL: update document/chunk status
  -> Qdrant: upsert vectors
  -> OpenAI API: embeddings
```

API và worker nên chạy từ cùng Docker image nhưng bằng command khác:

```text
api command:    ./api
worker command: ./worker
```

## Option A - Demo Nhanh Không Dùng ALB

Đây là hướng bạn đang nghĩ tới: gắn public IP trực tiếp cho ECS task/service.

### Luồng

```text
Client
  -> http://<ecs-task-public-ip>:8080
  -> ECS Fargate task
  -> API container
```

Nếu API và worker chạy chung task với Redis/Qdrant/Postgres sidecar:

```text
ECS Task public subnet
  - api container :8080 public
  - worker container internal
  - postgres container internal
  - redis container internal
  - qdrant container internal
```

### Ưu Điểm

- Rẻ và đơn giản.
- Không cần ALB.
- Không cần domain.
- Hợp lý để demo MVP bằng Postman/curl.

### Nhược Điểm

- Public IP của task có thể đổi khi redeploy/restart.
- Không có HTTPS.
- Không có load balancing.
- Không có health check/service routing tốt như ALB.
- Stateful containers trong ECS task không bền vững nếu dùng local filesystem.
- Không nên mở PostgreSQL/Redis/Qdrant ra internet.

### Security Group

Chỉ mở API port:

```text
Inbound:
  TCP 8080 từ IP của bạn hoặc 0.0.0.0/0 nếu demo public tạm thời

Outbound:
  All traffic, để gọi OpenAI API và AWS services
```

Không mở:

```text
5432 PostgreSQL
6379 Redis
6333/6334 Qdrant
```

Các service nội bộ nên chỉ được API/worker truy cập trong task/VPC.

### ECS Network

Với Fargate:

```text
Subnet: public subnet
assignPublicIp: ENABLED
Security group: mở 8080
```

Client gọi:

```bash
curl http://<task-public-ip>:8080/api/v1/health
```

### Khi Nào Dùng Option A?

Dùng khi:

- Đang học/deploy demo.
- Chưa cần domain.
- Chưa có user thật.
- Chấp nhận IP đổi sau mỗi lần deploy.

Không nên dùng lâu dài nếu có dữ liệu thật.

## Option B - Production Tối Thiểu Với ALB + HTTPS

### Luồng

```text
Client
  -> https://rag.example.com
  -> ALB HTTPS :443
  -> ECS API service :8080
```

Worker không cần public endpoint:

```text
ECS worker service
  -> Redis
  -> PostgreSQL
  -> Qdrant
  -> OpenAI API
```

### Ưu Điểm

- Có domain ổn định.
- Có HTTPS qua ACM.
- ALB health check API.
- Dễ scale nhiều API tasks.
- Client không phụ thuộc public IP task.
- Security tốt hơn: ECS tasks có thể nằm private subnet.

### Nhược Điểm

- Tốn thêm chi phí ALB.
- Cần Route 53/domain/ACM.
- Setup ban đầu nhiều bước hơn.

### Khi Nào Dùng Option B?

Dùng khi:

- Có frontend/browser client.
- Upload tài liệu thật.
- Có auth/token.
- Muốn endpoint ổn định.
- Muốn deploy giống production.

## Option C - Static Frontend Riêng, Backend API Riêng

Nếu sau này có frontend web React riêng cho chatbot:

```text
Browser
  -> CloudFront/S3
  -> tải frontend static

Browser
  -> https://api.rag.example.com
  -> ALB/API Gateway
  -> ECS API service
```

Hướng này hợp lý khi frontend là static SPA và backend là API.

## Khuyến Nghị Cho Project Này

Theo giai đoạn:

1. MVP demo:
   - ECS Fargate public IP trực tiếp.
   - HTTP port `8080`.
   - Restrict security group inbound theo IP của bạn nếu có thể.

2. Demo nghiêm túc hơn:
   - Dùng ALB + domain + HTTPS.
   - API service public qua ALB.
   - Worker service private.

3. Production:
   - ALB + HTTPS.
   - RDS PostgreSQL.
   - ElastiCache Redis hoặc Redis managed.
   - Qdrant Cloud hoặc Qdrant chạy riêng có persistent storage.
   - Secrets trong SSM Parameter Store hoặc Secrets Manager.
   - ECS tasks ở private subnets.

## Hạ Tầng Đề Xuất Cho MVP Không ALB

### ECS Cluster

```text
Cluster: rag-chatbot-cluster
Launch type: Fargate
Region: ap-southeast-1
```

### ECR

Tạo repository:

```text
rag-chatbot-api
```

Nếu API và worker dùng chung image, chỉ cần một ECR repo.

### ECS Task Definition

Một task definition có thể gồm:

```text
api container
worker container
postgres container
redis container
qdrant container
```

Nhưng với ECS Fargate, cách này chỉ nên dùng demo vì PostgreSQL/Qdrant cần storage bền vững.

Biến môi trường API/worker:

```text
PORT=8080
OPENAI_API_KEY=<from SSM/Secrets Manager>
OPENAI_EMBEDDING_MODEL=text-embedding-3-small
OPENAI_LLM_MODEL=gpt-4.1-mini
POSTGRES_DSN=postgres://user:pass@localhost:5432/ragchatbot?sslmode=disable
REDIS_ADDR=localhost:6379
QDRANT_ADDR=localhost:6334
CHUNK_SIZE=500
CHUNK_OVERLAP=100
SEARCH_TOP_K=5
```

Nếu tách DB/Redis/Qdrant ra managed services, thay `localhost` bằng endpoint nội bộ/VPC endpoint tương ứng.

### ECS Service

Cho demo không ALB:

```text
Service: rag-chatbot-service
Desired count: 1
Subnet: public subnet
Assign public IP: ENABLED
Load balancer: none
Security group inbound: TCP 8080
```

Sau khi service chạy, lấy public IP của task:

```bash
aws ecs list-tasks \
  --cluster rag-chatbot-cluster \
  --service-name rag-chatbot-service
```

Sau đó describe task/network interface để lấy public IP.

Test:

```bash
curl http://<task-public-ip>:8080/api/v1/health
```

## Hạ Tầng Đề Xuất Cho Production

### API Service

```text
ECS service: rag-chatbot-api-service
Container: api :8080
Subnet: private subnets
Public IP: DISABLED
Inbound: từ ALB security group
```

### Worker Service

```text
ECS service: rag-chatbot-worker-service
Container: worker
Subnet: private subnets
Public IP: DISABLED
No inbound public traffic
```

Worker chỉ cần outbound tới:

- Redis.
- PostgreSQL.
- Qdrant.
- OpenAI API.
- CloudWatch Logs.

### PostgreSQL

Production nên dùng:

```text
Amazon RDS PostgreSQL
```

Không nên dùng PostgreSQL container trong ECS cho dữ liệu thật.

### Redis

Production nên dùng:

```text
Amazon ElastiCache Redis
```

Hoặc Redis managed provider khác.

### Qdrant

Các lựa chọn:

- Qdrant Cloud: đơn giản nhất.
- EC2/ECS riêng + EBS/EFS persistent storage: tự vận hành nhiều hơn.
- Chạy sidecar Qdrant trong cùng ECS task: chỉ phù hợp demo.

### Secrets

Lưu vào SSM Parameter Store hoặc Secrets Manager:

```text
/rag-chatbot/OPENAI_API_KEY
/rag-chatbot/POSTGRES_DSN
/rag-chatbot/REDIS_ADDR
/rag-chatbot/QDRANT_ADDR
```

Không hardcode OpenAI key trong task definition hoặc source code.

## Checklist Deploy MVP Không ALB

1. Tạo Dockerfile build Go API và worker.
2. Build image local.
3. Push image lên ECR.
4. Tạo ECS cluster.
5. Tạo task definition gồm API, worker và các dependency demo nếu cần.
6. Tạo security group mở inbound `8080`.
7. Tạo ECS service trong public subnet với `assignPublicIp=ENABLED`.
8. Lấy public IP của running task.
9. Test health endpoint.
10. Chạy migration PostgreSQL.
11. Upload PDF test.
12. Kiểm tra document status.
13. Test search/chat/SSE.

## Checklist Deploy Production

1. Tạo ECR repo.
2. Build/push image.
3. Tạo RDS PostgreSQL.
4. Tạo ElastiCache Redis.
5. Tạo Qdrant Cloud hoặc Qdrant persistent deployment.
6. Tạo SSM/Secrets Manager parameters.
7. Tạo ECS API service private.
8. Tạo ECS worker service private.
9. Tạo ALB public HTTPS.
10. Gắn ACM certificate.
11. Route domain qua Route 53.
12. ALB health check tới `/api/v1/health`.
13. Chạy migration.
14. Test upload/search/chat/SSE.
15. Bật CloudWatch logs và alarms.

## Lưu Ý Về SSE

Endpoint chat streaming dùng SSE:

```text
POST /api/v1/chat
stream=true
```

Nếu đi qua ALB/nginx/proxy, cần tránh buffering response quá lâu.

Với direct public IP không ALB, SSE sẽ đơn giản hơn:

```text
Client -> API container
```

Nhưng khi chuyển qua ALB, cần test:

```bash
curl -N -X POST https://rag.example.com/api/v1/chat \
  -H "Content-Type: application/json" \
  -d '{"question":"...","stream":true}'
```

## Kết Luận

Bạn nghĩ đúng một phần:

- Nếu chỉ demo backend API, chưa cần HTTPS, có thể public trực tiếp ECS task bằng public IP và port `8080`.
- Không cần ALB cho demo nhanh.

Nhưng cần nhớ:

- Direct public IP không ổn định sau redeploy.
- HTTP không an toàn cho tài liệu thật/token/chat history.
- Stateful dependencies không nên chạy lâu dài bằng container filesystem trong ECS.
- Khi project nghiêm túc hơn, nên chuyển sang ALB + HTTPS + managed storage.
