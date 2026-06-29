# AWS Deployment Plan Cho RAG Chatbot AI Backend

Tài liệu này viết plan deploy chi tiết hơn từ requirement hiện tại của project `RAG-chatbot-AI-BE`.

Mục tiêu là deploy bản **demo dễ hiểu, dễ debug, ít service phức tạp**, phù hợp để backend developer tự triển khai và nắm được ý nghĩa từng bước. Đây chưa phải kiến trúc production.

## 1. Mục Tiêu

Deploy backend RAG và frontend React lên AWS sao cho:

- Backend Go API chạy ổn định trên server demo.
- Worker xử lý parse/chunk/embed chạy cùng hệ thống.
- PostgreSQL metadata dùng dịch vụ managed của AWS là RDS.
- Redis và Qdrant chạy bằng Docker trên EC2 để giảm độ phức tạp.
- File upload lưu local trên EC2 trước, sau đó sync lên S3.
- Browser/public client gọi được app qua HTTP hoặc HTTPS.
- Không public database, Redis, Qdrant ra internet.

## 2. Quyết Định Chính

Với demo hiện tại, chọn hướng:

- FE và BE deploy chung trên 1 EC2 instance.
- Dùng Docker Compose trên EC2 để chạy frontend, backend API, backend worker, Redis, Qdrant.
- PostgreSQL dùng Amazon RDS thay vì chạy trong Docker.
- File upload lưu vào thư mục bind mount trên EC2, ví dụ `/data/ragchat/uploads`.
- Dùng `aws s3 sync` chạy định kỳ để đẩy upload file lên S3.
- Dùng Nginx hoặc Caddy làm reverse proxy trước frontend/backend.
- HTTPS là optional cho MVP demo, nhưng nên làm nếu có domain.
- Không dùng ECS, ALB, API Gateway, CloudFront trong giai đoạn này.

Lý do chọn hướng này:

- Ít thành phần AWS hơn nên dễ học và dễ debug.
- Chi phí thấp hơn so với dùng nhiều managed service.
- Phù hợp demo nội bộ hoặc bản chạy thử cho vài user.
- Vẫn tách được phần state quan trọng nhất là PostgreSQL sang RDS.

## 3. Kiến Trúc Demo

```text
Browser
  -> Domain hoặc EC2 public IP
  -> Nginx/Caddy reverse proxy trên EC2
  -> React frontend
  -> Go API backend :8099
  -> Worker process
  -> Redis container
  -> Qdrant container
  -> RDS PostgreSQL
  -> Upload folder trên EC2
  -> S3 bucket sync định kỳ
```

Luồng upload document:

```text
Browser
  -> API upload PDF
  -> Backend lưu file vào /data/ragchat/uploads
  -> Backend ghi metadata vào RDS
  -> Backend enqueue job vào Redis
  -> Worker parse/chunk/embed
  -> Worker lưu vector vào Qdrant
  -> aws s3 sync đẩy file local lên S3
```

Luồng chat:

```text
Browser
  -> API chat
  -> Backend gọi embedding model
  -> Backend search Qdrant
  -> Backend lấy metadata/citation từ RDS
  -> Backend gọi chat model
  -> Trả answer + citations
```

## 4. Resource Naming Và Namespace

**Step này để làm gì:** tránh tạo nhiều resource AWS bị lẫn giữa các project hoặc môi trường.

Đề xuất namespace:

```text
Project = ragchat
Env     = demo
Region  = ap-southeast-1
```

Tên resource gợi ý:

```text
EC2 instance:       ragchat-demo-ec2
RDS database:       ragchat-demo-postgres
S3 bucket:          ragchat-demo-uploads-<account-id>
IAM role:           ragchat-demo-ec2-role
Security group EC2: ragchat-demo-ec2-sg
Security group RDS: ragchat-demo-rds-sg
Key pair:           ragchat-demo-key
Upload folder:      /data/ragchat/uploads
Docker project:     ragchat-demo
```

Tags nên thêm cho các resource chính:

```text
Project     = ragchat
Environment = demo
Owner       = ninh
ManagedBy   = manual
```

Lưu ý:

- S3 bucket name phải unique toàn cầu, nên thêm account id hoặc suffix riêng.
- RDS identifier chỉ cần unique trong region/account.
- Nên dùng cùng prefix `ragchat-demo` để filter AWS Console dễ hơn.

## 5. Step 1 - Chọn Region

**Step này để làm gì:** đảm bảo EC2, RDS, S3, IAM policy, command AWS CLI đều trỏ cùng một khu vực.

Đề xuất dùng Singapore:

```text
AWS_REGION=ap-southeast-1
```

Lưu ý:

- EC2 và RDS nên cùng region để giảm latency và tránh network phức tạp.
- S3 bucket có thể ở region khác, nhưng nên cùng region cho dễ quản lý.
- Nếu sau này dùng domain HTTPS với AWS Certificate Manager cho ALB/CloudFront thì certificate có rule region riêng. Nhưng plan hiện tại dùng Nginx/Caddy trên EC2 nên chưa cần ACM.

## 6. Step 2 - Tạo VPC/Subnet Hoặc Dùng Default VPC

**Step này để làm gì:** EC2 và RDS cần nằm trong network AWS để nói chuyện với nhau.

Cho demo, có thể dùng **default VPC** để giảm setup.

Mô hình đơn giản:

```text
Default VPC
  -> public subnet
  -> EC2 có public IP
  -> RDS nằm trong subnet group của VPC nhưng không public
```

Lưu ý:

- EC2 cần public IP để SSH và nhận HTTP/HTTPS từ browser.
- RDS không nên bật public access.
- EC2 và RDS phải ở cùng VPC hoặc có network route phù hợp.
- Nếu dùng custom VPC, cần hiểu public subnet, route table, internet gateway, subnet group cho RDS.

## 7. Step 3 - Tạo Security Groups

**Step này để làm gì:** kiểm soát port nào được mở ra internet và port nào chỉ mở nội bộ.

### EC2 security group

Inbound:

```text
22   từ IP của bạn        -> SSH
80   từ 0.0.0.0/0         -> HTTP
443  từ 0.0.0.0/0         -> HTTPS nếu bật
```

Nếu muốn test API trực tiếp không qua Nginx:

```text
8099 từ IP của bạn        -> chỉ mở tạm thời
```

Không nên mở public:

```text
5432 PostgreSQL
6379 Redis
6333 Qdrant HTTP
6334 Qdrant gRPC
8090 Asynqmon
```

Lưu ý quan trọng:

- Nginx/Caddy không phải lớp bảo vệ chính cho Redis/Qdrant nếu các port đó đã bị expose ra internet. Lớp chặn chính phải là EC2 security group và Docker port binding.
- Redis, Qdrant, Asynqmon chỉ nên bind nội bộ trong Docker network hoặc bind vào `127.0.0.1` trên EC2 host.
- Không public Qdrant dashboard, Redis, Asynqmon bằng route Nginx/Caddy.
- Khi cần kiểm tra Qdrant/Asynqmon, SSH vào EC2 hoặc dùng SSH local port forwarding.

Ví dụ SSH port forwarding từ máy local:

```bash
# Qdrant dashboard local: http://localhost:6333/dashboard
ssh -i ragchat-demo-key.pem -L 6333:localhost:6333 ubuntu@<ec2-public-dns>

# Asynqmon local: http://localhost:8090
ssh -i ragchat-demo-key.pem -L 8090:localhost:8090 ubuntu@<ec2-public-dns>

# Redis local debug, nếu thật sự cần
ssh -i ragchat-demo-key.pem -L 6379:localhost:6379 ubuntu@<ec2-public-dns>
```

### RDS security group

Inbound:

```text
5432 từ EC2 security group
```

Ý nghĩa: chỉ EC2 chạy app được kết nối tới PostgreSQL.

Lưu ý:

- Không mở RDS `5432` từ `0.0.0.0/0`.
- Redis/Qdrant chạy trên EC2 thì chỉ cần Docker network nội bộ, không cần public port ra internet.
- Asynqmon nếu cần xem UI thì nên bind localhost hoặc chỉ mở theo IP cá nhân.

## 8. Step 4 - Launch EC2 Instance

**Step này để làm gì:** tạo máy chủ chạy Docker Compose stack cho demo.

Gợi ý:

```text
AMI:          Ubuntu LTS hoặc Amazon Linux 2023
Instance:     t3.small hoặc t3.medium
Storage:      30-50 GB gp3
Public IP:    enabled
Name:         ragchat-demo-ec2
IAM role:     ragchat-demo-ec2-role
Security SG:  ragchat-demo-ec2-sg
```

Vì project có Qdrant, Redis, API, worker và có thể frontend trên cùng máy, `t3.micro` có thể hơi yếu. Nên bắt đầu từ `t3.small`, nếu embed/search nặng thì dùng `t3.medium`.

Lưu ý:

- Root volume cần đủ dung lượng cho Docker images, logs, Qdrant data và upload files.
- Nếu upload PDF nhiều, nên cân nhắc mount thêm EBS volume riêng vào `/data`.
- Nếu EC2 bị terminate mà không backup volume, upload local và Qdrant data có thể mất.

## 9. Step 5 - Cài Docker, Docker Compose, Git Và AWS CLI

**Step này để làm gì:** chuẩn bị runtime để chạy app và sync file lên S3.

Trên EC2 cần có:

- Docker Engine.
- Docker Compose plugin.
- Git để pull source code.
- AWS CLI để chạy `aws s3 sync`.

Kiểm tra:

```bash
docker --version
docker compose version
git --version
aws --version
```

Lưu ý:

- User deploy nên có quyền chạy Docker, ví dụ thuộc group `docker`.
- Không nên chạy app bằng root nếu không cần.
- Nếu dùng Ubuntu, sau khi thêm user vào group `docker`, cần logout/login lại.

## 10. Step 6 - Tạo IAM Role Cho EC2 Truy Cập S3

**Step này để làm gì:** cho EC2 sync file lên S3 mà không hardcode AWS access key trong server.

Tạo IAM role:

```text
Role name: ragchat-demo-ec2-role
Trusted entity: EC2
Attach vào EC2 instance
```

Policy tối thiểu cho S3 bucket upload:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "s3:ListBucket"
      ],
      "Resource": "arn:aws:s3:::ragchat-demo-uploads-<account-id>"
    },
    {
      "Effect": "Allow",
      "Action": [
        "s3:GetObject",
        "s3:PutObject",
        "s3:DeleteObject"
      ],
      "Resource": "arn:aws:s3:::ragchat-demo-uploads-<account-id>/*"
    }
  ]
}
```

Lưu ý:

- Không dùng access key hardcode trong `.env`.
- Nếu dùng `aws s3 sync --delete`, role cần quyền `s3:DeleteObject`.
- Nếu không muốn EC2 xóa file trên S3, bỏ `--delete` và bỏ quyền `DeleteObject`.

## 11. Step 7 - Tạo S3 Bucket Cho Uploads

**Step này để làm gì:** có nơi lưu bản backup/copy bền hơn EC2 local disk.

Tên bucket gợi ý:

```text
ragchat-demo-uploads-<account-id>
```

Cấu hình nên bật:

- Block Public Access.
- Bucket versioning nếu muốn khôi phục file bị xóa/ghi đè.
- Server-side encryption mặc định.

Prefix gợi ý:

```text
s3://ragchat-demo-uploads-<account-id>/uploads/
```

Lưu ý:

- S3 không thay thế database metadata. RDS vẫn cần lưu document id, filename, status, storage path, citation metadata.
- Không public bucket nếu file là tài liệu nội bộ.
- Nếu sau này muốn user download file trực tiếp từ S3, dùng presigned URL thay vì public bucket.

## 12. Step 8 - Tạo RDS PostgreSQL

**Step này để làm gì:** tách database metadata ra khỏi EC2 để dữ liệu quan trọng không phụ thuộc hoàn toàn vào Docker volume local.

Gợi ý cấu hình demo:

```text
Engine:        PostgreSQL
Identifier:    ragchat-demo-postgres
DB name:       ragchat
Username:      ragchat_app hoặc postgres
Public access: No
VPC:           cùng VPC với EC2
Security SG:   ragchat-demo-rds-sg
Instance:      db.t4g.micro hoặc db.t4g.small
Storage:       gp3, 20 GB+
Backups:       bật automated backup vài ngày
```

Kết nối từ app:

```text
POSTGRES_HOST=<rds-endpoint>
POSTGRES_PORT=5432
POSTGRES_DB=ragchat
POSTGRES_USER=<user>
POSTGRES_PASSWORD=<password>
POSTGRES_SSLMODE=require hoặc disable tùy cấu hình
POSTGRES_DSN=postgres://<user>:<password>@<rds-endpoint>:5432/ragchat?sslmode=require
```

Lưu ý:

- RDS endpoint không phải IP cố định để hardcode firewall bên ngoài.
- Với RDS, nên dùng password mạnh và lưu trong `.env` trên EC2 hoặc SSM Parameter Store.
- Nếu app báo connection timeout, thường là sai VPC/security group.
- Nếu app báo authentication failed, thường là sai user/password/db.
- Nếu app báo SSL error, kiểm tra `POSTGRES_SSLMODE`.

## 13. Step 9 - Chuẩn Bị Folder Persistent Trên EC2

**Step này để làm gì:** dữ liệu Redis/Qdrant/upload không bị mất khi container restart.

Tạo cấu trúc folder:

```text
/data/ragchat/uploads
/data/ragchat/redis
/data/ragchat/qdrant
/data/ragchat/logs
```

Mapping gợi ý:

```text
UPLOAD_DIR=/data/ragchat/uploads
Redis data  -> /data/ragchat/redis
Qdrant data -> /data/ragchat/qdrant
```

Lưu ý:

- Folder cần quyền đọc/ghi cho user hoặc container đang chạy app.
- Nếu dùng Docker bind mount, chú ý UID/GID của process trong container.
- Upload folder cần được sync lên S3 định kỳ.
- Qdrant data local nên backup nếu demo có dữ liệu quan trọng.

## 14. Step 10 - Chuẩn Bị Docker Compose Cho Demo

**Step này để làm gì:** định nghĩa các process cần chạy: API, worker, frontend, Redis, Qdrant, reverse proxy.

Hiện tại repo có `docker-compose.yml` cho local infra:

```text
postgres
redis
asynqmon
qdrant
```

Khi deploy AWS demo, nên tạo compose riêng, ví dụ:

```text
docker-compose.demo.yml
```

Compose demo nên có:

```text
nginx hoặc caddy
frontend
api
worker
redis
qdrant
asynqmon optional
```

Không cần chạy `postgres` container vì dùng RDS.

Env quan trọng cho API/worker:

```text
APP_HOST=0.0.0.0
APP_PORT=8099
UPLOAD_DIR=/data/ragchat/uploads
REDIS_ADDR=redis:6379
QDRANT_URL=http://qdrant:6333
QDRANT_HOST=qdrant
QDRANT_GRPC_PORT=6334
QDRANT_COLLECTION=documents
POSTGRES_DSN=postgres://<user>:<password>@<rds-endpoint>:5432/ragchat?sslmode=require
OPENAI_API_KEY=<your-key>
```

Lưu ý:

- Trong Docker network, service gọi nhau bằng service name, ví dụ `redis:6379`, `qdrant:6333`.
- Không dùng `localhost` từ trong container để gọi Redis/Qdrant container khác. `localhost` lúc đó là chính container hiện tại.
- API cần bind `0.0.0.0`, không bind `127.0.0.1`, để reverse proxy/container khác gọi được.
- Worker cần dùng cùng env với API để đọc RDS, Redis, Qdrant và OpenAI.

## 15. Step 11 - Build Và Deploy Source Code Lên EC2

**Step này để làm gì:** đưa code và container image lên server chạy demo.

Có 2 cách đơn giản.

### Cách 1: Build trực tiếp trên EC2

Flow:

```text
SSH vào EC2
  -> git clone/pull repo
  -> tạo .env.demo
  -> docker compose build
  -> docker compose up -d
```

Ưu điểm:

- Dễ hiểu.
- Không cần ECR/GitHub Actions.

Nhược điểm:

- Build trên EC2 tốn CPU/RAM.
- Khó chuẩn hóa nếu nhiều server.

### Cách 2: Build image ở local/GitHub rồi push registry

Flow:

```text
Build Docker image
  -> push ECR hoặc Docker Hub
  -> EC2 docker compose pull
  -> docker compose up -d
```

Ưu điểm:

- Deploy nhanh hơn.
- Gần với workflow production hơn.

Nhược điểm:

- Cần thêm registry và auth.
- Nhiều bước hơn cho demo.

Khuyến nghị cho bản đầu:

```text
Build trực tiếp trên EC2 trước.
Khi demo ổn, tách sang ECR/GitHub Actions sau.
```

Lưu ý:

- Không commit `.env.demo` chứa secret.
- OpenAI API key chỉ nên nằm trên EC2 hoặc secret manager.
- Nếu frontend là repo khác, cần build static files và serve qua Nginx/Caddy hoặc container frontend riêng.

## 16. Step 12 - Chạy Database Migration

**Step này để làm gì:** tạo schema PostgreSQL trước khi API/worker xử lý dữ liệu thật.

Repo hiện có migration trong:

```text
db/migrations
```

Có thể chạy:

```bash
make migrate-up
```

Điều kiện:

```text
POSTGRES_DSN phải trỏ tới RDS
EC2 phải connect được RDS
```

Lưu ý:

- Chạy migration một lần trước khi mở app cho user test.
- Không chạy `migrate-down` trên database demo đang có dữ liệu nếu không chắc.
- Nếu migration fail vì network, kiểm tra RDS security group.
- Nếu migration fail vì permission, kiểm tra user database có quyền tạo table/index không.

## 17. Step 13 - Cấu Hình Reverse Proxy

**Step này để làm gì:** browser gọi port chuẩn `80/443`, reverse proxy route request tới frontend và API nội bộ.

Ví dụ route:

```text
http://your-domain/
  -> frontend

http://your-domain/api/
  -> Go API :8099
```

Nếu chưa có frontend, có thể proxy toàn bộ request tới API:

```text
http://your-domain/
  -> Go API :8099
```

Lưu ý:

- API hiện chạy mặc định port `8099`.
- Chỉ cần expose `80/443` ra internet, không cần expose `8099`.
- Không tạo Nginx/Caddy route public tới Redis `6379`, Qdrant `6333/6334`, hoặc Asynqmon `8090`.
- Nếu cần mở Qdrant dashboard hoặc Asynqmon để debug, dùng SSH port forwarding thay vì public URL.
- Nếu API có SSE streaming, reverse proxy cần tắt buffering cho endpoint stream.
- Nếu upload PDF lớn, cần tăng body size limit trong Nginx/Caddy.
- Nếu frontend và backend khác origin, cần xử lý CORS. Dễ nhất là dùng same-origin qua reverse proxy.

## 18. Step 14 - HTTPS Với Domain

**Step này để làm gì:** browser truy cập an toàn hơn, tránh cảnh báo insecure, phù hợp demo cho người ngoài.

Với mô hình EC2 đơn giản, có 2 hướng:

```text
Nginx + Let's Encrypt certbot
Caddy tự động lấy certificate
```

Flow:

```text
Mua hoặc dùng domain sẵn
  -> tạo DNS A record trỏ về EC2 public IP
  -> mở port 80 và 443
  -> chạy certbot hoặc Caddy
  -> test https://your-domain
```

Lưu ý:

- Let's Encrypt cần domain trỏ đúng về EC2 và port `80`/`443` mở.
- Nếu EC2 stop/start mà public IP đổi, DNS record sẽ sai. Nên dùng Elastic IP nếu cần domain ổn định.
- Với plan này chưa dùng AWS ACM vì ACM certificate không gắn trực tiếp vào Nginx trên EC2 theo cách đơn giản.
- Nếu chỉ test nội bộ nhanh, HTTP bằng public IP vẫn dùng được, nhưng không đẹp cho demo public.

## 19. Step 15 - Cấu Hình S3 Sync Job

**Step này để làm gì:** copy file upload từ EC2 local disk lên S3 định kỳ để giảm rủi ro mất file.

Lệnh sync:

```bash
aws s3 sync /data/ragchat/uploads s3://ragchat-demo-uploads-<account-id>/uploads
```

Nếu muốn xóa file trên S3 khi local bị xóa:

```bash
aws s3 sync /data/ragchat/uploads s3://ragchat-demo-uploads-<account-id>/uploads --delete
```

Chạy định kỳ bằng cron, ví dụ mỗi phút:

```text
* * * * * aws s3 sync /data/ragchat/uploads s3://ragchat-demo-uploads-<account-id>/uploads >> /data/ragchat/logs/s3-sync.log 2>&1
```

Hoặc dùng `systemd timer` nếu muốn quản lý tốt hơn.

Lưu ý:

- `aws s3 sync` không phải real-time tuyệt đối. File có thể lên S3 trễ theo lịch chạy.
- Nếu upload file lớn, tránh sync khi file chưa ghi xong. Backend nên ghi file atomically hoặc dùng temp file rồi rename.
- Nếu dùng `--delete`, xóa nhầm local sẽ xóa theo trên S3 trong lần sync sau.
- S3 sync là backup/copy đơn giản, chưa phải storage architecture hoàn chỉnh.
- Nếu muốn S3 là source of truth, backend nên upload trực tiếp S3 hoặc dùng presigned upload.

## 20. Step 16 - Health Check Và Smoke Test

**Step này để làm gì:** xác nhận từng lớp đã chạy đúng trước khi demo.

Checklist trên EC2:

```bash
docker compose ps
docker compose logs api
docker compose logs worker
docker compose logs redis
docker compose logs qdrant
```

Test API:

```bash
curl http://localhost:8099/api/v1/health
```

Endpoint health hiện tại nằm dưới prefix `/api/v1`.

Test public endpoint:

```bash
curl http://your-domain/api/...
```

Test RDS:

```text
API start không báo lỗi connect database.
Migration chạy thành công.
Upload tạo metadata trong PostgreSQL.
```

Test Redis/worker:

```text
Upload document trả status pending.
Worker nhận job.
Status chuyển pending -> parsing -> chunked/ready.
```

Test Qdrant:

```text
Sau khi embedding xong, search/chat trả được context nếu các endpoint này đã hoàn thiện.
Qdrant collection documents tồn tại.
```

Test S3:

```text
Upload file vào app.
Đợi sync job chạy.
File xuất hiện ở s3://ragchat-demo-uploads-<account-id>/uploads/
```

Lưu ý:

- Nên test từng lớp theo thứ tự, đừng debug tất cả cùng lúc.
- Nếu upload thành công nhưng chat không có context, kiểm tra worker và Qdrant trước.
- Nếu worker chạy nhưng embed fail, kiểm tra `OPENAI_API_KEY`, quota và model name.

## 21. Step 17 - Logging Và Debug

**Step này để làm gì:** biết xem lỗi ở đâu khi demo bị hỏng.

Nguồn log chính:

```text
docker compose logs api
docker compose logs worker
docker compose logs qdrant
docker compose logs redis
/data/ragchat/logs/s3-sync.log
Nginx/Caddy access/error logs
RDS logs trong AWS Console nếu cần
```

Lỗi thường gặp:

```text
API không start
  -> sai env, sai Postgres DSN, thiếu OpenAI key

Migration fail
  -> RDS SG chưa cho EC2 vào, sai user/password/db

Worker không xử lý job
  -> worker chưa chạy, REDIS_ADDR sai, queue name không khớp

Chat không tìm được context
  -> Qdrant chưa có vector, embedding fail, collection name sai

S3 không sync
  -> EC2 IAM role thiếu quyền, bucket name sai, AWS CLI region/profile sai

Public URL không vào được
  -> EC2 SG chưa mở port 80/443, reverse proxy chưa chạy, DNS sai
```

Lưu ý:

- Docker container restart liên tục thường do app crash khi đọc config.
- `localhost` trong container không phải localhost của EC2 host.
- Khi nghi ngờ network, test từ trong container bằng curl/wget nếu image có tool.

## 22. Step 18 - Backup Và Data Durability

**Step này để làm gì:** tránh mất dữ liệu demo sau khi EC2 restart/terminate hoặc thao tác nhầm.

Nên có:

- RDS automated backups.
- S3 versioning cho uploads nếu file quan trọng.
- Snapshot EBS nếu Qdrant data quan trọng.
- Backup compose/env file ở nơi an toàn nhưng không public secret.

Lưu ý:

- RDS bảo vệ metadata, không bảo vệ Qdrant vectors.
- S3 bảo vệ uploaded files, không tự rebuild Qdrant index.
- Nếu mất Qdrant data nhưng còn file + metadata, có thể chạy job re-embed/reindex nếu app hỗ trợ.
- Redis thường chỉ là queue/cache, có thể mất job nếu chưa cấu hình persistence tốt.

## 23. Step 19 - Cost Và Cleanup

**Step này để làm gì:** tránh demo xong nhưng tài nguyên vẫn tính tiền.

Tài nguyên có thể tốn tiền:

- EC2 instance.
- EBS volume.
- RDS instance.
- RDS storage/backup.
- S3 storage/request.
- Elastic IP nếu không gắn vào instance đang chạy.
- NAT Gateway nếu sau này dùng custom private subnet.

Cleanup khi không dùng nữa:

```text
Stop hoặc terminate EC2.
Delete RDS nếu không cần dữ liệu.
Delete RDS snapshots nếu không cần.
Empty và delete S3 bucket nếu không cần uploads.
Release Elastic IP nếu có.
Delete security groups/key pair nếu không dùng.
```

Lưu ý:

- Stop EC2 vẫn tính tiền EBS volume.
- RDS stopped có giới hạn thời gian, AWS có thể tự start lại sau một khoảng nhất định.
- S3 bucket không xóa được nếu còn object bên trong.

## 24. Quy Trình Làm Việc Đề Xuất

Thứ tự triển khai nên đi như sau:

1. Chọn region và thống nhất namespace `ragchat-demo`.
2. Tạo security groups cho EC2 và RDS.
3. Launch EC2.
4. Cài Docker, Docker Compose, Git, AWS CLI trên EC2.
5. Tạo S3 bucket uploads.
6. Tạo IAM role cho EC2 và attach policy S3.
7. Tạo RDS PostgreSQL private.
8. SSH vào EC2, clone repo.
9. Tạo `.env.demo` trỏ tới RDS, Redis, Qdrant, OpenAI.
10. Chuẩn bị folder `/data/ragchat/...`.
11. Chuẩn bị Docker Compose demo không chạy Postgres local.
12. Build và start containers.
13. Chạy database migrations.
14. Cấu hình Nginx/Caddy reverse proxy.
15. Optional: trỏ domain và bật HTTPS.
16. Cấu hình cron/systemd timer để sync uploads lên S3.
17. Smoke test upload, worker, search/chat, S3 sync.
18. Ghi lại runbook debug và cleanup checklist.

## 25. Checklist Trước Khi Demo

Trước buổi demo, kiểm tra:

- EC2 đang running.
- Domain hoặc public IP truy cập được.
- API container healthy.
- Worker container running.
- Redis container running.
- Qdrant container running.
- RDS available.
- Migration đã chạy.
- OpenAI API key còn quota.
- Upload PDF thành công.
- Document status chuyển sang ready/chunked theo flow hiện tại của app.
- Chat trả answer có citation/context nếu phần chat/search đã được bật trong API.
- File upload xuất hiện trên S3 sau sync.
- Không public RDS/Redis/Qdrant ra internet.
- Có cách SSH vào EC2 để xem log khi lỗi.

## 26. Khi Nào Nên Nâng Cấp Kiến Trúc?

Plan này phù hợp demo. Nên nâng cấp khi:

- Có nhiều user thật.
- EC2 một máy không đủ CPU/RAM.
- Cần zero-downtime deploy.
- Cần autoscaling.
- Cần private subnet đúng chuẩn hơn.
- Cần quản lý secret bài bản hơn.
- Cần observability tốt hơn.
- Cần file upload trực tiếp S3 thay vì sync local.
- Cần Qdrant managed hoặc cluster riêng.

Hướng nâng cấp sau demo:

```text
EC2 Compose
  -> ECR + GitHub Actions
  -> ECS Fargate
  -> ALB + ACM HTTPS
  -> SSM/Secrets Manager
  -> CloudWatch Logs
  -> S3 direct upload/presigned URL
  -> Managed Redis hoặc ElastiCache nếu cần
  -> Qdrant Cloud hoặc self-hosted riêng nếu dữ liệu lớn
```

## 27. Kết Luận

Với requirement hiện tại, hướng hợp lý nhất là:

```text
1 EC2 instance
  + Docker Compose
  + RDS PostgreSQL
  + Redis/Qdrant local containers
  + local upload folder
  + S3 sync job
  + Nginx/Caddy reverse proxy
```

Đây là kiến trúc đủ tốt cho demo vì ít moving parts, dễ hiểu, dễ debug và vẫn giữ database/upload files ở nơi bền hơn so với chỉ để trong container.
