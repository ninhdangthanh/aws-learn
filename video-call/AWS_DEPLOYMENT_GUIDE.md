# Hướng Dẫn Deploy Video Call Lên AWS ECS

Guide này dùng cho app `video-call/` trong repo `aws-learn`.

Mục tiêu deploy:

- Frontend React/Vite chạy trong container Nginx.
- Backend Go chạy trong container riêng.
- ECS Fargate chạy cả 2 container trong cùng 1 task.
- ALB public internet qua HTTPS.
- API dùng `https://your-domain.com/api/...`.
- WebSocket signaling dùng `wss://your-domain.com/ws`.
- WebRTC media dùng STUN/TURN.

## 1. Lưu Ý Quan Trọng Về Video Call

Browser chỉ cho phép camera/microphone trên:

- `localhost`, hoặc
- site có HTTPS hợp lệ.

Vì vậy khi deploy thật, app phải được mở bằng HTTPS. Khi page đã chạy trên HTTPS, WebSocket cũng phải chạy bằng WSS, không được dùng `ws://`.

Code hiện tại đã được chỉnh để production tự động dùng same-origin:

- API: `window.location.origin`, ví dụ `https://call.example.com`.
- WS: tự động chọn `wss://call.example.com/ws` nếu page đang ở HTTPS.

Bạn không cần sửa `frontend/src/config.ts` mỗi lần đổi domain.

## 2. Kiến Trúc ECS

```text
Internet
  |
  v
Route 53 / DNS
  |
  v
Application Load Balancer
  |-- HTTP :80  -> redirect sang HTTPS
  |-- HTTPS:443 -> target group port 80
  |
  v
ECS Fargate Service
  |
  v
Task: videocall-task
  |-- frontend container :80
  |     |-- serve React static files
  |     |-- proxy /api -> backend:8080
  |     |-- proxy /ws  -> backend:8080
  |
  |-- backend container :8080
        |-- Go API
        |-- WebSocket signaling
        |-- SQLite database path /app/videocall.db
```

Lưu ý: ALB chỉ cần route vào frontend container port 80. Nginx trong frontend sẽ proxy `/api` và `/ws` sang backend trong cùng task.

## 3. Chuẩn Bị

Cần có:

- AWS account.
- AWS CLI đã login/configure.
- GitHub repo chứa code.
- Domain riêng, vì HTTPS cho camera/mic nên cần domain thật.
- ACM certificate cho domain.
- TURN server hoặc TURN provider nếu muốn video call ổn định trên nhiều loại mạng.

Ví dụ trong guide:

```bash
AWS_REGION=us-east-1
ACCOUNT_ID=123456789012
DOMAIN=call.example.com
```

Hãy thay các giá trị này bằng của bạn.

## 4. Tạo ECR Repositories

Tạo 2 repositories:

```bash
aws ecr create-repository --repository-name videocall-backend --region us-east-1
aws ecr create-repository --repository-name videocall-frontend --region us-east-1
```

Sau đó image sẽ có dạng:

```text
<ACCOUNT_ID>.dkr.ecr.<REGION>.amazonaws.com/videocall-backend
<ACCOUNT_ID>.dkr.ecr.<REGION>.amazonaws.com/videocall-frontend
```

## 5. Tạo CloudWatch Log Group

```bash
aws logs create-log-group --log-group-name /ecs/videocall --region us-east-1
```

Nếu log group đã tồn tại thì có thể bỏ qua lỗi `ResourceAlreadyExistsException`.

## 6. Tạo Runtime Secrets Trong SSM

Task definition đang đọc các biến bí mật từ AWS Systems Manager Parameter Store:

- `/videocall/JWT_SECRET`
- `/videocall/TURN_URLS`
- `/videocall/TURN_USERNAME`
- `/videocall/TURN_CREDENTIAL`

Tạo JWT secret:

```bash
aws ssm put-parameter \
  --name /videocall/JWT_SECRET \
  --type SecureString \
  --value "replace-with-a-long-random-secret" \
  --region us-east-1
```

Nếu đã có TURN server/provider:

```bash
aws ssm put-parameter \
  --name /videocall/TURN_URLS \
  --type SecureString \
  --value "turn:turn.example.com:3478,turns:turn.example.com:5349" \
  --region us-east-1

aws ssm put-parameter \
  --name /videocall/TURN_USERNAME \
  --type SecureString \
  --value "your-turn-username" \
  --region us-east-1

aws ssm put-parameter \
  --name /videocall/TURN_CREDENTIAL \
  --type SecureString \
  --value "your-turn-password" \
  --region us-east-1
```

Nếu chưa có TURN và chỉ muốn deploy thử trước, tạo 3 parameter TURN bằng một dấu cách:

```bash
aws ssm put-parameter --name /videocall/TURN_URLS --type SecureString --value " " --region us-east-1
aws ssm put-parameter --name /videocall/TURN_USERNAME --type SecureString --value " " --region us-east-1
aws ssm put-parameter --name /videocall/TURN_CREDENTIAL --type SecureString --value " " --region us-east-1
```

App sẽ trim giá trị này thành rỗng. Tuy nhiên, không có TURN thì video call có thể fail trên 4G/5G, WiFi công ty, hoặc NAT chặt.

## 7. IAM Role Cho ECS Task

Cần có role `ecsTaskExecutionRole`.

Role này cần policy AWS managed:

```text
AmazonECSTaskExecutionRolePolicy
```

Thêm inline policy để ECS đọc SSM parameters:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": ["ssm:GetParameters"],
      "Resource": "arn:aws:ssm:us-east-1:<ACCOUNT_ID>:parameter/videocall/*"
    }
  ]
}
```

Nếu SSM SecureString dùng customer managed KMS key, thêm quyền `kms:Decrypt` cho key đó.

Sau đó copy ARN của role, ví dụ:

```text
arn:aws:iam::<ACCOUNT_ID>:role/ecsTaskExecutionRole
```

## 8. Sửa Task Definition

Mở file:

```text
video-call/.aws/task-definition.json
```

Cần thay:

- `<ACCOUNT_ID>` bằng AWS account ID của bạn.
- `us-east-1` bằng region của bạn nếu khác.
- `executionRoleArn` dùng ARN của `ecsTaskExecutionRole`.

Đang mặc định:

```json
"executionRoleArn": "arn:aws:iam::<ACCOUNT_ID>:role/ecsTaskExecutionRole"
```

Nếu deploy region khác, sửa cả:

- `awslogs-region`
- SSM parameter ARN trong `secrets`
- GitHub Actions `AWS_REGION`

## 9. Tạo ECS Cluster

```bash
aws ecs create-cluster \
  --cluster-name videocall-cluster \
  --region us-east-1
```

Dùng Fargate, không cần tạo EC2 instance cho ECS.

## 10. Tạo ACM Certificate Cho HTTPS

Vào AWS Certificate Manager trong cùng region với ALB, ví dụ `us-east-1`.

Request public certificate cho domain:

```text
call.example.com
```

Validate bằng DNS. Nếu dùng Route 53, AWS có nút tạo DNS record tự động.

Chỉ tiếp tục sau khi certificate status là `Issued`.

## 11. Tạo Security Groups

Cần 2 security groups:

### ALB Security Group

Inbound:

- TCP 80 từ `0.0.0.0/0`
- TCP 443 từ `0.0.0.0/0`

Outbound:

- Cho phép ra ECS service security group port 80.

### ECS Service Security Group

Inbound:

- TCP 80 từ ALB security group.

Không cần public port 8080 của backend ra internet. Backend chỉ được frontend Nginx proxy nội bộ trong task.

## 12. Tạo Target Group

Tạo target group cho frontend:

- Target type: `IP`
- Protocol: `HTTP`
- Port: `80`
- VPC: VPC bạn sẽ chạy ECS
- Health check path: `/`

Đặt tên ví dụ:

```text
videocall-frontend-tg
```

## 13. Tạo Application Load Balancer

Tạo ALB internet-facing:

- Scheme: internet-facing
- Listeners:
  - HTTP 80
  - HTTPS 443
- Subnets: chọn ít nhất 2 public subnets
- Security group: ALB security group

Listener rules:

- HTTP 80: redirect sang HTTPS 443
- HTTPS 443: forward sang target group `videocall-frontend-tg`

Gắn ACM certificate vào listener HTTPS 443.

## 14. Trỏ Domain Về ALB

Nếu dùng Route 53:

- Tạo `A` record alias cho `call.example.com`.
- Alias target: ALB vừa tạo.

Nếu dùng DNS provider khác:

- Tạo `CNAME` từ `call.example.com` về DNS name của ALB.

Chờ DNS propagate xong, domain phải mở được qua HTTPS.

## 15. Register Task Definition Lần Đầu

Từ repo root `aws-learn`, chạy:

```bash
aws ecs register-task-definition \
  --cli-input-json file://video-call/.aws/task-definition.json \
  --region us-east-1
```

Lưu ý: image trong file ban đầu là placeholder `<IMAGE_BE>` và `<IMAGE_FE>`. Nếu AWS không chấp nhận placeholder khi register lần đầu, hãy push image trước bằng GitHub Actions/manual build, sau đó register task definition có image ECR thật.

## 16. Tạo ECS Service

Tạo service trong cluster `videocall-cluster`:

- Launch type: Fargate
- Task definition: `videocall-task`
- Service name: `videocall-service`
- Desired tasks: `1`
- VPC/subnets: chọn private subnets nếu có NAT, hoặc public subnets nếu muốn đơn giản lúc đầu.
- Security group: ECS service security group.
- Load balancer:
  - Type: Application Load Balancer
  - Container: `frontend`
  - Container port: `80`
  - Target group: `videocall-frontend-tg`

Nếu dùng private subnets, task cần ra internet để pull image ECR và ghi logs CloudWatch. Cách đơn giản là NAT Gateway. Cách tối ưu hơn là dùng VPC endpoints cho ECR, CloudWatch Logs, SSM.

## 17. GitHub Actions Deploy

Workflow deploy nằm ở:

```text
.github/workflows/video-call-deploy.yml
```

Trong GitHub repo, vào Settings -> Secrets and variables -> Actions, tạo:

```text
AWS_ACCESS_KEY_ID
AWS_SECRET_ACCESS_KEY
```

User/role của key này cần quyền:

- Login/push ECR.
- Register task definition.
- Update ECS service.
- Pass role `ecsTaskExecutionRole`.

Khi push vào branch `main`, workflow sẽ:

1. Build backend image từ `video-call/backend`.
2. Push image lên ECR `videocall-backend`.
3. Build frontend image từ `video-call/frontend`.
4. Push image lên ECR `videocall-frontend`.
5. Render task definition với image mới.
6. Deploy ECS service.

Có thể chạy manual từ tab GitHub Actions vì workflow có `workflow_dispatch`.

## 18. HTTPS Và WSS Hoạt Động Như Thế Nào

Production request flow:

```text
https://call.example.com
  -> ALB HTTPS 443
  -> frontend container port 80
  -> Nginx serve React
```

API:

```text
Browser: https://call.example.com/api/auth/login
  -> ALB HTTPS 443
  -> frontend Nginx /api
  -> backend:8080
```

WebSocket:

```text
Browser: wss://call.example.com/ws?token=...
  -> ALB HTTPS 443
  -> frontend Nginx /ws with Upgrade headers
  -> backend:8080/ws
```

ALB terminate TLS ở ngoài. Bên trong ECS, Nginx proxy sang backend bằng HTTP nội bộ là được.

Không cần mở port 8080 public.

## 19. TURN Cho WebRTC

WebSocket chỉ dùng để signaling. Video/audio thật sự đi qua WebRTC peer connection.

STUN giúp peer tìm public address, nhưng không đủ cho mọi network. TURN làm relay khi P2P không kết nối được.

Nên dùng TURN nếu app dùng thật:

- EC2 chạy `coturn`, hoặc
- Managed TURN provider.

Thông tin TURN được inject vào frontend container từ SSM qua:

```text
TURN_URLS
TURN_USERNAME
TURN_CREDENTIAL
```

Frontend container generate file runtime:

```text
/usr/share/nginx/html/env-config.js
```

React app đọc file này khi browser load page.

## 20. SQLite Persistence

Hiện backend dùng SQLite:

```text
DB_PATH=/app/videocall.db
```

Trong Fargate, filesystem của task không bền vững. Nếu task restart hoặc redeploy, data có thể mất.

Cho demo: có thể chấp nhận.

Cho production:

- Dùng EFS mount vào `/app`, hoặc
- Chuyển database sang RDS PostgreSQL/MySQL.

Khuyến nghị production: dùng RDS.

## 21. Checklist Sau Khi Deploy

Kiểm tra ECS:

- Service `videocall-service` stable.
- Có 1 running task.
- Target group health là healthy.
- CloudWatch logs không có lỗi.

Kiểm tra browser:

- Mở `https://call.example.com`.
- Đăng ký/đăng nhập được.
- DevTools Network thấy API gọi `https://...`.
- WebSocket kết nối `wss://.../ws?token=...`.
- Browser không báo lỗi mixed content.
- Browser hỏi permission camera/mic.
- Hai account online thấy nhau trong user list.
- Gọi video trên 2 thiết bị khác nhau.

Nếu local dev:

- Frontend Vite vẫn dùng `http://localhost:5173`.
- Backend vẫn dùng `http://localhost:8080`.
- WebSocket local vẫn dùng `ws://localhost:8080/ws`.

## 22. Lỗi Thường Gặp

### Camera/mic không hiện permission

Nguyên nhân hay gặp:

- Đang mở app bằng HTTP public, không phải HTTPS.
- Certificate sai hoặc domain không trusted.

### WebSocket fail

Kiểm tra:

- Browser đang connect `wss://your-domain/ws`.
- ALB listener HTTPS forward đúng target group.
- Nginx có proxy `/ws` với Upgrade headers.
- Backend logs có request `/ws`.

### API bị mixed content

Nếu page là HTTPS mà API gọi HTTP, browser sẽ block. Code hiện tại đã tránh lỗi này bằng same-origin config. Kiểm tra `/env-config.js` nếu bạn override `API_URL`.

### Call được nhưng không có video/audio remote

Kiểm tra:

- STUN/TURN config.
- ICE connection state trong browser console.
- Network có chặn UDP không.
- Thử thêm TURN relay.

### ECS task không start

Kiểm tra:

- Image ECR có tồn tại.
- `ecsTaskExecutionRole` có quyền pull ECR và đọc SSM.
- SSM parameter ARN đúng region/account.
- CloudWatch log group `/ecs/videocall` đã tồn tại.

## 23. Thứ Tự Làm Nhanh

Làm theo thứ tự này để ít lỗi:

1. Tạo ECR repositories.
2. Tạo CloudWatch log group.
3. Tạo SSM parameters.
4. Sửa `<ACCOUNT_ID>` và region trong `video-call/.aws/task-definition.json`.
5. Tạo/kiểm tra `ecsTaskExecutionRole`.
6. Tạo ECS cluster.
7. Tạo ACM certificate.
8. Tạo ALB security group và ECS security group.
9. Tạo target group.
10. Tạo ALB với HTTPS 443 và redirect HTTP 80.
11. Trỏ domain về ALB.
12. Push code lên GitHub `main` để workflow build/push image.
13. Tạo ECS service gắn với target group.
14. Mở `https://your-domain` và test `wss://.../ws`.
