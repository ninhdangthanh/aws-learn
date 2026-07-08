# Deploy đơn giản nhất lên AWS ECS Fargate + ALB (không HTTPS)

Mục tiêu: 1 endpoint cố định, mở trình duyệt vào đó là chạy, FE gọi BE bình thường.
Không HTTPS, không domain, không security thừa. Dùng cho demo / học.
Không quản lý server EC2 (dùng Fargate).

> ⚠️ ALB cho **DNS name cố định** (vd `s3up-alb-123.ap-southeast-1.elb.amazonaws.com`),
> **không phải IP tĩnh**. IP sau ALB vẫn đổi. Với "FE có endpoint cố định để gọi BE"
> thì DNS ALB là đủ. Nếu bắt buộc **IP tĩnh literal** → phải dùng **NLB + Elastic IP**
> (xem ghi chú cuối), phức tạp hơn.

## Kiến trúc chọn (và tại sao)

- **ECS Fargate** (serverless, không quản lý EC2).
- **1 ALB** internet-facing, listener HTTP **:80**.
- **1 Fargate service**, **1 task**, **2 container**:
  - `web` = nginx: serve FE tĩnh ở port **80** + reverse proxy sang BE.
  - `api` = Go backend, nghe **8080** (chỉ nội bộ trong task).
- **nginx proxy** để FE và BE dùng chung 1 endpoint → FE gọi BE **same-origin**,
  không cần CORS FE↔BE, không cần build lại FE khi đổi endpoint.

```
Browser ──http──> ALB:80 ──> TargetGroup ──> web (nginx, container port 80)
                                              ├── "/"          -> file tĩnh FE (dist/)
                                              └── "/uploads/*" -> 127.0.0.1:8080 (container api)
Browser ──https─> S3 bucket   (upload part trực tiếp qua presigned URL)
```
Vì task dùng `networkMode: awsvpc` (mặc định của Fargate), 2 container chung
network namespace → `web` gọi `api` qua `127.0.0.1:8080`.

## Chuẩn bị 1 lần (IAM, S3)

1. **S3 bucket** đã có (biến `S3_BUCKET`).
2. **S3 CORS**: vì browser PUT part **thẳng lên S3**, phải cho phép origin
   `http://<ALB_DNS>`. Vào bucket → Permissions → CORS:
   ```json
   [
     {
       "AllowedOrigins": ["http://<ALB_DNS>"],
       "AllowedMethods": ["PUT", "GET", "HEAD"],
       "AllowedHeaders": ["*"],
       "ExposeHeaders": ["ETag"]
     }
   ]
   ```
   (Điền ALB DNS sau khi tạo ALB. Muốn lười có thể để `"*"`.)
3. **ECS Task Role** có quyền S3 trên bucket:
   `s3:PutObject`, `s3:GetObject`, `s3:AbortMultipartUpload`,
   `s3:ListMultipartUploadParts`, `s3:ListBucketMultipartUploads`.
4. **ECS Task Execution Role** (`ecsTaskExecutionRole`) để kéo image từ ECR +
   ghi log CloudWatch. (AWS thường tạo sẵn khi dùng Console.)

## Các file cần thêm vào repo

### 1. `backend/Dockerfile`
```dockerfile
FROM golang:1.22-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /server ./main.go

FROM alpine:3.20
COPY --from=build /server /server
EXPOSE 8080
ENTRYPOINT ["/server"]
```

### 2. `frontend/Dockerfile`
```dockerfile
FROM node:20-alpine AS build
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
# VITE_API_BASE rỗng => FE gọi đường dẫn tương đối (same-origin qua nginx).
# Dùng file .env.production (Vite chắc chắn đọc file .env), KHÔNG dùng ENV
# trong Dockerfile vì Vite không luôn inject biến từ process.env.
RUN echo "VITE_API_BASE=" > .env.production
RUN npm run build

FROM nginx:1.27-alpine
COPY --from=build /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
```

> `api.ts` dùng `import.meta.env.VITE_API_BASE ?? "http://localhost:8080"`.
> `??` coi chuỗi rỗng `""` là hợp lệ => `API_BASE = ""` => `fetch("/uploads/init")`
> đường dẫn tương đối. Đúng ý ta. Phải có `.env.production` với `VITE_API_BASE=`
> nếu không Vite trả `undefined` => rơi về `localhost:8080` (sai).

### 3. `frontend/nginx.conf`
```nginx
server {
    listen 80;

    # ALB health check sẽ gọi "/" -> phải trả 200 (index.html có sẵn)
    location / {
        root /usr/share/nginx/html;
        try_files $uri /index.html;
    }

    # Proxy API sang backend cùng task
    location /uploads/ {
        proxy_pass http://127.0.0.1:8080;
    }
    location /healthz {
        proxy_pass http://127.0.0.1:8080;
    }
}
```

## Các bước deploy

### B1. Push image lên ECR
```bash
AWS_REGION=ap-southeast-1
ACCOUNT=$(aws sts get-caller-identity --query Account --output text)
REPO=$ACCOUNT.dkr.ecr.$AWS_REGION.amazonaws.com

aws ecr create-repository --repository-name s3up-api --region $AWS_REGION
aws ecr create-repository --repository-name s3up-web --region $AWS_REGION
aws ecr get-login-password --region $AWS_REGION | docker login --username AWS --password-stdin $REPO

# build (chạy trong notes/multipartS3Upload)
docker build -t $REPO/s3up-api:latest ./backend
docker build -t $REPO/s3up-web:latest ./frontend
docker push $REPO/s3up-api:latest
docker push $REPO/s3up-web:latest
```
> Fargate chạy trên linux/amd64. Nếu build trên Mac Apple Silicon (arm64), thêm
> `--platform linux/amd64` vào `docker build` (hoặc dùng `docker buildx`).

### B2. Task Definition (Fargate)
- Launch type: **Fargate**, `networkMode: awsvpc` (bắt buộc với Fargate).
- CPU/Mem tối thiểu: 0.25 vCPU / 0.5 GB là đủ demo.
- Task Role = role S3 ở trên. Execution Role = `ecsTaskExecutionRole`.
- Container `api`: image `s3up-api:latest`, env:
  `S3_BUCKET`, `AWS_REGION`, `S3_KEY_PREFIX`, `PART_SIZE_BYTES`,
  `PRESIGN_EXPIRY_SECONDS`. (Không cần `CORS_ORIGIN` vì same-origin.)
- Container `web`: image `s3up-web:latest`, **containerPort 80**.
  (Fargate awsvpc: không cần khai hostPort.)

### B3. Tạo ALB
- EC2 Console → Load Balancers → Create **Application Load Balancer**,
  internet-facing, chọn ≥2 public subnet.
- Listener: **HTTP :80**.
- **Target group**: type **IP**, protocol HTTP **port 80**,
  health check path `/` (nginx trả 200).
- **Security group**:
  - ALB SG: inbound **80** từ `0.0.0.0/0`.
  - Service SG: inbound **80** từ ALB SG (chỉ ALB gọi được task).

### B4. ECS Service
- Cluster: tạo cluster Fargate (không cần EC2).
- Create Service từ task definition, launch type **Fargate**, desired count **1**.
- Networking: chọn VPC + public subnet, gán Service SG ở trên.
  (Bật "Auto-assign public IP" nếu subnet không có NAT, để task kéo được image.)
- Load balancing: chọn ALB + target group ở B3, container để map = **`web` : 80**.

### B5. Xong
- Lấy **DNS name của ALB** (EC2 → Load Balancers). Điền vào S3 CORS (B chuẩn bị #2).
- Mở trình duyệt: `http://<ALB_DNS>` → FE hiện ra.
- FE gọi `http://<ALB_DNS>/uploads/init` → ALB → nginx → BE → trả presigned URL →
  browser PUT part thẳng lên S3.
- Kiểm tra BE sống: `http://<ALB_DNS>/healthz` → `{"status":"ok"}`.

## Chi phí ước tính
- ALB ~ $16–18/tháng (phí cố định) + Fargate 0.25vCPU/0.5GB ~ $9/tháng.
  (ALB là khoản đáng kể nhất — đây là cái giá để không dùng EC2.)

## Ghi chú / cạm bẫy
- **S3 CORS bắt buộc** — thiếu là part upload fail (dù FE↔BE OK).
  `ExposeHeaders: ["ETag"]` để FE đọc ETag khi complete multipart.
- Đổi endpoint (tạo ALB mới) → chỉ sửa lại S3 CORS, **không build lại FE**
  (FE dùng đường dẫn tương đối).
- ALB health check phải trỏ path trả 200 (`/`), nếu không target = unhealthy →
  ALB trả 503.
- **Cần IP tĩnh literal?** ALB không có. Thay ALB bằng **NLB** (listener TCP:80),
  và cấp **Elastic IP** cho từng AZ của NLB (NLB hỗ trợ gán EIP). Phần còn lại
  (Fargate, 2 container, nginx proxy) giữ nguyên. Đổi lại phức tạp & mất tính năng
  health-check HTTP/routing của ALB.
- Không HTTPS nên browser hiện "Not secure" — chấp nhận vì demo.
```
