# Hướng Dẫn Deploy Video Call Lên AWS ECS Với Route 53 Và ALB HTTPS/WSS

Tài liệu này hướng dẫn deploy app `video-call/` lên AWS ECS Fargate, dùng **Route 53 + AWS Certificate Manager + Application Load Balancer HTTPS** để public app bằng domain riêng.

Kiến trúc trong guide này:

```text
Browser
  |
  | HTTPS / WSS
  v
Route 53 domain
https://call.example.com
  |
  | Alias A/AAAA
  v
Application Load Balancer
  |-- HTTPS :443, terminate TLS bằng ACM certificate
  |-- HTTP  :80, redirect sang HTTPS :443
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

- Web UI: `https://call.example.com`
- API: `https://call.example.com/api/...`
- WebSocket signaling: `wss://call.example.com/ws`
- WebRTC media: P2P nếu kết nối được, hoặc relay qua TURN nếu có cấu hình TURN.

## 1. Vì Sao Dùng Route 53 Và ALB HTTPS

**Step này để làm gì:** giúp bạn hiểu vì sao domain riêng phải đi với HTTPS nếu muốn video call hoạt động đúng.

Browser chỉ cho phép camera/microphone trên secure context:

- `localhost`, hoặc
- website có HTTPS hợp lệ.

Nếu mở ALB default DNS trực tiếp:

```text
http://your-alb.ap-southeast-1.elb.amazonaws.com
```

thì frontend có thể load, nhưng camera/microphone và WebSocket production sẽ không ổn vì URL không phải HTTPS/WSS.

Với hướng trong guide này:

```text
https://call.example.com
```

ALB sẽ nhận HTTPS ở port `443`, dùng ACM certificate cho domain của bạn, rồi forward HTTP nội bộ vào ECS frontend container port `80`.

Lưu ý:

- Không dùng CloudFront trong guide này.
- Route 53 quản lý DNS cho domain/subdomain.
- ACM certificate cho ALB phải tạo cùng region với ALB, ở đây là `ap-southeast-1`.
- Backend `8080` không public ra internet.

## 2. HTTPS Và WSS Hoạt Động Như Thế Nào

**Step này để làm gì:** phân biệt rõ các lớp HTTPS, WSS, API, WebSocket signaling và WebRTC media.

Frontend page:

```text
Browser
  -> https://call.example.com
  -> Route 53
  -> ALB HTTPS :443
  -> frontend container :80
```

API:

```text
Browser
  -> https://call.example.com/api/auth/login
  -> Route 53
  -> ALB HTTPS :443
  -> frontend Nginx /api
  -> backend:8080
```

WebSocket signaling:

```text
Browser
  -> wss://call.example.com/ws?token=...
  -> Route 53
  -> ALB HTTPS :443
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

ALB xử lý HTTPS/WSS phía browser. Từ ALB về ECS target group, guide này dùng HTTP port `80`.

## 3. Những Thứ Cần Có

**Step này để làm gì:** xác định tài nguyên cần tạo trước khi deploy.

Cần có:

- AWS account.
- AWS CLI đã configure.
- GitHub repo chứa code.
- VPC và subnets để đặt ALB/ECS.
- Security groups cho ALB và ECS service.
- Domain hoặc subdomain. Domain có thể mua ở Route 53 hoặc mua ngoài AWS như INET.
- Route 53 hosted zone để quản lý DNS records của domain.
- ACM certificate cho domain ở region `ap-southeast-1`.
- 2 ECR repositories để chứa Docker images.
- CloudWatch log group.
- SSM Parameter Store để lưu secret.
- ECS cluster.
- Application Load Balancer.
- Target group cho frontend.
- ECS service.

Nên có sau nếu muốn video call ổn định:

- TURN server hoặc TURN provider.

## 4. Chọn Region, Domain Và Tên Resource

**Step này để làm gì:** thống nhất region và tên resource để tránh workflow build ở một region nhưng ECS/SSM/logs/ALB lại nằm ở region khác.

Guide này dùng AWS Asia Pacific (Singapore):

```bash
AWS_REGION=ap-southeast-1
ACCOUNT_ID=123456789012
APP_DOMAIN=call.example.com
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
ALB:               videocall-alb
```

Nếu dùng region khác `ap-southeast-1`, sửa đồng bộ ở:

- `video-call/.aws/task-definition.json`
- `.github/workflows/video-call-deploy.yml`
- Các câu lệnh AWS CLI trong guide này.

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

## 8. Tạo IAM Role Cho ECS Task Execution

**Step này để làm gì:** ECS cần quyền pull image từ ECR, ghi logs vào CloudWatch, và đọc secret từ SSM.

Kiểm tra role `ecsTaskExecutionRole`:

1. Vào **IAM**.
2. Chọn **Roles**.
3. Tìm `ecsTaskExecutionRole`.

Nếu không thấy role này thì tự tạo mới.

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
- App sẽ tự dùng domain hiện tại của browser.
- Khi page là `https://call.example.com`, app sẽ tự gọi API qua HTTPS và WebSocket qua WSS.

Frontend container đang có:

```json
"BACKEND_URL": "http://localhost:8080"
```

Giá trị này đúng cho task hiện tại vì frontend và backend chạy trong cùng ECS task. Nginx của frontend proxy `/api` và `/ws` sang backend qua `localhost:8080`.

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

## 11. Tạo ACM Certificate Cho Domain

**Step này để làm gì:** ALB cần certificate hợp lệ để nhận HTTPS/WSS từ browser.

Vào **AWS Certificate Manager** ở region:

```text
ap-southeast-1
```

Tạo public certificate cho domain bạn muốn dùng, ví dụ:

```text
call.example.com
```

Nếu muốn dùng cả root domain và `www`, thêm cả hai:

```text
example.com
www.example.com
```

Chọn validation bằng DNS. ACM sẽ đưa các CNAME records để xác minh domain.

Nếu domain đang quản lý trong Route 53, ACM thường có nút:

```text
Create records in Route 53
```

Bấm nút đó để tạo DNS validation records tự động.

Chờ certificate chuyển sang:

```text
Issued
```

Quan trọng:

- Certificate cho ALB phải nằm cùng region với ALB, ở đây là `ap-southeast-1`.
- Không dùng certificate `us-east-1` trong hướng này. `us-east-1` chỉ bắt buộc khi dùng CloudFront.

## 12. Tạo Security Groups

**Step này để làm gì:** mở đúng port cần thiết, nhưng không public backend 8080 ra internet.

Cần 2 security groups.

### ALB Security Group

Inbound:

- TCP 80 từ `0.0.0.0/0`
- TCP 443 từ `0.0.0.0/0`

Outbound:

- Cho phép ra ECS service security group port 80.

Lý do:

- Port `443` nhận HTTPS/WSS thật từ browser.
- Port `80` chỉ dùng để redirect HTTP sang HTTPS.

### ECS Service Security Group

Inbound:

- TCP 80 từ ALB security group.

Outbound:

- Cho phép outbound để task pull image, ghi logs, đọc SSM qua AWS public endpoints.

Không cần mở port 8080 public. Backend chỉ nhận traffic nội bộ từ frontend Nginx trong cùng task.

## 13. Tạo Target Group Cho Frontend

**Step này để làm gì:** ALB cần target group để biết gửi request vào ECS task nào.

Tạo target group:

- Target type: `IP`
- Protocol: `HTTP`
- Port: `80`
- VPC: VPC bạn sẽ chạy ECS
- Health check path: `/`
- Name: `videocall-frontend-tg`

Vì ECS Fargate dùng network mode `awsvpc`, target type phải là `IP`, không phải `instance`.

## 14. Tạo Application Load Balancer

**Step này để làm gì:** ALB là entrypoint public HTTPS/WSS cho app và route traffic vào ECS frontend container.

Tạo ALB:

- Scheme: internet-facing
- IP address type: IPv4
- VPC: cùng VPC với ECS service
- Subnets: chọn ít nhất 2 public subnets
- Security group: ALB security group

Tạo listeners:

### Listener HTTPS 443

- Protocol: `HTTPS`
- Port: `443`
- Certificate: ACM certificate đã `Issued`
- Default action: forward sang target group `videocall-frontend-tg`

### Listener HTTP 80

Nên redirect toàn bộ HTTP sang HTTPS:

- Protocol: `HTTP`
- Port: `80`
- Default action: redirect to HTTPS `443`
- Status code: `HTTP_301`

Nếu muốn test nhanh trước khi certificate sẵn sàng, có thể tạm forward HTTP `80` sang target group. Sau khi HTTPS chạy, nên đổi lại thành redirect.

Sau khi tạo ALB, lưu DNS name, ví dụ:

```text
videocall-alb-123456.ap-southeast-1.elb.amazonaws.com
```

## 15. Tạo Route 53 Hosted Zone Và DNS Record

**Step này để làm gì:** trỏ domain của bạn vào ALB bằng DNS.

Route 53 không chỉ là nơi bán domain. Trong guide này cần phân biệt:

```text
Route 53 Domains      -> mua/gia hạn/chuyển domain
Route 53 Hosted Zones -> quản lý DNS records
Route 53 Records      -> A, A Alias, CNAME, MX, TXT...
```

Nếu domain mua ở ngoài AWS, ví dụ INET, bạn vẫn dùng được Route 53 Hosted Zone bình thường. Khác biệt chính là bạn phải vào trang quản lý domain ở INET để đổi nameserver sang 4 nameserver mà Route 53 cấp.

Nếu domain mua ở Route 53:

1. Vào **Route 53** -> **Hosted zones**.
2. Chọn hosted zone của domain.
3. Tạo record cho domain/subdomain.

Ví dụ dùng subdomain:

```text
call.example.com
```

Tạo record:

```text
Record name: call
Record type: A
Alias: Yes
Route traffic to: Alias to Application and Classic Load Balancer
Region: ap-southeast-1
Load balancer: videocall-alb-...
```

Nếu muốn hỗ trợ IPv6, tạo thêm record:

```text
Record type: AAAA
Alias: Yes
Route traffic to: cùng ALB
```

Nếu domain mua ở nơi khác nhưng muốn dùng Route 53 DNS:

1. Tạo public hosted zone trong Route 53 cho domain.
2. Copy 4 name servers của hosted zone.
3. Vào trang quản lý domain nơi bạn mua domain.
4. Đổi nameservers sang 4 name servers của Route 53.
5. Chờ DNS propagate.

Sau đó vẫn tạo Alias A/AAAA record về ALB như trên.

Với project `ninh-video-call-demo.food`:

```text
Domain mua ở: INET
DNS quản lý ở: Route 53 Hosted Zone
```

Vì vậy phần deploy AWS phía sau vẫn giống guide này: ACM, A Alias, ALB, target group và ECS không khác gì so với domain mua trực tiếp trong Route 53.

## 16. GitHub Actions CI/CD Build Và Push Image Lên ECR

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

## 17. Push Image Lần Đầu

**Step này để làm gì:** ECS service cần image thật trong ECR. Nếu task definition còn placeholder `<IMAGE_BE>` và `<IMAGE_FE>`, ECS không thể chạy task thật.

Cách khuyến nghị:

1. Commit code.
2. Push lên GitHub branch `main`.
3. Vào tab **Actions**.
4. Chạy workflow `Deploy Video Call to Amazon ECS`.
5. Kiểm tra 2 ECR repos đã có image tag là commit SHA.

Nếu workflow fail vì ECS service chưa tồn tại, vẫn có thể image đã được push lên ECR. Bạn có thể tạo ECS service sau đó chạy lại workflow.

## 18. Register Task Definition Và Tạo ECS Service

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

Trước khi test domain, thử mở ALB DNS bằng HTTP:

```text
http://videocall-alb-123456.ap-southeast-1.elb.amazonaws.com
```

Nếu listener `80` đã redirect, browser sẽ chuyển sang HTTPS. Khi dùng ALB DNS, HTTPS có thể báo certificate mismatch vì certificate cấp cho domain của bạn, không phải `*.elb.amazonaws.com`. Đây là bình thường. Test chính thức bằng domain Route 53.

## 19. Kiểm Tra HTTPS, API Và WSS Qua Domain

**Step này để làm gì:** xác nhận Route 53 + ALB HTTPS đã public app đúng cách.

Kiểm tra frontend:

```text
https://call.example.com
```

Kiểm tra API trong DevTools:

```text
https://call.example.com/api/auth/login
```

Kiểm tra WebSocket trong DevTools -> Network -> WS:

```text
wss://call.example.com/ws?token=...
```

Kỳ vọng:

- Browser không báo mixed content.
- Browser hỏi quyền camera/mic.
- Certificate hợp lệ, không báo insecure.
- API login/register hoạt động.
- WebSocket status `101 Switching Protocols`.
- User online list cập nhật được.

Nếu page load được nhưng API fail:

- Kiểm tra frontend Nginx có proxy `/api` sang `BACKEND_URL`.
- Kiểm tra `BACKEND_URL` trong task definition là `http://localhost:8080`.
- Kiểm tra backend logs có nhận request `/api/...` không.

Nếu WebSocket fail:

- Kiểm tra URL là `wss://`, không phải `ws://`.
- Kiểm tra ALB listener `443` forward tới frontend target group.
- Kiểm tra Nginx `/ws` vẫn giữ `Upgrade` và `Connection`.
- Kiểm tra backend logs có request `/ws` không.

## 20. TURN Cho WebRTC

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

## 21. SQLite Persistence

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

## 22. Checklist Sau Khi Deploy

**Step này để làm gì:** kiểm tra theo thứ tự để biết lỗi nằm ở DNS, ALB, ECS, API, WebSocket hay WebRTC.

Kiểm tra DNS/ACM/ALB:

- ACM certificate ở `ap-southeast-1` có trạng thái `Issued`.
- Route 53 Alias A record trỏ domain về ALB.
- ALB listener `443` forward sang target group.
- ALB listener `80` redirect sang HTTPS `443`.
- ALB security group mở inbound `80` và `443`.

Kiểm tra ECS/ALB:

- ECS service `videocall-service` stable.
- Có 1 running task.
- Target group có healthy target.
- CloudWatch logs không có lỗi.

Kiểm tra browser:

- Mở `https://call.example.com` được.
- Đăng ký/đăng nhập được.
- DevTools Network thấy API gọi `https://call.example.com/api/...`.
- WebSocket kết nối `wss://call.example.com/ws?token=...`.
- Browser không báo mixed content.
- Browser hỏi permission camera/mic.
- 2 account thấy nhau online.
- Gọi video trên 2 thiết bị khác nhau.

Kiểm tra WebRTC:

- Console không báo ICE failed.
- Nếu mạng khó, TURN server có traffic.
- Nếu không có TURN, thử lại trên cùng WiFi hoặc mạng đơn giản hơn để phân biệt lỗi app và lỗi NAT.

## 23. Lỗi Thường Gặp

**Step này để làm gì:** có bảng debug nhanh khi deploy không chạy ngay lần đầu.

### Camera/mic không hiện permission

Nguyên nhân thường gặp:

- Bạn đang mở ALB HTTP trực tiếp thay vì domain HTTPS.
- ACM certificate chưa hợp lệ.
- URL không phải `https://call.example.com`.

Cách kiểm tra:

- Mở đúng domain HTTPS.
- Browser phải hiển thị connection secure.
- ALB listener `443` phải dùng certificate đúng domain.

### Domain không trỏ vào ALB

Nguyên nhân thường gặp:

- Route 53 hosted zone chưa đúng domain.
- Domain mua ở nơi khác nhưng nameserver chưa đổi sang Route 53.
- Alias record chưa trỏ đúng ALB region.

Cách kiểm tra:

```bash
dig call.example.com
```

Kết quả phải resolve về ALB.

### HTTPS báo certificate invalid

Nguyên nhân thường gặp:

- Certificate tạo sai region.
- Certificate chưa `Issued`.
- Certificate không chứa đúng domain đang mở.
- Bạn đang mở ALB DNS HTTPS thay vì domain thật.

Cách kiểm tra:

- Certificate cho ALB phải ở `ap-southeast-1`.
- Subject Alternative Names phải có `call.example.com`.
- Test bằng `https://call.example.com`, không phải `https://*.elb.amazonaws.com`.

### API login/register fail

Nguyên nhân thường gặp:

- Nginx frontend không proxy `/api`.
- `BACKEND_URL` sai.
- Backend container không chạy hoặc không nghe port `8080`.
- Backend logs có lỗi database/secret.

Cách kiểm tra:

- CloudWatch logs của frontend và backend.
- Task definition frontend env có `BACKEND_URL=http://localhost:8080`.
- Backend có route `/api/auth/login`.

### WebSocket fail

Nguyên nhân thường gặp:

- Browser đang connect `ws://` thay vì `wss://`.
- ALB listener `443` không forward tới frontend target group.
- Nginx không proxy `/ws`.
- Backend không nhận request `/ws`.

Cách kiểm tra:

- DevTools Network -> WS.
- URL phải là `wss://call.example.com/ws?token=...`.
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

## 24. Thứ Tự Làm Nhanh Từ Con Số 0

**Step này để làm gì:** checklist tổng hợp nếu bạn muốn làm theo một mạch.

1. Chọn region `ap-southeast-1`.
2. Chuẩn bị domain hoặc subdomain trong Route 53.
3. Tạo ACM certificate cho domain ở `ap-southeast-1`.
4. Validate certificate bằng DNS trong Route 53.
5. Tạo ECR repositories.
6. Tạo CloudWatch log group.
7. Tạo SSM parameters cho JWT và TURN placeholder.
8. Tạo hoặc cập nhật `ecsTaskExecutionRole`.
9. Sửa `<ACCOUNT_ID>` trong `video-call/.aws/task-definition.json`.
10. Tạo ECS cluster.
11. Tạo ALB security group và ECS service security group.
12. Tạo target group `videocall-frontend-tg`.
13. Tạo ALB internet-facing.
14. Tạo listener HTTPS `443` forward vào target group.
15. Tạo listener HTTP `80` redirect sang HTTPS `443`.
16. Tạo Route 53 Alias A record trỏ domain vào ALB.
17. Push code lên GitHub `main` để workflow build/push image.
18. Register task definition nếu cần.
19. Tạo ECS service gắn với target group.
20. Kiểm tra target group healthy.
21. Mở `https://call.example.com`.
22. Kiểm tra API qua HTTPS.
23. Kiểm tra WebSocket qua WSS.
24. Test video call.
25. Nếu video không ổn định, cấu hình TURN thật.

## 25. Tài Liệu Tham Khảo

- [Route 53 - Routing traffic to an ELB load balancer](https://docs.aws.amazon.com/Route53/latest/DeveloperGuide/routing-to-elb-load-balancer.html)
- [AWS Certificate Manager - DNS validation](https://docs.aws.amazon.com/acm/latest/userguide/dns-validation.html)
- [Application Load Balancer - HTTPS listeners](https://docs.aws.amazon.com/elasticloadbalancing/latest/application/create-https-listener.html)
- [Application Load Balancer - Listener rules](https://docs.aws.amazon.com/elasticloadbalancing/latest/application/listener-update-rules.html)
