# AWS Deployment Plan Cho RAG Chatbot AI Backend

## Mục Tiêu

Deploy backend RAG và frontend React lên AWS, dùng dịch vụ AWS phù hợp để đảm bảo:

- Backend ổn định và có thể mở rộng.
- File upload được lưu trữ tin cậy.
- Dữ liệu metadata được quản lý bằng dịch vụ managed.
- HTTPS được dùng với browser/public access.
- Tránh mở các dịch vụ stateful ra internet.

## Quyết định chính

- FE và BE sẽ deploy chung trên 1 EC2 instance.
- Dùng Docker Compose trên EC2 để chạy backend, frontend, Redis và Qdrant cho demo.
- PostgreSQL dùng dịch vụ managed của AWS (RDS) thay vì chạy trong Docker.
- File upload sẽ được lưu vào một thư mục chung trên EC2, rồi tự đồng bộ lên Amazon S3.
- Không dùng ECS, ALB, API Gateway hay các dịch vụ phức tạp.
- Chỉ dùng cho demo, không phải production.

## Kiến trúc đơn giản cho demo

```text
Browser
  -> EC2 public IP / domain
  -> Nginx hoặc reverse proxy
  -> React frontend
  -> Go API backend
  -> RDS PostgreSQL service của AWS
  -> Redis / Qdrant chạy trong Docker trên cùng EC2
  -> thư mục upload trên EC2
  -> S3 bucket (sync tự động)
```

## Có cần HTTPS không?

Với demo đơn giản, có thể dùng HTTP tạm thời nếu bạn chỉ mở cho nội bộ hoặc test nhanh.

Nếu muốn đẹp hơn và dễ dùng từ browser, nên bật HTTPS bằng Nginx + Let's Encrypt. Đây là bước optional, không bắt buộc cho MVP demo.

## Cách lưu file trên S3

Hiện tại bạn đang upload trực tiếp vào thư mục local. Với mô hình này, cách đơn giản nhất là:

- backend nhận file upload,
- lưu vào một thư mục chung như `/data/uploads` trên EC2,
- rồi dùng một job sync tự động đẩy lên S3.

### Cách 1: Dùng `aws s3 sync` chạy nền trên EC2 (đề xuất)

Đây là cách đơn giản nhất và dễ quản lý nhất.

- Cài AWS CLI trên EC2.
- Gắn IAM role cho EC2 cho phép write vào S3 bucket.
- Chạy lệnh:

```bash
aws s3 sync /data/uploads s3://your-bucket/uploads --delete
```

- Đưa lệnh này vào `systemd` service hoặc `cron` để chạy định kỳ, ví dụ mỗi 1 phút.

Ưu điểm:

- Dễ hiểu, dễ debug.
- Không cần thêm container riêng.
- Phù hợp với demo.

### Cách 2: Dùng Docker image hỗ trợ sync sẵn

Đây là lựa chọn khép kín hơn nếu bạn muốn mọi thứ chạy trong Docker.

- Tạo một container phụ chỉ làm nhiệm vụ sync file từ volume Docker sang S3.
- Container này chạy định kỳ bằng cron hoặc script nội bộ.

Ưu điểm:

- Tất cả logic sync nằm trong Docker stack.
- Dễ quản lý cùng các service khác.

Nhược điểm:

- Hơi nhiều bước hơn cách 1.

Chi tiết triển khai cho cách này sẽ được viết riêng trong một file note khác.

## Hạ tầng đề xuất cho demo

### EC2 instance

- 1 instance Ubuntu hoặc Amazon Linux.
- Cài Docker + Docker Compose.
- Mở port:
  - `22` cho SSH,
  - `80` cho frontend/backend,
  - `443` nếu bật HTTPS.

### Docker Compose services

Trên cùng một EC2, chạy:

- frontend React
- backend Go API
- Redis
- Qdrant

### Persistent storage

- Dùng bind mount để lưu file upload và dữ liệu local trên EC2.
- Ví dụ:

```text
/data/uploads     -> file upload của app
/data/redis       -> dữ liệu Redis
/data/qdrant      -> dữ liệu Qdrant
```

## Bảo mật đơn giản

- Chỉ mở cổng `80`/`443` ra internet.
- Không cần mở PostgreSQL, Redis, Qdrant ra public internet.
- Dùng IAM role cho EC2 để truy cập S3.
- Nếu có thể, dùng `aws configure` hoặc role IAM thay vì hardcode key.

## Quy trình làm việc đề xuất

1. Launch EC2 instance.
2. Cài Docker + Docker Compose.
3. Cài AWS CLI và gắn IAM role cho S3.
4. Tạo bucket S3 cho uploads.
5. Chạy Docker Compose để start FE/BE/Redis/Qdrant.
6. Đặt thư mục upload vào bind mount.
7. Cấu hình sync job từ EC2 folder sang S3.
8. Test upload → file xuất hiện trong S3.

## Khuyến nghị cho project này

Với mục tiêu demo, cách hợp lý nhất là:

- dùng 1 EC2 instance,
- deploy FE + BE chung,
- dùng S3 làm nơi lưu file chính,
- sync đơn giản bằng `aws s3 sync`.

Đây là giải pháp dễ nhất, ít phức tạp nhất, và đủ cho việc demo nội bộ hoặc cho người dùng thử nhanh.

## Ghi chú

Chi tiết triển khai cho việc sync file từ EC2 sang S3 sẽ được viết riêng trong file note: [docs/s3-sync-note.md](./s3-sync-note.md).
