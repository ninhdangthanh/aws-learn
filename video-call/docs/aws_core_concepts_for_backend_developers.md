# AWS Core Concepts Cho Backend Developer

File này gom các khái niệm AWS nền tảng dùng xuyên suốt nhiều service. Đây không phải guide DevOps đầy đủ, mà là mức **nhận biết đủ để backend developer đọc console, đọc log, deploy app, debug lỗi kết nối và quản lý nhiều project không bị rối**.

Nếu đang làm project `video-call/`, các guide chi tiết vẫn nằm ở:

- [AWS deployment guide](AWS_DEPLOYMENT_GUIDE.md)
- [Route 53, ACM, and ALB notes](route53_acm_alb_notes.md)
- [AWS ECS architecture questions](aws_ecs_architecture_questions.md)

## 1. AWS Account, Region Và Availability Zone

**AWS account** là biên giới lớn nhất cho billing, IAM permissions và resource ownership.

Ví dụ account ID:

```text
123456789012
```

**Region** là khu vực địa lý nơi resource được tạo, ví dụ:

```text
ap-southeast-1 = Singapore
us-east-1      = North Virginia
```

Khi deploy app, phải thống nhất region. Nếu ECR image ở `ap-southeast-1` nhưng ECS service ở `us-east-1`, ECS sẽ không dùng đúng image như bạn nghĩ.

**Availability Zone** là data center zone bên trong một region:

```text
ap-southeast-1a
ap-southeast-1b
ap-southeast-1c
```

Backend developer cần nhớ:

- Resource thường nằm trong một account.
- Nhiều resource chỉ tồn tại trong một region cụ thể.
- App production nên chạy qua ít nhất 2 Availability Zones nếu cần độ ổn định cao.
- ALB public thường cần ít nhất 2 public subnets ở 2 AZ khác nhau.

## 2. VPC Là Gì?

**VPC** là private network riêng của bạn trong AWS.

Có thể hiểu:

```text
VPC = mạng nội bộ của project hoặc môi trường
```

Ví dụ:

```text
VPC CIDR: 10.0.0.0/16
```

CIDR này nghĩa là các resource trong VPC có thể dùng IP private nằm trong range `10.0.x.x`.

Trong VPC thường có:

- Subnets.
- Route tables.
- Internet Gateway.
- NAT Gateway nếu private subnet cần đi internet.
- Security Groups.
- Network ACLs.
- VPC endpoints.

Backend developer không cần tự thiết kế network quá sâu ngay từ đầu, nhưng cần biết app của mình đang nằm trong VPC nào để debug:

- Vì sao service không gọi được database?
- Vì sao task không pull được image?
- Vì sao backend không ra internet gọi API bên thứ ba?
- Vì sao ALB health check fail?

## 3. Subnet Là Gì?

**Subnet** là một phần nhỏ của VPC, thường gắn với một Availability Zone.

Ví dụ:

```text
VPC:    10.0.0.0/16
Subnet: 10.0.1.0/24 in ap-southeast-1a
Subnet: 10.0.2.0/24 in ap-southeast-1b
```

Có 2 loại hay gặp:

```text
Public subnet
  -> có route ra Internet Gateway
  -> thường đặt ALB public, bastion host

Private subnet
  -> không nhận traffic trực tiếp từ internet
  -> thường đặt ECS tasks, databases, internal services
```

Với app web phổ biến:

```text
Internet
  -> ALB trong public subnets
  -> ECS tasks trong private subnets
  -> Database trong private subnets
```

Trong project nhỏ hoặc demo, ECS task đôi khi cũng đặt ở public subnet để đơn giản. Nhưng production thường tách public/private rõ hơn.

## 4. Route Table, Internet Gateway Và NAT Gateway

**Route table** quyết định traffic từ subnet đi đâu.

Public subnet thường có route:

```text
0.0.0.0/0 -> Internet Gateway
```

Ý nghĩa: traffic đi internet sẽ ra Internet Gateway.

Private subnet thường không route thẳng ra Internet Gateway. Nếu cần đi internet để gọi API ngoài, pull package, gửi webhook, subnet đó thường route qua NAT Gateway:

```text
0.0.0.0/0 -> NAT Gateway -> Internet Gateway
```

Nhận biết nhanh:

- ALB public cần public subnet.
- Backend private muốn gọi API ngoài cần NAT Gateway hoặc giải pháp tương đương.
- ECS task trong private subnet muốn pull ECR image cần NAT Gateway hoặc VPC endpoints cho ECR/S3/CloudWatch Logs.

## 5. Security Group Là Gì?

**Security Group** là firewall ở mức resource.

Ví dụ với web app:

```text
ALB security group
  inbound:
    80  từ 0.0.0.0/0
    443 từ 0.0.0.0/0

ECS service security group
  inbound:
    80 từ ALB security group
```

Điểm quan trọng: thay vì mở ECS service cho cả internet, nên chỉ cho ALB gọi vào ECS.

Khi backend không truy cập được resource khác, kiểm tra:

- Security group của caller có outbound không?
- Security group của target có inbound từ caller không?
- Port có đúng không?
- ALB target group health check có dùng đúng path và port không?

## 6. ARN Là Gì?

**ARN** là định danh đầy đủ của một resource trong AWS.

ARN thường có dạng:

```text
arn:partition:service:region:account-id:resource
```

Ví dụ:

```text
arn:aws:ecs:ap-southeast-1:123456789012:cluster/videocall-cluster
arn:aws:ecr:ap-southeast-1:123456789012:repository/videocall-backend
arn:aws:logs:ap-southeast-1:123456789012:log-group:/ecs/videocall
arn:aws:ssm:ap-southeast-1:123456789012:parameter/videocall/prod/jwt-secret
```

Đọc ARN giúp biết resource thuộc:

- Service nào: `ecs`, `ecr`, `logs`, `ssm`.
- Region nào: `ap-southeast-1`.
- Account nào: `123456789012`.
- Resource cụ thể nào: cluster, repository, log group, parameter.

ARN thường xuất hiện trong:

- IAM policy.
- ECS task execution role.
- CloudWatch logs.
- EventBridge rules.
- Error messages.

Backend developer nên đọc được ARN để tránh nhầm resource giữa project, region hoặc account.

## 7. IAM User, Role Và Policy

**IAM** kiểm soát ai được làm gì trên resource nào.

Các khái niệm hay gặp:

```text
IAM user
  -> user dài hạn, thường dùng cho người hoặc automation cũ

IAM role
  -> danh tính tạm thời để AWS service hoặc workload assume

IAM policy
  -> JSON nói rõ allow/deny action nào trên resource nào
```

Ví dụ ECS hay có:

```text
Task execution role
  -> ECS agent dùng để pull image từ ECR, ghi log vào CloudWatch, đọc secret khi start container

Task role
  -> code bên trong container dùng để gọi AWS API
```

Phân biệt này rất quan trọng:

- App code cần đọc S3 thì cấp quyền cho **task role**.
- ECS cần pull image hoặc ghi log thì cấp quyền cho **task execution role**.

Không nên nhét AWS access key dài hạn vào container nếu có thể dùng IAM role.

## 8. Resource Naming Và Namespace Cho Nhiều Project

Khi làm nhiều project trong cùng một AWS account, nên đặt tên resource có namespace rõ ràng.

Một format dễ dùng:

```text
<project>-<env>-<component>-<type>
```

Ví dụ:

```text
videocall-prod-api-service
videocall-prod-web-service
videocall-prod-alb
videocall-prod-api-tg
videocall-prod-cluster
videocall-prod-backend-repo
videocall-prod-frontend-repo
```

Hoặc nếu project nhỏ:

```text
videocall-cluster
videocall-service
videocall-alb
videocall-frontend-tg
```

Khi có nhiều môi trường:

```text
videocall-dev-cluster
videocall-staging-cluster
videocall-prod-cluster
```

Nên chọn một namespace cố định:

```text
Project = videocall
Env     = dev | staging | prod
Owner   = ninh
```

Rồi dùng namespace đó cho:

- ECS cluster/service/task family.
- ECR repositories.
- ALB và target groups.
- CloudWatch log groups.
- SSM parameters.
- Security groups.
- IAM roles nếu role chỉ dành cho project đó.

## 9. Tags Để Filter Resource

Tên resource giúp nhìn nhanh, nhưng **tags** mới là cách filter bền vững trong AWS.

Nên tag các resource chính:

```text
Project     = videocall
Environment = prod
Owner       = ninh
ManagedBy   = manual | terraform | github-actions
Service     = backend | frontend | shared
CostCenter  = learning
```

Khi nhiều project dùng chung account, tags giúp:

- Filter resource trong AWS Console.
- Xem cost theo project.
- Viết IAM policy hoặc automation chọn đúng resource.
- Dọn resource demo mà không xóa nhầm production.

Ví dụ naming và tagging đi cùng nhau:

```text
Resource name:
  videocall-prod-alb

Tags:
  Project=videocall
  Environment=prod
  Service=edge
```

Không phải resource nào cũng tag giống nhau hoàn toàn, nhưng nên giữ ít nhất:

```text
Project
Environment
Owner
```

## 10. SSM Parameter Store Và Secrets Manager

AWS có nhiều nơi để lưu config/secret. Hai dịch vụ hay gặp:

```text
SSM Parameter Store
  -> config và secret đơn giản
  -> path-based naming dễ đọc

Secrets Manager
  -> secret có rotation, metadata, integration sâu hơn
  -> thường đắt hơn nhưng phù hợp secret quan trọng
```

Ví dụ SSM path theo namespace:

```text
/videocall/prod/jwt-secret
/videocall/prod/database-url
/videocall/prod/turn-username
/videocall/prod/turn-password
```

Ưu điểm của path:

- Dễ filter theo project: `/videocall/...`
- Dễ tách env: `/videocall/dev/...`, `/videocall/prod/...`
- IAM policy có thể cấp quyền theo prefix.

Backend developer nên tránh hard-code secret trong:

- GitHub Actions YAML.
- Dockerfile.
- Task definition commit lên git.
- Frontend bundle.

## 11. CloudWatch Logs Và Metrics

**CloudWatch Logs** là nơi xem log runtime của app trên AWS.

Ví dụ log group:

```text
/ecs/videocall
/ecs/videocall-prod-api
/aws/lambda/image-worker-prod
```

Khi ECS task fail, nên kiểm tra:

- Log group có tồn tại không?
- Task execution role có quyền ghi log không?
- Container có start được không?
- App có bind đúng port không?
- Health check có gọi đúng endpoint không?

**CloudWatch Metrics** là số liệu vận hành:

- CPU.
- Memory.
- Request count.
- 4xx/5xx từ ALB.
- Target response time.
- Healthy/unhealthy targets.

Backend developer không cần thành chuyên gia monitoring ngay, nhưng nên biết log và metric nằm ở đâu để debug production issue.

## 12. Load Balancer, Target Group Và Health Check

**Application Load Balancer** nhận HTTP/HTTPS từ browser rồi route tới backend target.

**Target group** là danh sách target phía sau ALB.

Với ECS:

```text
ALB
  -> listener :443
  -> target group
  -> ECS task IP:port
```

Health check quyết định target có được nhận traffic không.

Ví dụ:

```text
Health check path: /health
Matcher: 200
Port: traffic-port
```

Nếu app chạy ổn trong container nhưng ALB báo unhealthy, thường kiểm tra:

- App có endpoint `/health` không?
- Endpoint trả `200` không?
- Container port và target group port có khớp không?
- Security group có cho ALB gọi vào ECS không?
- App có bind `0.0.0.0` thay vì chỉ bind `localhost` không?

## 13. Public Endpoint Và Private Endpoint

Không phải service nào cũng nên public ra internet.

Phân loại đơn giản:

```text
Public endpoint
  -> browser hoặc client ngoài internet gọi được
  -> ví dụ: ALB HTTPS, CloudFront, API Gateway

Private endpoint
  -> chỉ resource trong VPC hoặc account gọi được
  -> ví dụ: database, internal service, Redis
```

Với backend app:

```text
Browser
  -> public ALB
  -> private ECS service
  -> private database
```

Nguyên tắc nhận biết:

- Thứ người dùng cần gọi thì public qua endpoint có kiểm soát.
- Thứ app nội bộ dùng thì giữ private.
- Database gần như không nên public nếu không có lý do rất rõ.

## 14. Multi-Project Checklist

Khi tạo resource mới cho một project, tự hỏi:

- Resource này ở account nào?
- Resource này ở region nào?
- Tên có namespace project/env chưa?
- Có tags `Project`, `Environment`, `Owner` chưa?
- Có đang dùng nhầm VPC/subnet/security group của project khác không?
- Secret/config có path theo project/env chưa?
- IAM role/policy có scope vừa đủ hay đang mở quá rộng?
- Log group có tên dễ filter không?
- Cost của resource này có dễ truy ra project không?

Ví dụ namespace tốt:

```text
Project: videocall
Env:     prod
Region:  ap-southeast-1

ECS cluster:    videocall-prod-cluster
ECS service:    videocall-prod-web-service
ALB:            videocall-prod-alb
Target group:   videocall-prod-web-tg
Log group:      /ecs/videocall/prod/web
SSM parameter:  /videocall/prod/jwt-secret
ECR repo:       videocall/web
```

## 15. Những Khái Niệm Nên Nhận Biết Thêm

Một số khái niệm AWS tổng thể khác bạn sẽ gặp khi đọc docs hoặc console:

**Resource**

Một thứ cụ thể được tạo trong AWS: ECS cluster, ECR repo, ALB, S3 bucket, IAM role, log group.

**Service**

Dịch vụ AWS, ví dụ ECS, ECR, S3, Route 53, CloudWatch, IAM.

**Managed service**

AWS vận hành phần hạ tầng chính cho bạn. Ví dụ RDS quản lý database server tốt hơn việc tự cài database trên EC2.

**Control plane và data plane**

Control plane là API để tạo/sửa/xóa/cấu hình resource. Data plane là luồng request thật của app/user.

Ví dụ:

```text
Tạo ECS service = control plane
Browser gọi API qua ALB = data plane
```

**Least privilege**

Chỉ cấp quyền vừa đủ. Nếu app chỉ đọc một SSM parameter, không nên cấp quyền đọc toàn bộ SSM của account.

**Stateful và stateless**

Stateless service dễ scale/restart hơn vì không giữ dữ liệu quan trọng trong container local. Stateful resource như database, volume, queue cần backup và migration cẩn thận hơn.

**Immutable deployment**

Thay vì sửa trực tiếp server đang chạy, build image mới rồi deploy task/container mới.

**Infrastructure as Code**

Quản lý hạ tầng bằng code như Terraform, CloudFormation, CDK. Với learning project có thể thao tác manual trước, nhưng production nên tiến tới IaC để tránh click nhầm và dễ rebuild.

## 16. Mental Model Ngắn Gọn

Khi nhìn một AWS architecture, có thể đọc theo thứ tự:

```text
Account
  -> Region
  -> VPC
  -> Subnets
  -> Security Groups
  -> Public entrypoint: Route 53 / ALB / API Gateway / CloudFront
  -> Compute: ECS / Lambda / EC2
  -> Data: RDS / S3 / DynamoDB / Redis
  -> Config & secrets: SSM / Secrets Manager
  -> Logs & metrics: CloudWatch
  -> Permissions: IAM roles / policies
```

Với backend developer, mục tiêu không phải thuộc hết AWS, mà là đọc được resource đang nằm ở đâu, ai gọi ai, quyền nào đang được dùng, log nằm đâu, và làm sao filter đúng project khi account có nhiều thứ chạy cùng lúc.
