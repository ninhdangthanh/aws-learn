# Huong Dan Deploy Video Call Len AWS ECS

Guide nay dung cho app `video-call/` trong repo `aws-learn`.

Muc tieu deploy:

- Frontend React/Vite chay trong container Nginx.
- Backend Go chay trong container rieng.
- ECS Fargate chay ca 2 container trong cung 1 task.
- ALB public internet qua HTTPS.
- API dung `https://your-domain.com/api/...`.
- WebSocket signaling dung `wss://your-domain.com/ws`.
- WebRTC media dung STUN/TURN.

## 1. Luu y Quan Trong Ve Video Call

Browser chi cho phep camera/microphone tren:

- `localhost`, hoac
- site co HTTPS hop le.

Vi vay khi deploy that, app phai duoc mo bang HTTPS. Khi page da chay tren HTTPS, WebSocket cung phai chay bang WSS, khong duoc dung `ws://`.

Code hien tai da duoc chinh de production tu dong dung same-origin:

- API: `window.location.origin`, vi du `https://call.example.com`
- WS: tu dong chon `wss://call.example.com/ws` neu page dang o HTTPS

Ban khong can sua `frontend/src/config.ts` moi lan doi domain.

## 2. Kien Truc ECS

```
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

Luu y: ALB chi can route vao frontend container port 80. Nginx trong frontend se proxy `/api` va `/ws` sang backend trong cung task.

## 3. Chuan Bi

Can co:

- AWS account.
- AWS CLI da login/configure.
- GitHub repo chua code.
- Domain rieng, vi HTTPS cho camera/mic nen co domain that.
- ACM certificate cho domain.
- TURN server hoac TURN provider neu muon video call on dinh tren nhieu loai mang.

Vi du trong guide:

```bash
AWS_REGION=us-east-1
ACCOUNT_ID=123456789012
DOMAIN=call.example.com
```

Hay thay cac gia tri nay bang cua ban.

## 4. Tao ECR Repositories

Tao 2 repositories:

```bash
aws ecr create-repository --repository-name videocall-backend --region us-east-1
aws ecr create-repository --repository-name videocall-frontend --region us-east-1
```

Sau do image se co dang:

```text
<ACCOUNT_ID>.dkr.ecr.<REGION>.amazonaws.com/videocall-backend
<ACCOUNT_ID>.dkr.ecr.<REGION>.amazonaws.com/videocall-frontend
```

## 5. Tao CloudWatch Log Group

```bash
aws logs create-log-group --log-group-name /ecs/videocall --region us-east-1
```

Neu log group da ton tai thi co the bo qua loi `ResourceAlreadyExistsException`.

## 6. Tao Runtime Secrets Trong SSM

Task definition dang doc cac bien bi mat tu AWS Systems Manager Parameter Store:

- `/videocall/JWT_SECRET`
- `/videocall/TURN_URLS`
- `/videocall/TURN_USERNAME`
- `/videocall/TURN_CREDENTIAL`

Tao JWT secret:

```bash
aws ssm put-parameter \
  --name /videocall/JWT_SECRET \
  --type SecureString \
  --value "replace-with-a-long-random-secret" \
  --region us-east-1
```

Neu da co TURN server/provider:

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

Neu chua co TURN va chi muon deploy thu truoc, tao 3 parameter TURN bang mot dau cach:

```bash
aws ssm put-parameter --name /videocall/TURN_URLS --type SecureString --value " " --region us-east-1
aws ssm put-parameter --name /videocall/TURN_USERNAME --type SecureString --value " " --region us-east-1
aws ssm put-parameter --name /videocall/TURN_CREDENTIAL --type SecureString --value " " --region us-east-1
```

App se trim gia tri nay thanh rong. Tuy nhien, khong co TURN thi video call co the fail tren 4G/5G, WiFi cong ty, hoac NAT chat.

## 7. IAM Role Cho ECS Task

Can co role `ecsTaskExecutionRole`.

Role nay can policy AWS managed:

```text
AmazonECSTaskExecutionRolePolicy
```

Them inline policy de ECS doc SSM parameters:

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

Neu SSM SecureString dung customer managed KMS key, them quyen `kms:Decrypt` cho key do.

Sau do copy ARN cua role, vi du:

```text
arn:aws:iam::<ACCOUNT_ID>:role/ecsTaskExecutionRole
```

## 8. Sua Task Definition

Mo file:

```text
video-call/.aws/task-definition.json
```

Can thay:

- `<ACCOUNT_ID>` bang AWS account ID cua ban.
- `us-east-1` bang region cua ban neu khac.
- `executionRoleArn` dung ARN cua `ecsTaskExecutionRole`.

Dang mac dinh:

```json
"executionRoleArn": "arn:aws:iam::<ACCOUNT_ID>:role/ecsTaskExecutionRole"
```

Neu deploy region khac, sua ca:

- `awslogs-region`
- SSM parameter ARN trong `secrets`
- GitHub Actions `AWS_REGION`

## 9. Tao ECS Cluster

```bash
aws ecs create-cluster \
  --cluster-name videocall-cluster \
  --region us-east-1
```

Dung Fargate, khong can tao EC2 instance cho ECS.

## 10. Tao ACM Certificate Cho HTTPS

Vao AWS Certificate Manager trong cung region voi ALB, vi du `us-east-1`.

Request public certificate cho domain:

```text
call.example.com
```

Validate bang DNS. Neu dung Route 53, AWS co nut tao DNS record tu dong.

Chi tiep tuc sau khi certificate status la `Issued`.

## 11. Tao Security Groups

Can 2 security groups:

### ALB Security Group

Inbound:

- TCP 80 tu `0.0.0.0/0`
- TCP 443 tu `0.0.0.0/0`

Outbound:

- Cho phep ra ECS service security group port 80.

### ECS Service Security Group

Inbound:

- TCP 80 tu ALB security group.

Khong can public port 8080 cua backend ra internet. Backend chi duoc frontend Nginx proxy noi bo trong task.

## 12. Tao Target Group

Tao target group cho frontend:

- Target type: `IP`
- Protocol: `HTTP`
- Port: `80`
- VPC: VPC ban se chay ECS
- Health check path: `/`

Dat ten vi du:

```text
videocall-frontend-tg
```

## 13. Tao Application Load Balancer

Tao ALB internet-facing:

- Scheme: internet-facing
- Listeners:
  - HTTP 80
  - HTTPS 443
- Subnets: chon it nhat 2 public subnets
- Security group: ALB security group

Listener rules:

- HTTP 80: redirect sang HTTPS 443
- HTTPS 443: forward sang target group `videocall-frontend-tg`

Gan ACM certificate vao listener HTTPS 443.

## 14. Tro Domain Ve ALB

Neu dung Route 53:

- Tao `A` record alias cho `call.example.com`
- Alias target: ALB vua tao

Neu dung DNS provider khac:

- Tao `CNAME` tu `call.example.com` ve DNS name cua ALB

Cho DNS propagate xong, domain phai mo duoc qua HTTPS.

## 15. Register Task Definition Lan Dau

Tu repo root `aws-learn`, chay:

```bash
aws ecs register-task-definition \
  --cli-input-json file://video-call/.aws/task-definition.json \
  --region us-east-1
```

Luu y: image trong file ban dau la placeholder `<IMAGE_BE>` va `<IMAGE_FE>`. Neu AWS khong chap nhan placeholder khi register lan dau, hay push image truoc bang GitHub Actions/manual build, sau do register task definition co image ECR that.

## 16. Tao ECS Service

Tao service trong cluster `videocall-cluster`:

- Launch type: Fargate
- Task definition: `videocall-task`
- Service name: `videocall-service`
- Desired tasks: `1`
- VPC/subnets: chon private subnets neu co NAT, hoac public subnets neu muon don gian luc dau
- Security group: ECS service security group
- Load balancer:
  - Type: Application Load Balancer
  - Container: `frontend`
  - Container port: `80`
  - Target group: `videocall-frontend-tg`

Neu dung private subnets, task can ra internet de pull image ECR va ghi logs CloudWatch. Cach don gian la NAT Gateway. Cach toi uu hon la dung VPC endpoints cho ECR, CloudWatch Logs, SSM.

## 17. GitHub Actions Deploy

Workflow deploy nam o:

```text
.github/workflows/video-call-deploy.yml
```

Trong GitHub repo, vao Settings -> Secrets and variables -> Actions, tao:

```text
AWS_ACCESS_KEY_ID
AWS_SECRET_ACCESS_KEY
```

User/role cua key nay can quyen:

- Login/push ECR.
- Register task definition.
- Update ECS service.
- Pass role `ecsTaskExecutionRole`.

Khi push vao branch `main`, workflow se:

1. Build backend image tu `video-call/backend`.
2. Push image len ECR `videocall-backend`.
3. Build frontend image tu `video-call/frontend`.
4. Push image len ECR `videocall-frontend`.
5. Render task definition voi image moi.
6. Deploy ECS service.

Co the chay manual tu tab GitHub Actions vi workflow co `workflow_dispatch`.

## 18. HTTPS Va WSS Hoat Dong Nhu The Nao

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

ALB terminate TLS o ngoai. Ben trong ECS, Nginx proxy sang backend bang HTTP noi bo la duoc.

Khong can mo port 8080 public.

## 19. TURN Cho WebRTC

WebSocket chi dung de signaling. Video/audio that su di qua WebRTC peer connection.

STUN giup peer tim public address, nhung khong du cho moi network. TURN lam relay khi P2P khong ket noi duoc.

Nen dung TURN neu app dung that:

- EC2 chay `coturn`, hoac
- Managed TURN provider.

Thong tin TURN duoc inject vao frontend container tu SSM qua:

```text
TURN_URLS
TURN_USERNAME
TURN_CREDENTIAL
```

Frontend container generate file runtime:

```text
/usr/share/nginx/html/env-config.js
```

React app doc file nay khi browser load page.

## 20. SQLite Persistence

Hien backend dung SQLite:

```text
DB_PATH=/app/videocall.db
```

Trong Fargate, filesystem cua task khong ben vung. Neu task restart hoac redeploy, data co the mat.

Cho demo: co the chap nhan.

Cho production:

- Dung EFS mount vao `/app`, hoac
- Chuyen database sang RDS PostgreSQL/MySQL.

Khuyen nghi production: dung RDS.

## 21. Checklist Sau Khi Deploy

Kiem tra ECS:

- Service `videocall-service` stable.
- Co 1 running task.
- Target group health la healthy.
- CloudWatch logs khong co loi.

Kiem tra browser:

- Mo `https://call.example.com`.
- Dang ky/dang nhap duoc.
- DevTools Network thay API goi `https://...`.
- WebSocket ket noi `wss://.../ws?token=...`.
- Browser khong bao loi mixed content.
- Browser hoi permission camera/mic.
- Hai account online thay nhau trong user list.
- Goi video tren 2 thiet bi khac nhau.

Neu local dev:

- Frontend Vite van dung `http://localhost:5173`.
- Backend van dung `http://localhost:8080`.
- WebSocket local van dung `ws://localhost:8080/ws`.

## 22. Loi Thuong Gap

### Camera/mic khong hien permission

Nguyen nhan hay gap:

- Dang mo app bang HTTP public, khong phai HTTPS.
- Certificate sai hoac domain khong trusted.

### WebSocket fail

Kiem tra:

- Browser dang connect `wss://your-domain/ws`.
- ALB listener HTTPS forward dung target group.
- Nginx co proxy `/ws` voi Upgrade headers.
- Backend logs co request `/ws`.

### API bi mixed content

Neu page la HTTPS ma API goi HTTP, browser se block. Code hien tai da tranh loi nay bang same-origin config. Kiem tra `/env-config.js` neu ban override `API_URL`.

### Call duoc nhung khong co video/audio remote

Kiem tra:

- STUN/TURN config.
- ICE connection state trong browser console.
- Network co chan UDP khong.
- Thu them TURN relay.

### ECS task khong start

Kiem tra:

- Image ECR co ton tai.
- `ecsTaskExecutionRole` co quyen pull ECR va doc SSM.
- SSM parameter ARN dung region/account.
- CloudWatch log group `/ecs/videocall` da ton tai.

## 23. Thu Tu Lam Nhanh

Lam theo thu tu nay de it loi:

1. Tao ECR repositories.
2. Tao CloudWatch log group.
3. Tao SSM parameters.
4. Sua `<ACCOUNT_ID>` va region trong `video-call/.aws/task-definition.json`.
5. Tao/kiem tra `ecsTaskExecutionRole`.
6. Tao ECS cluster.
7. Tao ACM certificate.
8. Tao ALB security group va ECS security group.
9. Tao target group.
10. Tao ALB voi HTTPS 443 va redirect HTTP 80.
11. Tro domain ve ALB.
12. Push code len GitHub `main` de workflow build/push image.
13. Tao ECS service gan voi target group.
14. Mo `https://your-domain` va test `wss://.../ws`.
