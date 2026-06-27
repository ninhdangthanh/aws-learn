# Câu Hỏi Kiến Trúc AWS Cho Project Video Call

File này gom các câu hỏi kiến trúc/vận hành phát sinh trong lúc deploy project `video-call/`. Các ghi chú nền tảng vẫn nằm ở:

- [AWS deployment guide](AWS_DEPLOYMENT_GUIDE.md)
- [Route 53, ACM, and ALB notes](route53_acm_alb_notes.md)

## 1. Domain Mua Ngoài AWS Khác Gì So Với Domain Mua Ở Route 53?

Với project này:

```text
Domain mua ở: INET
DNS quản lý ở: Route 53 Hosted Zone
```

Khác biệt chính so với mua domain trực tiếp từ Route 53 Domains là bạn phải vào INET để đổi nameserver sang 4 nameserver mà Route 53 cấp.

Các phần còn lại giống nhau:

- Tạo Route 53 hosted zone.
- Tạo ACM certificate.
- Validate ACM bằng CNAME trong hosted zone.
- Tạo A Alias record trỏ domain về ALB.
- Gắn ACM certificate vào ALB HTTPS listener.
- ALB forward request vào ECS.

Route 53 không chỉ là nơi bán domain:

```text
Route 53 Domains      -> mua/gia hạn/chuyển domain
Route 53 Hosted Zones -> quản lý DNS records
Route 53 Records      -> A, A Alias, CNAME, MX, TXT...
Route 53 Health Checks -> health check/routing nâng cao
```

## 2. Nếu Mua Domain Mới Thì Cần Làm Gì?

Flow chuẩn:

```text
Mua domain mới
  -> tạo Route 53 public hosted zone
  -> đổi nameserver ở nơi mua domain sang 4 nameserver của Route 53
  -> tạo ACM certificate ở ap-southeast-1
  -> validate ACM bằng CNAME record trong hosted zone
  -> tạo A Alias record trỏ domain về ALB
  -> gắn certificate mới vào ALB HTTPS listener :443
  -> thêm domain mới vào ALB host header rule nếu đang giới hạn theo domain
```

Nếu ALB listener `443` đang có rule:

```text
Host header = ninh-video-call-demo.food
```

thì domain mới sẽ chưa vào app cho đến khi bạn thêm rule mới hoặc thêm domain mới vào rule hiện tại:

```text
Host header = my-new-call-demo.com
  -> forward to frontend target group
```

Tóm tắt:

```text
DNS đúng mới tới được ALB.
Certificate đúng mới HTTPS được.
Host header rule đúng mới vào đúng target group.
```

## 3. Hiện Trạng Project Đang Deploy Như Thế Nào?

Hiện tại project dùng một ECS service:

```text
ECS cluster:  videocall-cluster
ECS service:  videocall-service
Task family:  videocall-task
```

Trong một ECS task có 2 container:

```text
backend container  :8080
frontend container :80
```

Backend và frontend đang chạy chung vòng đời:

- Deploy service là deploy cả backend và frontend.
- Scale service là scale cả backend và frontend cùng nhau.
- Nếu task restart, cả backend và frontend cùng restart.
- Frontend gọi backend qua `localhost:8080` vì hai container cùng nằm trong một ECS task.

Task definition hiện tại có:

```json
"networkMode": "awsvpc"
```

```json
"name": "BACKEND_URL",
"value": "http://localhost:8080"
```

`dependsOn` của frontend:

```json
"dependsOn": [
  {
    "containerName": "backend",
    "condition": "START"
  }
]
```

Ý nghĩa: ECS start backend trước, rồi mới start frontend.

## 4. Hiện Tại FE Call BE Bằng Cách Nào?

Browser không gọi thẳng backend container.

Browser gọi cùng domain:

```text
https://ninh-video-call-demo.food/api/...
wss://ninh-video-call-demo.food/ws
```

Frontend production dùng same-origin:

```text
API_URL = window.location.origin
WS_URL  = wss://<current-host>/ws
```

Sau đó request đi:

```text
Browser
  -> ALB HTTPS :443
  -> frontend Nginx :80
  -> /api hoặc /ws
  -> backend :8080
```

Frontend Nginx proxy sang backend bằng:

```nginx
location /api/ {
    proxy_pass ${BACKEND_URL};
}

location /ws {
    proxy_pass ${BACKEND_URL};
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "Upgrade";
}
```

Trong ECS task definition:

```text
BACKEND_URL=http://localhost:8080
```

## 5. Có Tách Frontend Và Backend Thành 2 ECS Service Được Không?

Có. Backend và frontend có thể tách thành 2 ECS service cùng thuộc một ECS cluster.

Ví dụ:

```text
ECS cluster: videocall-cluster

Service frontend:
  task: frontend-task
  container: frontend :80
  gắn với ALB public target group

Service backend:
  task: backend-task
  container: backend :8080
  private hoặc gắn target group riêng
```

Khi tách service, `BACKEND_URL=http://localhost:8080` không còn đúng nữa.

Có 2 hướng phổ biến.

### Route Bằng ALB

Tạo 2 target groups:

```text
frontend target group: port 80
backend target group:  port 8080
```

Listener rules:

```text
/api/* -> backend target group
/ws*   -> backend target group
/*     -> frontend target group
```

Browser vẫn gọi:

```text
https://ninh-video-call-demo.food/api/...
wss://ninh-video-call-demo.food/ws
```

nhưng ALB route trực tiếp sang backend service.

### Service Discovery Nội Bộ

Dùng ECS Service Connect hoặc AWS Cloud Map để backend có DNS nội bộ:

```text
http://backend.videocall.local:8080
```

Frontend Nginx dùng:

```text
BACKEND_URL=http://backend.videocall.local:8080
```

Với demo hiện tại, để chung một service vẫn hợp lý. Khi cần scale frontend/backend độc lập, hãy tách service.

## 6. ALB Và API Gateway Khác Nhau Thế Nào?

ALB phù hợp với app web chạy container:

- Route HTTP/HTTPS theo host/path.
- Hỗ trợ WebSocket.
- Gắn trực tiếp với ECS target group.
- Phù hợp để serve frontend và route API/WebSocket.

API Gateway phù hợp khi cần quản lý API chuyên sâu:

- Rate limit/throttling.
- API key.
- Authorizer.
- Stage/version API.
- Transform request/response.
- Tích hợp Lambda hoặc HTTP backend.

API Gateway có thể đưa vào project này không?

Có, nhưng hiện tại không bắt buộc. Với app này, ALB đã đủ để route:

```text
Frontend: /
API:      /api/*
WS:       /ws
```

Sau này nếu cần API key, rate limit, authorizer riêng, hoặc muốn tách API management khỏi ALB, lúc đó hãy cân nhắc API Gateway.

## 7. Export Config Về Terraform Được Không?

Được, nhưng Terraform không tự biết tài nguyên bạn đã click tạo trên AWS. Bạn cần import tài nguyên thật vào Terraform state hoặc dùng tool generate.

Các hướng phổ biến:

- Viết Terraform thủ công rồi `terraform import`.
- Dùng `import { ... }` block.
- Dùng tool như `terraformer` để generate bước đầu.

Nhóm tài nguyên nên đưa vào Terraform:

- VPC/subnets/route tables.
- Security groups.
- ALB, listeners, listener rules.
- Target groups.
- Route 53 hosted zone/records.
- ACM certificate/validation records.
- ECR repositories.
- ECS cluster/service.
- IAM role/policies.
- CloudWatch log group.
- SSM parameter metadata.

Khuyến nghị:

```text
Terraform quản lý hạ tầng nền.
GitHub Actions quản lý build image và deploy revision mới.
```

Nếu Terraform quản lý ECS service, cần tránh để Terraform rollback image tag mà GitHub Actions vừa deploy.

## 8. VPC, Subnet, Security Group Là Gì?

VPC là mạng riêng ảo trong AWS account:

```text
VPC = private network của bạn trên AWS
```

Subnet là vùng mạng nhỏ trong VPC, thường nằm trong một Availability Zone.

- ALB internet-facing thường nằm ở public subnets.
- ECS task production thường nên nằm ở private subnets.
- Nếu ECS task ở private subnet, cần NAT Gateway hoặc VPC endpoints để pull ECR image, ghi CloudWatch logs, đọc SSM.

Security group là firewall:

```text
Inbound  = traffic đi vào resource
Outbound = traffic đi ra khỏi resource
```

ALB security group:

```text
Inbound:
  TCP 80, 443 từ internet
```

ECS service security group:

```text
Inbound:
  TCP 80 từ ALB security group
```

Không cần mở backend `8080` ra internet.

## 9. Register Task Definition, Task Definition ARN Và Task ARN

Task definition là bản thiết kế container. Khi chạy:

```bash
aws ecs register-task-definition \
  --cli-input-json file://video-call/.aws/task-definition.json \
  --region ap-southeast-1
```

AWS tạo task definition revision mới:

```text
arn:aws:ecs:ap-southeast-1:<ACCOUNT_ID>:task-definition/videocall-task:12
```

Task ARN là task đang chạy cụ thể:

```text
arn:aws:ecs:ap-southeast-1:<ACCOUNT_ID>:task/videocall-cluster/abc123...
```

So sánh:

```text
Task definition ARN = bản thiết kế.
Task ARN            = instance đang chạy từ bản thiết kế.
```

## 10. Lưu File Database Trong ECS Có An Toàn Không?

Hiện backend dùng:

```text
DB_PATH=/app/videocall.db
```

Nếu file nằm trong container filesystem, nó không bền vững. Khi task restart/deploy lại, file có thể mất.

Các lựa chọn tốt hơn:

- RDS: phù hợp nhất cho database production.
- EFS: mount filesystem bền vững vào ECS task, dùng được cho file nhưng cần cẩn thận nếu dùng SQLite.
- S3: phù hợp lưu object/file/backup, không phù hợp làm database realtime.

Nếu muốn backup file database lên S3, app hoặc job riêng có thể định kỳ upload snapshot:

```text
/app/videocall.db -> s3://bucket/backups/videocall.db
```

Nhưng backup lên S3 không thay thế RDS cho database đang ghi liên tục.

## 11. Các AWS Service/Tài Nguyên Đang Dùng

Đã dùng:

- ECS: chạy service/task container.
- ECR: lưu Docker images.
- EC2 Application Load Balancer: entrypoint HTTPS/WSS.
- EC2 Target Group: nơi ALB forward traffic.
- EC2 Security Group: firewall cho ALB/ECS.
- IAM Role: `ecsTaskExecutionRole`.
- Route 53 Hosted Zone: DNS domain.
- ACM: HTTPS certificate.
- SSM Parameter Store: runtime secrets.
- CloudWatch Logs: container logs.

Cần hiểu/thêm nếu production:

- VPC/subnets/route tables.
- NAT Gateway hoặc VPC endpoints nếu ECS chạy private subnet.
- RDS hoặc EFS nếu cần data bền vững.
- AWS Backup nếu dùng RDS/EFS production.
- WAF nếu muốn bảo vệ ALB.
- Terraform nếu muốn quản lý hạ tầng bằng code.
- CloudTrail để audit thay đổi trong AWS account.

## 12. Nếu Không Dùng ALB Thì Client Internet Có Truy Cập Frontend ECS Được Không?

Có, nhưng cần một entrypoint public nào đó. Client ngoài internet không thể tự truy cập ECS task private nếu không có public IP, DNS, route table và security group phù hợp.

Các lựa chọn phổ biến:

### Cách 1: Gán Public IP Trực Tiếp Cho ECS Task

Với ECS Fargate, task có thể chạy trong public subnet và bật:

```text
assignPublicIp = ENABLED
```

Khi đó task có public IP, client có thể truy cập trực tiếp:

```text
http://<task-public-ip>:80
```

Điều kiện:

- Task nằm trong public subnet.
- Public subnet có route ra Internet Gateway.
- Security group của ECS task mở inbound `80` hoặc `443` từ internet.
- Container frontend listen đúng port được map.

Nhược điểm:

- Public IP của task có thể đổi khi task restart/deploy.
- Không có HTTPS certificate tự động như ALB + ACM.
- Không có health check/load balancing tốt.
- Khó chạy nhiều task.
- Không đẹp cho production vì phải mở task trực tiếp ra internet.

Kết luận: dùng được để demo nhanh, không nên là hướng production chính.

### Cách 2: Dùng Network Load Balancer

NLB có thể public TCP/UDP traffic tới ECS service.

Luồng:

```text
Browser
  -> NLB public DNS
  -> ECS frontend task
```

Ưu điểm:

- Có endpoint ổn định hơn public IP của task.
- Hỗ trợ TCP/UDP tốt hơn ALB.
- Phù hợp nếu cần pass-through TCP hoặc dùng cho TURN/coturn.

Nhược điểm:

- NLB không có routing HTTP path thông minh như ALB.
- Nếu cần `/api`, `/ws`, `/*` route khác nhau, NLB không tiện bằng ALB.
- HTTPS/TLS và app routing cần tự thiết kế kỹ hơn.

Với web frontend React + API/WebSocket, ALB thường tiện hơn NLB.

### Cách 3: Dùng CloudFront + Origin Là ECS Public Endpoint

CloudFront có thể đứng trước một origin public.

Nhưng CloudFront vẫn cần origin mà nó truy cập được, ví dụ:

```text
CloudFront
  -> ALB
  -> ECS
```

hoặc:

```text
CloudFront
  -> public IP / public DNS khác
  -> ECS/Nginx
```

Nếu bỏ ALB hoàn toàn, vẫn phải có public origin ổn định cho CloudFront. Public IP trực tiếp của ECS task không ổn định, nên không phải lựa chọn đẹp.

CloudFront phù hợp hơn nếu frontend là static files trên S3:

```text
Browser
  -> CloudFront
  -> S3 static frontend
```

Sau đó API/WebSocket vẫn cần một backend endpoint riêng.

### Cách 4: API Gateway Cho API/WebSocket, Frontend Static Trên S3

Một kiến trúc khác là không chạy frontend bằng ECS nữa:

```text
Browser
  -> CloudFront/S3
  -> tải React static files

Browser
  -> API Gateway HTTP/WebSocket API
  -> backend service
```

Ưu điểm:

- Frontend static rẻ và đơn giản.
- Không cần ECS frontend container.
- API Gateway có thể public API/WebSocket.

Nhược điểm:

- Cần thiết kế lại routing/API/WebSocket.
- Backend ECS private cần VPC Link hoặc public endpoint phía sau.
- Với WebSocket signaling hiện tại, cần đổi deployment path và có thể chỉnh lại config.

### Cách 5: App Runner Hoặc Lightsail

Nếu mục tiêu là đơn giản hóa deploy web container, có thể dùng:

- AWS App Runner.
- Lightsail container/service.
- EC2 tự chạy Docker + Nginx.

Các hướng này có public endpoint dễ hơn ECS thuần, nhưng trade-off là ít linh hoạt hơn hoặc không giống architecture ECS Fargate hiện tại.

## Nên Chọn Gì Cho Project Này?

Với project hiện tại:

```text
Frontend React container
Backend Go container
API /api
WebSocket /ws
HTTPS domain riêng
```

ALB vẫn là lựa chọn hợp lý nhất cho web/API/WebSocket:

```text
Browser
  -> Route 53 domain
  -> ALB HTTPS/WSS
  -> ECS frontend/backend
```

Nếu không dùng ALB để tiết kiệm hoặc demo nhanh, cách đơn giản nhất là public IP trực tiếp cho ECS task. Nhưng cần chấp nhận:

- IP không ổn định.
- HTTPS khó hơn.
- Security kém hơn.
- Scale/redeploy dễ gãy endpoint.

Riêng TURN server cho WebRTC media thì không nên đặt sau ALB. TURN nên đi theo hướng riêng:

```text
Browser
  -> TURN server public, ví dụ coturn trên EC2 hoặc sau NLB TCP/UDP
```

Tóm lại:

- Web frontend/API/WebSocket: nên dùng ALB.
- Media relay TURN: dùng EC2 public hoặc NLB, không dùng ALB.
