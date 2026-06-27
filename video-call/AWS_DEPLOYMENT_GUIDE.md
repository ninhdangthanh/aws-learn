# Hướng Dẫn Deploy Video Call Lên AWS ECS Với CloudFront HTTPS/WSS

Tài liệu này hướng dẫn deploy app `video-call/` lên AWS ECS Fargate, dùng **CloudFront default domain** để có HTTPS/WSS mà chưa cần Route 53, domain riêng, hoặc ACM certificate riêng.

Kiến trúc trong guide này:

```text
Browser
  |
  | HTTPS / WSS
  v
CloudFront default domain
https://dxxxxx.cloudfront.net
  |
  | HTTP origin
  v
Application Load Balancer
  |
  | HTTP :80
  v
ECS Fargate Service
  |
  v
ECS Task: videocall-task
  |-- frontend container :80
  |     |-- serve React app
  |     |-- proxy /api -> backend:8080
  |     |-- proxy /ws  -> backend:8080
  |
  |-- backend container :8080
        |-- REST API
        |-- WebSocket signaling
        |-- SQLite database
```

Kết quả sau khi deploy:

- Web UI: `https://dxxxxx.cloudfront.net`
- API: `https://dxxxxx.cloudfront.net/api/...`
- WebSocket signaling: `wss://dxxxxx.cloudfront.net/ws`
- WebRTC media: P2P nếu kết nối được, hoặc relay qua TURN nếu có cấu hình TURN.

## 1. Vì Sao Dùng CloudFront Thay Vì Route 53 Lúc Này

**Step này để làm gì:** giúp bạn hiểu vì sao có thể bỏ qua Route 53/domain/ACM trong giai đoạn đầu.

Browser chỉ cho phép camera/microphone trên secure context:

- `localhost`, hoặc
- website có HTTPS hợp lệ.

Nếu dùng ALB default DNS trực tiếp:

```text
http://your-alb.ap-southeast-1.elb.amazonaws.com
```

thì chưa có HTTPS. Bạn cũng không thể cấp ACM public certificate cho domain `*.elb.amazonaws.com`, vì domain đó thuộc AWS, không thuộc bạn.

CloudFront giải quyết chuyện này bằng default domain:

```text
https://dxxxxx.cloudfront.net
```

Với default domain của CloudFront, AWS cung cấp sẵn SSL/TLS certificate. Vì vậy bạn có thể có HTTPS/WSS ngay mà chưa cần mua domain.

Lưu ý:

- Route 53 không bắt buộc.
- ACM certificate riêng không bắt buộc nếu dùng domain mặc định của CloudFront.
- Domain riêng có thể thêm sau.
- Đây là hướng rất phù hợp để demo/test production-like trước.

## 2. HTTPS Và WSS Hoạt Động Như Thế Nào

**Step này để làm gì:** phân biệt rõ các lớp HTTPS, WSS, API, WebSocket signaling và WebRTC media.

Frontend page:

```text
Browser
  -> https://dxxxxx.cloudfront.net
  -> CloudFront
  -> ALB HTTP :80
  -> frontend container :80
```

API:

```text
Browser
  -> https://dxxxxx.cloudfront.net/api/auth/login
  -> CloudFront
  -> ALB
  -> frontend Nginx /api
  -> backend:8080
```

WebSocket signaling:

```text
Browser
  -> wss://dxxxxx.cloudfront.net/ws?token=...
  -> CloudFront
  -> ALB
  -> frontend Nginx /ws
  -> backend:8080/ws
```

WebRTC media:

```text
Browser A <-> Browser B
```

hoặc nếu P2P fail:

```text
Browser A <-> TURN server <-> Browser B
```

CloudFront xử lý HTTPS/WSS phía browser. Từ CloudFront về ALB, guide này dùng HTTP để đơn giản và không cần ACM trên ALB.

## 3. Những Thứ Cần Có

**Step này để làm gì:** xác định tài nguyên cần tạo, và xác nhận không còn phụ thuộc Route 53/domain riêng.

Cần có:

- AWS account.
- AWS CLI đã configure.
- GitHub repo chứa code.
- 2 ECR repositories để chứa Docker images.
- CloudWatch log group.
- SSM Parameter Store để lưu secret.
- ECS cluster.
- Application Load Balancer.
- ECS service.
- CloudFront distribution.

Chưa cần:

- Route 53.
- Domain riêng.
- ACM certificate riêng.

Nên có sau nếu muốn video call ổn định:

- TURN server hoặc TURN provider.

## 4. Chọn Region Và Tên Resource

**Step này để làm gì:** thống nhất region và tên resource để tránh workflow build ở một region nhưng ECS/SSM/logs lại nằm ở region khác.

Guide này dùng AWS Asia Pacific (Singapore):

```bash
AWS_REGION=ap-southeast-1
ACCOUNT_ID=123456789012
```

Các resource mặc định:

```text
ECR backend repo:  videocall-backend
ECR frontend repo: videocall-frontend
ECS cluster:       videocall-cluster
ECS service:       videocall-service
Task family:       videocall-task
Log group:         /ecs/videocall
Target group:      videocall-frontend-tg
```

Nếu dùng region khác `ap-southeast-1`, sửa đồng bộ ở:

- `video-call/.aws/task-definition.json`
- `.github/workflows/video-call-deploy.yml`
- Các câu lệnh AWS CLI trong guide này.

CloudFront là dịch vụ global, nhưng origin ALB/ECS/ECR/SSM vẫn nằm trong region `ap-southeast-1`.

## 5. Tạo ECR Repositories

**Step này để làm gì:** GitHub Actions cần nơi push Docker images, ECS sẽ pull image từ đó để chạy container.

Tạo 2 repositories:

```bash
aws ecr create-repository \
  --repository-name videocall-backend \
  --region ap-southeast-1

aws ecr create-repository \
  --repository-name videocall-frontend \
  --region ap-southeast-1
```

Sau khi tạo, image URI sẽ có dạng:

```text
<ACCOUNT_ID>.dkr.ecr.ap-southeast-1.amazonaws.com/videocall-backend
<ACCOUNT_ID>.dkr.ecr.ap-southeast-1.amazonaws.com/videocall-frontend
```

Nếu repository đã tồn tại, có thể bỏ qua lỗi `RepositoryAlreadyExistsException`.

## 6. Tạo CloudWatch Log Group

**Step này để làm gì:** khi ECS task fail hoặc app lỗi, CloudWatch Logs là nơi debug chính.

Tạo log group:

```bash
aws logs create-log-group \
  --log-group-name /ecs/videocall \
  --region ap-southeast-1
```

Nếu log group đã tồn tại, bỏ qua lỗi `ResourceAlreadyExistsException`.

## 7. Tạo Runtime Secrets Trong SSM Parameter Store

**Step này để làm gì:** lưu secret ngoài code và ngoài GitHub. ECS sẽ inject secret vào container khi start task.

Task definition đang đọc các parameters:

```text
/videocall/JWT_SECRET
/videocall/TURN_URLS
/videocall/TURN_USERNAME
/videocall/TURN_CREDENTIAL
```

Tạo JWT secret:

```bash
aws ssm put-parameter \
  --name /videocall/JWT_SECRET \
  --type SecureString \
  --value "replace-with-a-long-random-secret" \
  --region ap-southeast-1
```

Nếu chưa có TURN, tạo placeholder bằng một dấu cách để ECS task vẫn start được:

```bash
aws ssm put-parameter \
  --name /videocall/TURN_URLS \
  --type SecureString \
  --value " " \
  --region ap-southeast-1

aws ssm put-parameter \
  --name /videocall/TURN_USERNAME \
  --type SecureString \
  --value " " \
  --region ap-southeast-1

aws ssm put-parameter \
  --name /videocall/TURN_CREDENTIAL \
  --type SecureString \
  --value " " \
  --region ap-southeast-1
```

Sau này khi có TURN thật, cập nhật lại bằng `--overwrite`.

Ví dụ:

```bash
aws ssm put-parameter \
  --name /videocall/TURN_URLS \
  --type SecureString \
  --value "turn:turn.example.com:3478,turns:turn.example.com:5349" \
  --overwrite \
  --region ap-southeast-1
```

## 8. Tạo IAM Role Cho ECS Task Execution

**Step này để làm gì:** ECS cần quyền pull image từ ECR, ghi logs vào CloudWatch, và đọc secret từ SSM.

Kiểm tra role `ecsTaskExecutionRole`:

1. Vào **IAM**.
2. Chọn **Roles**.
3. Tìm `ecsTaskExecutionRole`.

Nếu không thấy role này thì tự tạo mới. Đây là chuyện bình thường, không phải lỗi.

### Cách tạo bằng AWS Console

1. Vào **IAM** -> **Roles**.
2. Chọn **Create role**.
3. Trusted entity type: chọn **AWS service**.
4. Use case: chọn **Elastic Container Service**.
5. Chọn use case **Elastic Container Service Task**.
6. Bấm **Next**.
7. Attach policy:

```text
AmazonECSTaskExecutionRolePolicy
```

8. Role name:

```text
ecsTaskExecutionRole
```

9. Bấm **Create role**.

Sau khi tạo xong, mở role vừa tạo và thêm inline policy đọc SSM ở phần bên dưới.

### Cách tạo bằng AWS CLI

Tạo trust policy file tên `ecs-task-execution-trust-policy.json`:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Service": "ecs-tasks.amazonaws.com"
      },
      "Action": "sts:AssumeRole"
    }
  ]
}
```

Tạo role:

```bash
aws iam create-role \
  --role-name ecsTaskExecutionRole \
  --assume-role-policy-document file://ecs-task-execution-trust-policy.json
```

Attach AWS managed policy:

```bash
aws iam attach-role-policy \
  --role-name ecsTaskExecutionRole \
  --policy-arn arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy
```

Tóm lại, role này cần AWS managed policy:

```text
AmazonECSTaskExecutionRolePolicy
```

Thêm inline policy để đọc SSM parameters:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": ["ssm:GetParameters"],
      "Resource": "arn:aws:ssm:ap-southeast-1:<ACCOUNT_ID>:parameter/videocall/*"
    }
  ]
}
```

Nếu SSM SecureString dùng customer managed KMS key, thêm quyền `kms:Decrypt` cho key đó.

Sau đó lấy ARN của role, ví dụ:

```text
arn:aws:iam::<ACCOUNT_ID>:role/ecsTaskExecutionRole
```

## 9. Sửa Task Definition

**Step này để làm gì:** task definition là bản thiết kế để ECS biết chạy container nào, port nào, log ở đâu, secret nào.

Mở file:

```text
video-call/.aws/task-definition.json
```

Cần thay:

- `<ACCOUNT_ID>` bằng AWS account ID của bạn.
- `executionRoleArn` bằng ARN của `ecsTaskExecutionRole`.
- Region trong SSM ARN và `awslogs-region` phải là `ap-southeast-1`.

Các chỗ thường cần sửa:

```json
"valueFrom": "arn:aws:ssm:ap-southeast-1:<ACCOUNT_ID>:parameter/videocall/JWT_SECRET"
```

```json
"executionRoleArn": "arn:aws:iam::<ACCOUNT_ID>:role/ecsTaskExecutionRole"
```

Lưu ý về `API_URL` và `WS_URL`:

- Production nên để rỗng.
- App sẽ tự dùng domain hiện tại của CloudFront.
- Khi page là `https://dxxxxx.cloudfront.net`, app sẽ tự gọi API qua HTTPS và WebSocket qua WSS.

## 10. Tạo ECS Cluster

**Step này để làm gì:** cluster là nơi ECS quản lý service và task. Với Fargate, bạn không cần tự tạo EC2 instance cho app.

Tạo cluster:

```bash
aws ecs create-cluster \
  --cluster-name videocall-cluster \
  --region ap-southeast-1
```

Nếu tạo bằng Console:

1. Vào **ECS**.
2. Chọn **Clusters**.
3. Chọn **Create cluster**.
4. Đặt tên `videocall-cluster`.
5. Chọn Fargate/serverless nếu console hỏi infrastructure.

## 11. Tạo Security Groups

**Step này để làm gì:** mở đúng port cần thiết, nhưng không public backend 8080 ra internet.

Cần 2 security groups.

### ALB Security Group

Inbound:

- TCP 80 từ `0.0.0.0/0`

Outbound:

- Cho phép ra ECS service security group port 80.

Lý do ALB chỉ cần HTTP 80: CloudFront đã xử lý HTTPS/WSS phía browser. CloudFront gọi ALB qua HTTP origin.

Ghi chú bảo mật:

- Cấu hình đơn giản nhất là mở ALB port 80 public.
- Sau khi chạy ổn, bạn có thể siết lại để chỉ cho CloudFront vào ALB bằng AWS managed prefix list cho CloudFront origin-facing hoặc bằng custom header/WAF.

### ECS Service Security Group

Inbound:

- TCP 80 từ ALB security group.

Outbound:

- Cho phép outbound để task pull image, ghi logs, đọc SSM qua AWS public endpoints.

Không cần mở port 8080 public. Backend chỉ nhận traffic nội bộ từ frontend Nginx trong cùng task.

## 12. Tạo Target Group Cho Frontend

**Step này để làm gì:** ALB cần target group để biết gửi request vào ECS task nào.

Tạo target group:

- Target type: `IP`
- Protocol: `HTTP`
- Port: `80`
- VPC: VPC bạn sẽ chạy ECS
- Health check path: `/`
- Name: `videocall-frontend-tg`

Vì ECS Fargate dùng network mode `awsvpc`, target type phải là `IP`, không phải `instance`.

## 13. Tạo Application Load Balancer

**Step này để làm gì:** ALB là origin phía sau CloudFront và route traffic vào ECS frontend container.

Tạo ALB:

- Scheme: internet-facing
- IP address type: IPv4
- VPC: cùng VPC với ECS service
- Subnets: chọn ít nhất 2 public subnets
- Security group: ALB security group

Listener:

- HTTP 80

Rule:

- Forward sang target group `videocall-frontend-tg`.

Không cần listener HTTPS 443 trên ALB trong phương án CloudFront default domain. HTTPS/WSS nằm ở CloudFront.

Sau khi tạo ALB, lưu DNS name, ví dụ:

```text
videocall-alb-123456.ap-southeast-1.elb.amazonaws.com
```

DNS name này sẽ dùng làm **origin domain** khi tạo CloudFront distribution.

## 14. GitHub Actions CI/CD Build Và Push Image Lên ECR

**Step này để làm gì:** tự động build Docker image, push lên ECR, cập nhật ECS service mỗi khi push code lên `main`.

Workflow chính nằm ở:

```text
.github/workflows/video-call-deploy.yml
```

Workflow này làm:

1. Checkout code.
2. Login AWS bằng GitHub secrets.
3. Login ECR.
4. Build backend image từ `video-call/backend`.
5. Push backend image lên `videocall-backend`.
6. Build frontend image từ `video-call/frontend`.
7. Push frontend image lên `videocall-frontend`.
8. Render task definition với image tag mới.
9. Deploy ECS service.

Trong GitHub repo, tạo secrets:

```text
AWS_ACCESS_KEY_ID
AWS_SECRET_ACCESS_KEY
```

IAM user/role của key này cần quyền:

- ECR login/push.
- ECS register task definition.
- ECS update service.
- IAM pass role cho `ecsTaskExecutionRole`.

Workflow chạy khi:

- Push vào branch `main` và có thay đổi trong `video-call/**`.
- Hoặc chạy manual bằng `workflow_dispatch` trong tab GitHub Actions.

## 15. Push Image Lần Đầu

**Step này để làm gì:** ECS service cần image thật trong ECR. Nếu task definition còn placeholder `<IMAGE_BE>` và `<IMAGE_FE>`, ECS không thể chạy task thật.

Cách khuyến nghị:

1. Commit code.
2. Push lên GitHub branch `main`.
3. Vào tab **Actions**.
4. Chạy workflow `Deploy Video Call to Amazon ECS`.
5. Kiểm tra 2 ECR repos đã có image tag là commit SHA.

Nếu workflow fail vì ECS service chưa tồn tại, vẫn có thể image đã được push lên ECR. Bạn có thể tạo ECS service sau đó chạy lại workflow.

## 16. Register Task Definition Và Tạo ECS Service

**Step này để làm gì:** tạo service chạy lâu dài trên ECS, gắn service vào ALB target group.

Nếu task definition đã có image ECR thật, register task:

```bash
aws ecs register-task-definition \
  --cli-input-json file://video-call/.aws/task-definition.json \
  --region ap-southeast-1
```

Tạo ECS service trong Console:

1. Vào **ECS** -> **Clusters** -> `videocall-cluster`.
2. Chọn **Create service**.
3. Launch type: Fargate.
4. Task definition family: `videocall-task`.
5. Service name: `videocall-service`.
6. Desired tasks: `1`.
7. Networking:
   - Chọn VPC.
   - Chọn subnets.
   - Chọn ECS service security group.
8. Load balancing:
   - Load balancer type: Application Load Balancer.
   - Container: `frontend`.
   - Container port: `80`.
   - Target group: `videocall-frontend-tg`.

Subnet nên chọn thế nào:

- Dễ nhất lúc demo: public subnets và bật public IP cho task.
- Đúng hơn cho production: private subnets, task ra internet qua NAT Gateway hoặc VPC endpoints.

Sau khi tạo service, kiểm tra:

- ECS task chuyển sang `RUNNING`.
- Target group thấy target `healthy`.
- CloudWatch logs không có lỗi start container.

Trước khi tạo CloudFront, thử mở ALB DNS bằng HTTP:

```text
http://videocall-alb-123456.ap-southeast-1.elb.amazonaws.com
```

Nếu thấy app load được qua HTTP, origin đã ổn.

## 17. Tạo CloudFront Distribution

**Step này để làm gì:** CloudFront tạo HTTPS/WSS endpoint public bằng default domain `*.cloudfront.net`, không cần domain riêng.

Vào **CloudFront** -> **Create distribution**.

### Origin

Origin domain:

```text
videocall-alb-123456.ap-southeast-1.elb.amazonaws.com
```

Không nhập `http://`, chỉ nhập DNS name.

Origin protocol policy:

```text
HTTP only
```

HTTP port:

```text
80
```

### Default Cache Behavior

Để đơn giản cho lần deploy đầu, dùng một behavior cho toàn bộ path:

```text
Path pattern: Default (*)
Viewer protocol policy: Redirect HTTP to HTTPS
Allowed HTTP methods: GET, HEAD, OPTIONS, PUT, POST, PATCH, DELETE
Cache policy: CachingDisabled
Origin request policy: AllViewer
Compress objects automatically: Yes
```

Vì sao chọn cấu hình này:

- `Redirect HTTP to HTTPS`: browser luôn vào secure context.
- Allowed methods all: API login/register dùng POST.
- `CachingDisabled`: tránh cache nhầm API/WebSocket trong giai đoạn đầu.
- `AllViewer`: forward headers/query string/cookies, giúp API Authorization và WebSocket token hoạt động.

Sau này tối ưu performance, bạn có thể tách behavior:

- `/api/*`: caching disabled.
- `/ws*`: caching disabled, forward WebSocket headers.
- `/*`: cache static assets.

Nhưng giai đoạn đầu cứ dùng một behavior đơn giản để giảm lỗi cấu hình.

### Distribution Settings

Alternate domain name:

```text
Để trống
```

Custom SSL certificate:

```text
Default CloudFront certificate
```

Supported HTTP versions:

```text
HTTP/2, HTTP/1.1
```

WebSocket qua CloudFront hiện dùng HTTP/1.1 cho connection upgrade, CloudFront sẽ xử lý phần này nếu behavior forward đúng headers.

Tạo distribution và chờ status chuyển sang `Deployed`. Có thể mất vài phút.

Sau khi deployed, CloudFront sẽ có domain dạng:

```text
dxxxxx.cloudfront.net
```

Mở thử:

```text
https://dxxxxx.cloudfront.net
```

## 18. Kiểm Tra HTTPS, API Và WSS Qua CloudFront

**Step này để làm gì:** xác nhận CloudFront đã thay thế vai trò domain/HTTPS cho app.

Kiểm tra frontend:

```text
https://dxxxxx.cloudfront.net
```

Kiểm tra API trong DevTools:

```text
https://dxxxxx.cloudfront.net/api/auth/login
```

Kiểm tra WebSocket trong DevTools -> Network -> WS:

```text
wss://dxxxxx.cloudfront.net/ws?token=...
```

Kỳ vọng:

- Browser không báo mixed content.
- Browser hỏi quyền camera/mic.
- API không bị cache nhầm.
- WebSocket status `101 Switching Protocols`.
- User online list cập nhật được.

Nếu page load được nhưng API fail:

- Kiểm tra CloudFront allowed methods đã chọn all methods chưa.
- Kiểm tra cache policy có phải `CachingDisabled` không.
- Kiểm tra origin request policy có forward headers/query strings không.

Nếu WebSocket fail:

- Kiểm tra URL là `wss://`, không phải `ws://`.
- Kiểm tra CloudFront origin request policy có forward viewer headers không.
- Kiểm tra Nginx `/ws` vẫn giữ `Upgrade` và `Connection`.
- Kiểm tra backend logs có request `/ws` không.

## 19. TURN Cho WebRTC

**Step này để làm gì:** WebSocket chỉ dùng để signaling. Video/audio thật sự đi qua WebRTC, và nhiều network cần TURN để call ổn định.

STUN:

- Giúp browser tìm public IP/port.
- Không relay media.
- Có thể đủ khi mạng đơn giản.

TURN:

- Relay media qua server trung gian.
- Cần khi gặp symmetric NAT, firewall công ty, một số mạng 4G/5G.
- Tốn bandwidth server vì video/audio đi qua TURN.

Bạn có thể deploy thử trước mà chưa có TURN:

- Giữ 3 SSM TURN parameters là một dấu cách `" "`.
- App vẫn chạy.
- Một số cuộc gọi có thể thành công nếu mạng dễ.
- Một số cuộc gọi sẽ không có remote video/audio nếu NAT khó.

Nếu dùng managed TURN provider, cập nhật SSM:

```bash
aws ssm put-parameter \
  --name /videocall/TURN_URLS \
  --type SecureString \
  --value "turn:your-turn-host:3478,turns:your-turn-host:5349" \
  --overwrite \
  --region ap-southeast-1

aws ssm put-parameter \
  --name /videocall/TURN_USERNAME \
  --type SecureString \
  --value "your-turn-username" \
  --overwrite \
  --region ap-southeast-1

aws ssm put-parameter \
  --name /videocall/TURN_CREDENTIAL \
  --type SecureString \
  --value "your-turn-password" \
  --overwrite \
  --region ap-southeast-1
```

Sau đó redeploy ECS service để frontend container regenerate `/env-config.js`.

Nếu tự chạy `coturn` trên EC2:

- Gắn Elastic IP.
- Tạo DNS cho TURN nếu có domain, ví dụ `turn.example.com`.
- Mở TCP/UDP 3478.
- Mở TCP 5349 nếu dùng TLS TURN.
- Mở UDP relay port range, ví dụ `49152-65535`.
- Cấu hình user/password và realm.
- Theo dõi bandwidth vì media relay có thể tốn nhiều traffic.

## 20. SQLite Persistence

**Step này để làm gì:** tránh hiểu nhầm rằng deploy ECS xong là data đã bền vững.

Hiện backend dùng SQLite:

```text
DB_PATH=/app/videocall.db
```

Trong Fargate, filesystem của task không bền vững. Nếu task restart hoặc redeploy, data có thể mất.

Cho demo:

- Có thể chấp nhận SQLite trong task.

Cho production:

- Nên chuyển sang RDS PostgreSQL/MySQL.
- Hoặc tạm thời mount EFS vào `/app` nếu vẫn muốn SQLite, nhưng SQLite trên network filesystem cần cân nhắc kỹ về locking và concurrency.

Khuyến nghị: nếu app dùng thật, chuyển database sang RDS.

## 21. Checklist Sau Khi Deploy

**Step này để làm gì:** kiểm tra theo thứ tự để biết lỗi nằm ở CloudFront, ALB, ECS, API, WebSocket hay WebRTC.

Kiểm tra ECS/ALB:

- ECS service `videocall-service` stable.
- Có 1 running task.
- Target group có healthy target.
- ALB DNS mở được bằng HTTP.
- CloudWatch logs không có lỗi.

Kiểm tra CloudFront:

- Distribution status là `Deployed`.
- `https://dxxxxx.cloudfront.net` mở được.
- HTTP tự redirect sang HTTPS.
- Default certificate của CloudFront hoạt động.

Kiểm tra browser:

- Đăng ký/đăng nhập được.
- DevTools Network thấy API gọi `https://dxxxxx.cloudfront.net/api/...`.
- WebSocket kết nối `wss://dxxxxx.cloudfront.net/ws?token=...`.
- Browser không báo mixed content.
- Browser hỏi permission camera/mic.
- 2 account thấy nhau online.
- Gọi video trên 2 thiết bị khác nhau.

Kiểm tra WebRTC:

- Console không báo ICE failed.
- Nếu mạng khó, TURN server có traffic.
- Nếu không có TURN, thử lại trên cùng WiFi hoặc mạng đơn giản hơn để phân biệt lỗi app và lỗi NAT.

## 22. Lỗi Thường Gặp

**Step này để làm gì:** có bảng debug nhanh khi deploy không chạy ngay lần đầu.

### Camera/mic không hiện permission

Nguyên nhân thường gặp:

- Bạn đang mở ALB HTTP trực tiếp thay vì CloudFront HTTPS.
- URL không phải `https://dxxxxx.cloudfront.net`.

Cách kiểm tra:

- Mở đúng CloudFront domain.
- Browser phải hiển thị connection secure.

### API login/register fail

Nguyên nhân thường gặp:

- CloudFront chỉ cho GET/HEAD, chưa allow POST.
- CloudFront đang cache API.
- Authorization header không được forward.

Cách kiểm tra:

- Allowed methods phải là all methods.
- Cache policy nên là `CachingDisabled`.
- Origin request policy nên là `AllViewer` cho lần deploy đầu.

### WebSocket fail

Nguyên nhân thường gặp:

- Browser đang connect `ws://` thay vì `wss://`.
- CloudFront behavior không forward WebSocket headers/query string.
- Nginx không proxy `/ws`.
- Backend không nhận request `/ws`.

Cách kiểm tra:

- DevTools Network -> WS.
- URL phải là `wss://dxxxxx.cloudfront.net/ws?token=...`.
- Response kỳ vọng là `101 Switching Protocols`.
- CloudWatch backend logs.

### Call được nhưng không có remote video/audio

Nguyên nhân thường gặp:

- ICE failed.
- STUN không đủ.
- Thiếu TURN.
- Firewall chặn UDP.

Cách kiểm tra:

- Browser console log ICE state.
- Thử mạng khác.
- Cấu hình TURN rồi redeploy.

### ECS task không start

Nguyên nhân thường gặp:

- Image chưa tồn tại trong ECR.
- `ecsTaskExecutionRole` thiếu quyền pull ECR.
- Role thiếu quyền đọc SSM.
- SSM parameter sai region/account.
- CloudWatch log group chưa tồn tại.

## 23. Thêm Domain Riêng Sau Này

**Step này để làm gì:** khi demo ổn, bạn có thể đổi từ `dxxxxx.cloudfront.net` sang domain đẹp hơn.

Sau này nếu có domain riêng, ví dụ:

```text
call.example.com
```

bạn có thể:

1. Tạo ACM certificate cho `call.example.com` ở region `us-east-1`.
2. Validate certificate bằng DNS ở provider bạn đang dùng.
3. Thêm alternate domain name `call.example.com` vào CloudFront distribution.
4. Chọn ACM certificate đó trong CloudFront.
5. Tạo CNAME:

```text
call.example.com -> dxxxxx.cloudfront.net
```

Lưu ý: với CloudFront custom domain, ACM certificate phải nằm ở `us-east-1`, dù ECS/ALB đang ở `ap-southeast-1`.

## 24. Thứ Tự Làm Nhanh Từ Con Số 0

**Step này để làm gì:** checklist tổng hợp nếu bạn muốn làm theo một mạch.

1. Chọn region `ap-southeast-1`.
2. Tạo ECR repositories.
3. Tạo CloudWatch log group.
4. Tạo SSM parameters cho JWT và TURN placeholder.
5. Tạo hoặc cập nhật `ecsTaskExecutionRole`.
6. Sửa `<ACCOUNT_ID>` trong `video-call/.aws/task-definition.json`.
7. Tạo ECS cluster.
8. Tạo ALB security group và ECS service security group.
9. Tạo target group `videocall-frontend-tg`.
10. Tạo ALB HTTP 80.
11. Push code lên GitHub `main` để workflow build/push image.
12. Register task definition nếu cần.
13. Tạo ECS service gắn với target group.
14. Kiểm tra ALB HTTP load được app.
15. Tạo CloudFront distribution trỏ origin về ALB DNS.
16. Chờ CloudFront status `Deployed`.
17. Mở `https://dxxxxx.cloudfront.net`.
18. Kiểm tra API qua HTTPS.
19. Kiểm tra WebSocket qua WSS.
20. Test video call.
21. Nếu video không ổn định, cấu hình TURN thật.
22. Sau này nếu muốn, thêm domain riêng vào CloudFront.

## 25. Tài Liệu Tham Khảo

- [CloudFront - Require HTTPS for viewers](https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/using-https-viewers-to-cloudfront.html)
- [CloudFront - Use WebSockets with distributions](https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/distribution-working-with.websockets.html)
- [Route 53 - Registering a new domain](https://docs.aws.amazon.com/Route53/latest/DeveloperGuide/domain-register.html)
- [AWS Certificate Manager - DNS validation](https://docs.aws.amazon.com/acm/latest/userguide/dns-validation.html)
