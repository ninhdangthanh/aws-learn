# Ghi Chú Về Domain, Route 53, ACM Và ALB

Tài liệu này tóm tắt cách domain, Route 53, ACM certificate và Application Load Balancer phối hợp với nhau trong project video call.

https://ninh-video-call-demo.food
(NÀY MUA Ở NGOÀI, 42k TRÊN TRANG inet.com)

Điểm quan trọng của project này:

```text
Domain được mua ở ngoài AWS: INET
DNS đang quản lý bằng AWS Route 53 Hosted Zone
```

Vì vậy so với hướng mua domain trực tiếp trong AWS Route 53 Domains, khác biệt chính chỉ là:

```text
Bạn phải vào INET để đổi nameserver sang 4 nameserver của Route 53.
```

Các phần còn lại vẫn giống nhau:

- Tạo Route 53 hosted zone.
- Tạo DNS records trong hosted zone.
- Tạo ACM certificate.
- Validate ACM bằng CNAME record trong Route 53.
- Tạo A Alias record trỏ domain về ALB.
- Gắn ACM certificate vào ALB HTTPS listener.

Route 53 không chỉ là nơi bán domain. Route 53 gồm nhiều phần, trong đó hay gặp nhất là:

- **Route 53 Domains**: dịch vụ đăng ký/mua/gia hạn domain.
- **Route 53 Hosted Zones**: nơi quản lý DNS records của domain.
- **Route 53 Records**: các record như A, A Alias, CNAME, MX, TXT, NS, SOA.
- **Route 53 Health Checks**: kiểm tra endpoint có sống không, dùng cho routing nâng cao.

Trong project này, bạn không dùng Route 53 Domains để mua domain, nhưng vẫn dùng Route 53 Hosted Zone để quản lý DNS.

Mục tiêu cuối cùng:

```text
https://ninh-video-call-demo.food
  -> Route 53
  -> Application Load Balancer HTTPS :443
  -> ECS frontend container :80
  -> frontend Nginx proxy /api, /ws sang backend :8080
```

## 1. Kiến Trúc Tổng Thể

```text
Browser
  |
  | HTTPS / WSS
  v
Domain
ninh-video-call-demo.food
  |
  | DNS lookup
  v
Route 53 Hosted Zone
  |
  | A Alias record
  v
Application Load Balancer
  |
  | HTTPS :443
  | HTTP  :80 redirect sang HTTPS
  v
Target Group
  |
  | HTTP :80
  v
ECS Service
  |
  v
ECS Task
  |-- frontend container :80
  |     |-- serve React app
  |     |-- proxy /api -> backend:8080
  |     |-- proxy /ws  -> backend:8080
  |
  |-- backend container :8080
```

Ý chính:

- Browser chỉ biết domain.
- Route 53 trả lời domain đó đang trỏ về ALB nào.
- ALB nhận HTTPS/WSS từ browser.
- ALB forward request vào frontend container trong ECS.
- Backend không cần public ra internet.

## 2. Domain Là Gì?

Ví dụ domain của project:

```text
ninh-video-call-demo.food
```

Domain là tên dễ nhớ để người dùng truy cập website. Thay vì phải nhớ IP hoặc DNS dài của ALB, người dùng chỉ cần mở:

```text
https://ninh-video-call-demo.food
```

Domain thường được mua tại một registrar, ví dụ:

- INET
- GoDaddy
- Namecheap
- AWS Route 53 Domains
- Cloudflare Registrar

Registrar chịu trách nhiệm:

- Đăng ký domain.
- Gia hạn domain.
- Quản lý quyền sở hữu domain.
- Cho phép đổi nameserver của domain.

Registrar không nhất thiết phải là nơi quản lý DNS record. Bạn có thể mua domain ở INET nhưng dùng Route 53 để quản lý DNS.

Với project này:

```text
INET = registrar, nơi mua và gia hạn domain.
Route 53 Hosted Zone = nơi quản lý DNS records.
```

Do đó câu “dùng Route 53” trong project này nên hiểu là “dùng Route 53 để quản lý DNS”, không phải “mua domain từ Route 53”.

## 3. Nameserver Là Gì?

Nameserver là DNS server chịu trách nhiệm trả lời DNS cho domain.

Ví dụ khi tạo Route 53 hosted zone, AWS cấp 4 authoritative nameserver:

```text
ns-1038.awsdns-01.org
ns-987.awsdns-59.net
ns-57.awsdns-07.com
ns-2037.awsdns-62.co.uk
```

Sau khi bạn cập nhật 4 nameserver này ở registrar, internet sẽ hiểu rằng:

```text
DNS của ninh-video-call-demo.food do Route 53 quản lý
```

Luồng lúc đó là:

```text
Browser
  -> hỏi DNS của ninh-video-call-demo.food
  -> được dẫn tới Route 53 nameserver
  -> Route 53 đọc hosted zone
  -> trả lời record tương ứng
```

Lưu ý quan trọng:

- Mỗi hosted zone thường có một bộ 4 nameserver riêng.
- Nếu xóa hosted zone rồi tạo lại, AWS có thể cấp bộ nameserver mới.
- Khi nameserver đổi, bạn phải cập nhật lại ở registrar.
- Nếu registrar vẫn trỏ về bộ nameserver cũ, DNS record trong hosted zone mới sẽ không có tác dụng.

## 4. Hosted Zone Là Gì?

Hosted zone là nơi chứa DNS records của domain trong Route 53.

Có thể hiểu đơn giản:

```text
Hosted zone = database DNS của domain
Nameserver  = server đọc database đó để trả lời internet
```

Ví dụ hosted zone của domain có thể chứa:

```text
ninh-video-call-demo.food       A Alias  -> ALB
_abc.ninh-video-call-demo.food  CNAME    -> _xyz.acm-validations.aws
ninh-video-call-demo.food       MX       -> mail server
ninh-video-call-demo.food       TXT      -> verification/SPF/DKIM
```

Hosted zone không tự route traffic. Nó chỉ chứa dữ liệu DNS. Traffic thật sự đi tới ALB sau khi browser resolve domain xong.

## 5. DNS Record Là Gì?

DNS record là từng dòng cấu hình trong hosted zone.

### A Record

A record trỏ domain về IPv4 address.

Ví dụ:

```text
example.com -> 54.xxx.xxx.xxx
```

Với ALB, bạn thường không dùng A record trỏ IP trực tiếp vì IP của ALB có thể thay đổi.

### A Alias Record

A Alias là cơ chế riêng của AWS Route 53. Nó cho phép domain trỏ trực tiếp tới AWS resource, ví dụ ALB.

Ví dụ:

```text
ninh-video-call-demo.food -> A Alias -> videocall-alb
```

Điểm hay của Alias:

- Không cần biết IP thật của ALB.
- AWS tự resolve ALB phía sau.
- Dùng được cho root domain, ví dụ `example.com`.
- Phù hợp hơn CNAME khi trỏ domain chính vào ALB.

Đây là record quan trọng nhất để public app ECS qua ALB.

### CNAME Record

CNAME trỏ một hostname sang hostname khác.

Ví dụ:

```text
www.example.com -> example.com
```

ACM cũng dùng CNAME để validate certificate:

```text
_abc.ninh-video-call-demo.food -> _xyz.acm-validations.aws
```

Không nên xóa CNAME validation của ACM sau khi certificate đã issued, vì ACM còn dùng record đó để tự gia hạn certificate.

### MX Record

MX record dùng cho email.

Ví dụ:

```text
example.com -> mail server của Google Workspace
```

Project video call không cần MX nếu bạn không dùng email theo domain.

### TXT Record

TXT record thường dùng cho xác minh và bảo mật email.

Ví dụ:

- Google Search Console verification.
- SPF.
- DKIM.
- DMARC.
- Các dịch vụ cần chứng minh bạn sở hữu domain.

### NS Record

NS record chỉ ra authoritative nameserver của zone.

Trong Route 53 hosted zone, bạn sẽ thấy NS record chứa 4 nameserver AWS cấp.

### SOA Record

SOA record chứa metadata của DNS zone, ví dụ serial, retry, refresh. Thường bạn không cần chỉnh record này khi deploy app.

## 6. Registrar Và Route 53 Khác Nhau Thế Nào?

Registrar là nơi quản lý quyền sở hữu domain.

Ví dụ với domain mua ở INET:

```text
INET
  -> mua domain
  -> gia hạn domain
  -> đổi nameserver
```

Route 53 là một dịch vụ DNS lớn của AWS, không chỉ là nơi bán domain. Trong Route 53 có nhiều mảng khác nhau:

```text
Route 53 Domains      -> mua/gia hạn/chuyển domain
Route 53 Hosted Zones -> quản lý DNS records
Route 53 Records      -> A, A Alias, CNAME, MX, TXT...
Route 53 Health Checks -> kiểm tra endpoint và routing nâng cao
```

Route 53 hosted zone là nơi quản lý DNS records:

```text
Route 53
  -> A Alias
  -> CNAME validation
  -> MX
  -> TXT
  -> các DNS records khác
```

Sau khi đổi nameserver ở INET sang 4 nameserver của Route 53:

```text
INET giữ quyền sở hữu domain
Route 53 trả lời DNS cho domain
```

Nói ngắn gọn với project hiện tại:

```text
INET: domain thuộc về ai, gia hạn ở đâu, nameserver là gì?
Route 53 Hosted Zone: domain trỏ đi đâu, records là gì?
```

## 7. ACM Certificate Là Gì?

ACM, viết tắt của AWS Certificate Manager, dùng để cấp SSL/TLS certificate cho HTTPS.

Với project này, certificate được gắn vào HTTPS listener của ALB.

Luồng ACM:

```text
Request certificate
  -> Pending validation
  -> Thêm CNAME validation vào Route 53
  -> ACM kiểm tra DNS
  -> Certificate chuyển sang Issued
  -> Attach certificate vào ALB HTTPS listener :443
```

ACM không route traffic. ACM chỉ cấp certificate để ALB có thể nói chuyện HTTPS với browser.

Quan trọng:

- Nếu certificate dùng cho ALB ở `ap-southeast-1`, certificate cũng phải tạo ở `ap-southeast-1`.
- Certificate `us-east-1` chỉ bắt buộc khi dùng CloudFront.
- Certificate phải chứa đúng domain bạn mở trên browser.

Ví dụ nếu mở:

```text
https://ninh-video-call-demo.food
```

thì certificate phải có domain:

```text
ninh-video-call-demo.food
```

## 8. CNAME Validation Của ACM Dùng Để Làm Gì?

Khi bạn request certificate cho domain, AWS cần chắc rằng bạn có quyền quản lý domain đó.

AWS sẽ yêu cầu bạn thêm một CNAME record dạng:

```text
_abc.ninh-video-call-demo.food -> _xyz.acm-validations.aws
```

Nếu bạn thêm được record này vào DNS, AWS hiểu rằng:

```text
Bạn có quyền sửa DNS của domain
```

Khi validation thành công, certificate chuyển sang:

```text
Issued
```

Không nên xóa CNAME validation sau đó, vì ACM dùng record này để tự renew certificate trước khi hết hạn.

## 9. Vì Sao Người Khác Không Thể Lấy Certificate Domain Của Mình?

Không phải vì 4 nameserver là bí mật. Nameserver là thông tin public, ai cũng có thể tra được.

Điểm bảo vệ thật sự là quyền sửa DNS.

Nếu người khác request certificate cho:

```text
ninh-video-call-demo.food
```

AWS cũng sẽ yêu cầu họ thêm CNAME validation vào DNS của domain đó.

Nhưng nếu họ không có quyền sửa DNS trong Route 53 hosted zone của bạn, họ không thể thêm record. Vì vậy certificate sẽ mãi ở trạng thái:

```text
Pending validation
```

và không bao giờ được `Issued`.

## 10. ALB Làm Gì Trong Kiến Trúc Này?

Application Load Balancer là cổng vào public của app.

Trong project này, ALB có hai listener:

```text
HTTP  :80  -> redirect sang HTTPS :443
HTTPS :443 -> forward sang target group frontend
```

HTTPS listener dùng ACM certificate để thực hiện TLS handshake với browser.

Sau khi giải mã HTTPS, ALB forward request vào target group bằng HTTP port `80`:

```text
Browser HTTPS
  -> ALB HTTPS :443
  -> Target Group HTTP :80
  -> ECS frontend container :80
```

Đây gọi là TLS termination tại ALB.

## 11. Target Group Là Gì?

Target group là danh sách các target mà ALB có thể forward request tới.

Với ECS Fargate dùng network mode `awsvpc`, target group nên có:

```text
Target type: IP
Protocol: HTTP
Port: 80
Health check path: /
```

ECS service sẽ đăng ký task đang chạy vào target group. Nếu task healthy, ALB sẽ gửi traffic vào task đó.

Nếu target group không healthy, kiểm tra:

- ECS task có đang `RUNNING` không.
- Frontend container có listen port `80` không.
- Security group của ECS có cho inbound `80` từ ALB security group không.
- Health check path `/` có trả về status thành công không.

## 12. Luồng Khi Browser Truy Cập Website

Khi người dùng mở:

```text
https://ninh-video-call-demo.food
```

luồng xảy ra như sau:

```text
Browser
  -> hỏi DNS cho ninh-video-call-demo.food
  -> Root DNS
  -> .food registry
  -> Route 53 nameserver
  -> Route 53 hosted zone
  -> A Alias record
  -> ALB
  -> HTTPS listener :443
  -> Target Group
  -> ECS frontend container :80
```

Sau đó frontend React được trả về browser.

Khi frontend gọi API:

```text
https://ninh-video-call-demo.food/api/auth/login
```

request vẫn đi qua cùng domain, vào frontend Nginx, rồi Nginx proxy sang backend:

```text
Browser
  -> ALB
  -> frontend Nginx /api
  -> backend :8080
```

Khi frontend mở WebSocket:

```text
wss://ninh-video-call-demo.food/ws?token=...
```

request đi:

```text
Browser
  -> ALB HTTPS/WSS :443
  -> frontend Nginx /ws
  -> backend :8080/ws
```

## 13. Vì Sao Backend Không Cần Public Port 8080?

Backend và frontend đang chạy trong cùng ECS task.

Task definition có:

```text
frontend container :80
backend container  :8080
```

Frontend Nginx có nhiệm vụ:

```text
/api -> http://localhost:8080
/ws  -> http://localhost:8080
```

Vì vậy internet chỉ cần đi vào frontend container qua ALB. Backend `8080` chỉ nhận traffic nội bộ trong task.

Security group nên là:

```text
ALB security group:
  inbound 80, 443 từ internet

ECS service security group:
  inbound 80 từ ALB security group
```

Không cần:

```text
inbound 8080 từ internet
```

## 14. Chỉ Cho Phép Truy Cập Bằng Domain

Bạn có thể cấu hình ALB listener rule để chỉ forward request khi host header đúng domain.

Ví dụ rule:

```text
IF Host header = ninh-video-call-demo.food
THEN Forward to videocall-frontend-tg
```

Default rule:

```text
Return fixed response 404
```

Khi đó:

```text
https://ninh-video-call-demo.food -> OK
https://ALB-DNS.amazonaws.com      -> 404 hoặc certificate mismatch
```

Cách này giúp giảm việc người dùng truy cập trực tiếp bằng ALB DNS. Nó không thay thế security group hoặc authentication, nhưng giúp routing rõ ràng và sạch hơn.

## 15. Những Gì Đã Làm Trong Project

Checklist hiện tại:

- Mua domain tại INET.
- Tạo Route 53 hosted zone.
- Lấy 4 nameserver từ Route 53.
- Cập nhật 4 nameserver đó ở INET.
- Tạo ACM certificate cho domain.
- Tạo CNAME validation record cho ACM.
- Certificate chuyển sang `Issued`.
- Tạo A Alias record trỏ domain về ALB.
- Gắn ACM certificate vào HTTPS listener của ALB.
- Cấu hình HTTP `80` redirect sang HTTPS `443`.
- Cấu hình ALB listener rule theo host header domain.
- ALB forward request vào target group frontend.
- Target group forward vào ECS frontend container port `80`.
- Frontend Nginx proxy `/api` và `/ws` sang backend `8080`.

## 16. Checklist Debug Nhanh

### Domain Không Vào Được Website

Kiểm tra:

```bash
dig ninh-video-call-demo.food
```

Kỳ vọng:

- Domain resolve được.
- DNS đang đi qua Route 53 nameserver.
- A Alias record trỏ tới ALB.

Nếu không resolve:

- Kiểm tra registrar đã đổi nameserver sang Route 53 chưa.
- Kiểm tra hosted zone có đúng domain không.
- Kiểm tra record có tạo ở đúng hosted zone không.

### HTTPS Báo Không An Toàn

Kiểm tra:

- ACM certificate đã `Issued` chưa.
- Certificate tạo ở đúng region với ALB chưa.
- Certificate có đúng domain đang mở chưa.
- ALB listener `443` có gắn đúng certificate chưa.

Không nên test HTTPS bằng ALB DNS:

```text
https://videocall-alb-xxx.ap-southeast-1.elb.amazonaws.com
```

Vì certificate của bạn cấp cho:

```text
ninh-video-call-demo.food
```

không cấp cho domain `elb.amazonaws.com`.

### ALB Vào Được Nhưng App Không Load

Kiểm tra:

- Target group có healthy target không.
- ECS service có running task không.
- Frontend container có listen port `80` không.
- ECS security group có inbound `80` từ ALB security group không.
- CloudWatch logs của frontend có lỗi không.

### API Login/Register Không Chạy

Kiểm tra:

- Browser đang gọi `https://ninh-video-call-demo.food/api/...` chưa.
- Frontend Nginx có proxy `/api` không.
- `BACKEND_URL` trong frontend container là `http://localhost:8080` chưa.
- Backend container có đang chạy không.
- CloudWatch logs backend có lỗi database hoặc JWT secret không.

### WebSocket Không Kết Nối

Kiểm tra:

- Browser đang dùng `wss://ninh-video-call-demo.food/ws?...` chưa.
- ALB listener `443` forward vào frontend target group chưa.
- Nginx location `/ws` có set `Upgrade` và `Connection` header không.
- Backend route `/ws` có nhận request không.

## 17. Hiện Trạng Project Này Đang Deploy Như Thế Nào?

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

Điều này nghĩa là backend và frontend đang chạy chung vòng đời:

- Deploy service là deploy cả backend và frontend.
- Scale service là scale cả backend và frontend cùng nhau.
- Nếu task restart, cả backend và frontend trong task đó cùng restart.
- Frontend có thể gọi backend qua `localhost:8080` vì hai container cùng nằm trong một ECS task với network mode `awsvpc`.

Task definition hiện tại có các điểm quan trọng:

```json
"networkMode": "awsvpc"
```

```json
"name": "backend",
"containerPort": 8080
```

```json
"name": "frontend",
"containerPort": 80
```

```json
"name": "BACKEND_URL",
"value": "http://localhost:8080"
```

`dependsOn` trong frontend:

```json
"dependsOn": [
  {
    "containerName": "backend",
    "condition": "START"
  }
]
```

Ý nghĩa: ECS sẽ start backend trước, rồi mới start frontend. Điều này giúp Nginx frontend proxy sang backend ổn định hơn lúc task khởi động.

## 18. Hiện Tại FE Call BE Bằng Cách Nào?

Browser không gọi thẳng backend container.

Browser gọi cùng domain hiện tại:

```text
https://ninh-video-call-demo.food/api/...
wss://ninh-video-call-demo.food/ws
```

Trong frontend code, production đang dùng same-origin:

```text
API_URL = window.location.origin
WS_URL  = wss://<current-host>/ws
```

Vì vậy nếu người dùng mở:

```text
https://ninh-video-call-demo.food
```

thì frontend sẽ gọi:

```text
https://ninh-video-call-demo.food/api/auth/login
wss://ninh-video-call-demo.food/ws?token=...
```

Sau đó request đi qua:

```text
Browser
  -> ALB HTTPS :443
  -> frontend Nginx :80
  -> /api hoặc /ws
  -> backend :8080
```

Frontend Nginx quyết định proxy sang backend bằng config:

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

Vậy câu trả lời ngắn gọn:

```text
FE gọi BE bằng same-origin URL từ browser.
Frontend Nginx proxy /api và /ws sang http://localhost:8080.
```

## 19. Có Tách Frontend Và Backend Thành 2 ECS Service Được Không?

Có. Backend và frontend có thể tách thành 2 ECS service cùng thuộc một ECS cluster.

Ví dụ:

```text
ECS cluster: videocall-cluster

Service 1:
  name: videocall-frontend-service
  task: frontend-task
  container: frontend :80
  gắn với ALB public target group

Service 2:
  name: videocall-backend-service
  task: backend-task
  container: backend :8080
  không public ra internet, chỉ nhận traffic nội bộ
```

Khi tách ra, `BACKEND_URL=http://localhost:8080` không còn đúng nữa, vì frontend và backend không còn chung task.

Bạn cần một trong các cách sau:

### Cách 1: Vẫn để ALB public route /api và /ws vào backend

ALB có thể có nhiều target group:

```text
Target group frontend: port 80
Target group backend:  port 8080
```

ALB listener rule:

```text
Host = ninh-video-call-demo.food AND Path = /api/* -> backend target group
Host = ninh-video-call-demo.food AND Path = /ws*   -> backend target group
Host = ninh-video-call-demo.food AND Path = /*     -> frontend target group
```

Khi đó browser vẫn gọi:

```text
https://ninh-video-call-demo.food/api/...
wss://ninh-video-call-demo.food/ws
```

Nhưng traffic `/api` và `/ws` không đi qua frontend Nginx nữa. ALB route trực tiếp tới backend service.

Ưu điểm:

- Tách service rõ ràng.
- Scale frontend/backend độc lập.
- Không cần service discovery nội bộ cho đường browser gọi API.

Nhược điểm:

- ALB config phức tạp hơn.
- Cần target group backend.
- Cần security group backend cho phép inbound `8080` từ ALB security group.

### Cách 2: Frontend service gọi backend service qua service discovery nội bộ

Bạn có thể dùng ECS Service Connect hoặc AWS Cloud Map để backend có DNS nội bộ, ví dụ:

```text
http://backend.videocall.local:8080
```

Khi đó frontend Nginx có thể proxy:

```text
BACKEND_URL=http://backend.videocall.local:8080
```

Ưu điểm:

- Backend không cần gắn trực tiếp với ALB.
- Backend chỉ nằm trong private network.

Nhược điểm:

- Cần cấu hình thêm service discovery hoặc Service Connect.
- Debug network nội bộ sẽ nhiều bước hơn.

Với project hiện tại, nếu mới học/deploy demo, để chung một service vẫn hợp lý. Khi cần scale backend/frontend độc lập hoặc muốn kiến trúc production rõ hơn, hãy tách thành 2 service.

## 20. ALB Và API Gateway Khác Nhau Thế Nào?

### ALB

ALB phù hợp với app web chạy container:

- Route HTTP/HTTPS theo host/path.
- Hỗ trợ WebSocket.
- Gắn trực tiếp với ECS target group.
- Phù hợp để serve frontend và proxy API.
- Chi phí và cấu hình thường dễ hiểu hơn cho app ECS nhỏ.

Project hiện tại dùng ALB là hợp lý vì:

```text
Browser -> ALB -> ECS frontend/backend
```

và app có WebSocket `/ws`.

### API Gateway

API Gateway phù hợp khi bạn muốn quản lý API như một lớp riêng:

- Rate limiting/throttling.
- API key.
- Authorizer.
- Request/response transformation.
- Version/stage API.
- Tích hợp Lambda hoặc HTTP backend.
- Có HTTP API và WebSocket API riêng.

API Gateway có thể đưa vào project này không?

Có, nhưng không bắt buộc.

Nếu thêm API Gateway, kiến trúc có thể thành:

```text
Frontend:
  Browser -> ALB -> ECS frontend

API:
  Browser -> API Gateway -> VPC Link -> ALB/NLB/private backend service
```

Hoặc:

```text
Browser -> API Gateway -> backend service
Browser -> ALB -> frontend service
```

Nhưng với video call app hiện tại, dùng API Gateway sẽ làm kiến trúc phức tạp hơn, đặc biệt vì bạn có cả REST API và WebSocket signaling.

Khuyến nghị:

- Giai đoạn hiện tại: dùng ALB là đủ.
- Sau này nếu cần rate limit, API key, authorizer riêng, hoặc muốn quản lý API độc lập: cân nhắc API Gateway.
- Nếu chỉ cần route `/api` và `/ws` vào ECS, ALB làm tốt rồi.

## 21. Các AWS Service Đang Dùng Và Còn Thiếu Gì Không?

Các service/tài nguyên bạn đã dùng:

- ECS: chạy container bằng service/task.
- ECR: lưu Docker image backend và frontend.
- EC2 Load Balancing: Application Load Balancer.
- EC2 Target Group: nơi ALB forward traffic.
- EC2 Security Group: firewall cho ALB và ECS task.
- IAM Role: `ecsTaskExecutionRole` để ECS pull image, ghi log, đọc secret.
- Route 53 Hosted Zone: quản lý DNS domain.
- ACM: cấp certificate HTTPS cho ALB.
- SSM Parameter Store: lưu secret runtime như JWT/TURN.
- CloudWatch Logs: nhận log từ ECS containers.

Những thứ nên thêm hoặc cân nhắc:

- VPC/subnet rõ ràng: biết service đang chạy ở public hay private subnet.
- NAT Gateway hoặc VPC endpoints: nếu ECS task chạy private subnet và cần pull image/gửi logs/đọc SSM.
- RDS hoặc EFS: nếu cần lưu database/file bền vững.
- AWS Backup: nếu dùng EFS/RDS production.
- WAF: nếu muốn chặn request xấu ở ALB.
- Terraform: để quản lý hạ tầng bằng code thay vì click console.
- CloudTrail: audit ai đã thay đổi tài nguyên AWS.

## 22. Export Config Về Terraform Được Không?

Được, nhưng cần hiểu đúng: Terraform không tự biết các tài nguyên bạn đã click trên AWS Console. Bạn cần đưa chúng vào Terraform state bằng import, hoặc dùng tool hỗ trợ generate.

Có 3 hướng phổ biến.

### Hướng 1: Viết Terraform thủ công rồi import

Bạn viết resource Terraform tương ứng:

```hcl
resource "aws_lb" "videocall" {
  name = "videocall-alb"
}
```

Sau đó import tài nguyên thật:

```bash
terraform import aws_lb.videocall arn:aws:elasticloadbalancing:ap-southeast-1:ACCOUNT_ID:loadbalancer/app/videocall-alb/...
```

Sau khi import, chạy:

```bash
terraform plan
```

Nếu plan báo muốn thay đổi nhiều thứ, bạn bổ sung config cho khớp với hạ tầng thật.

### Hướng 2: Dùng Terraform import block

Terraform bản mới hỗ trợ import block:

```hcl
import {
  to = aws_lb.videocall
  id = "arn:aws:elasticloadbalancing:ap-southeast-1:ACCOUNT_ID:loadbalancer/app/videocall-alb/..."
}
```

Sau đó chạy:

```bash
terraform plan
terraform apply
```

### Hướng 3: Dùng tool generate như Terraformer

Một số tool như `terraformer` có thể đọc AWS account và sinh Terraform file/state ban đầu.

Ví dụ nhóm tài nguyên cần export:

```text
ECS cluster
ECS service
ECS task definition
ECR repositories
ALB
ALB listeners
Target groups
Security groups
Route 53 records
ACM certificate
IAM role/policies
CloudWatch log group
SSM parameters metadata
VPC/subnets nếu muốn quản lý luôn network
```

Lưu ý:

- Secret value trong SSM SecureString không nên đưa vào Terraform plaintext.
- ACM certificate DNS validation record có thể import, nhưng private key/certificate do ACM quản lý.
- Task definition thường thay đổi mỗi lần deploy image, nên cần quyết định Terraform quản lý task definition hay để GitHub Actions quản lý revision.
- Nếu Terraform quản lý ECS service, cần cẩn thận để Terraform không rollback image tag mà GitHub Actions vừa deploy.

Khuyến nghị cho project này:

```text
Terraform quản lý hạ tầng nền:
  VPC, subnet, security group, ALB, target group, listener, Route 53, ACM, ECR, ECS cluster, IAM, log group.

GitHub Actions quản lý deploy app:
  build image, push ECR, render task definition, update ECS service.
```

## 23. VPC, Subnet, Security Group Là Gì?

### VPC

VPC là mạng riêng ảo trong AWS account.

Có thể hiểu:

```text
VPC = private network của bạn trên AWS
```

Trong VPC có:

- Subnet.
- Route table.
- Internet Gateway.
- NAT Gateway.
- Security Group.
- Load Balancer.
- ECS task network interface.

ALB và ECS service phải nằm trong cùng VPC hoặc trong network có thể route tới nhau.

### Subnet

Subnet là một phần nhỏ của VPC, thường nằm trong một Availability Zone.

Ví dụ:

```text
VPC: 10.0.0.0/16
Subnet A: 10.0.1.0/24, ap-southeast-1a
Subnet B: 10.0.2.0/24, ap-southeast-1b
```

Public subnet:

- Có route ra Internet Gateway.
- ALB internet-facing thường nằm ở public subnets.

Private subnet:

- Không public trực tiếp ra internet.
- ECS task production thường nên nằm ở private subnets.
- Nếu task cần pull image/gửi logs/đọc SSM, cần NAT Gateway hoặc VPC endpoints.

### Security Group

Security group là firewall cấp resource.

Inbound là traffic đi vào resource.

Outbound là traffic đi ra khỏi resource.

Ví dụ ALB security group:

```text
Inbound:
  TCP 80  từ 0.0.0.0/0
  TCP 443 từ 0.0.0.0/0

Outbound:
  TCP 80 tới ECS service security group
```

Ví dụ ECS service security group:

```text
Inbound:
  TCP 80 từ ALB security group

Outbound:
  allow all, hoặc tối thiểu cho ECR/CloudWatch/SSM/backend dependencies
```

Không nên mở:

```text
TCP 8080 từ 0.0.0.0/0
```

vì backend không cần public.

## 24. Register Task Definition, Task Definition ARN Và Task ARN

### Task Definition

Task definition là bản thiết kế để ECS biết:

- Chạy container nào.
- Image nào.
- CPU/memory bao nhiêu.
- Port nào.
- Env/secrets nào.
- Log gửi về đâu.
- Role nào được dùng.

Trong repo, file task definition là:

```text
video-call/.aws/task-definition.json
```

Khi chạy:

```bash
aws ecs register-task-definition \
  --cli-input-json file://video-call/.aws/task-definition.json \
  --region ap-southeast-1
```

AWS tạo một revision mới cho task definition.

Ví dụ ARN:

```text
arn:aws:ecs:ap-southeast-1:ACCOUNT_ID:task-definition/videocall-task:12
```

Đây gọi là task definition ARN.

### Task ARN

Task ARN là ARN của một task đang chạy cụ thể.

Ví dụ:

```text
arn:aws:ecs:ap-southeast-1:ACCOUNT_ID:task/videocall-cluster/abc123...
```

So sánh:

```text
Task definition ARN = bản thiết kế version 12.
Task ARN            = một instance đang chạy từ bản thiết kế đó.
```

ECS service dùng task definition để tạo task. Khi bạn deploy revision mới, service sẽ stop task cũ và start task mới theo deployment strategy.

## 25. Lưu File Database Trong ECS Có An Toàn Không?

Hiện backend dùng:

```text
DB_PATH=/app/videocall.db
```

Nếu file database nằm trong filesystem của container/Fargate task, nó không bền vững.

Khi xảy ra một trong các việc sau:

- ECS task restart.
- Deploy image mới.
- Service scale down/up.
- Fargate host thay đổi.

file trong container có thể mất.

Nếu service chết thì lấy lại file thế nào?

- Nếu file chỉ nằm trong container filesystem, gần như không nên kỳ vọng lấy lại được.
- Có thể debug tạm khi task còn sống bằng ECS Exec nếu đã bật, nhưng đây không phải backup.
- Cách đúng là không lưu dữ liệu quan trọng trong container filesystem.

Các hướng đúng hơn:

### Dùng RDS

Phù hợp nhất cho database production:

```text
Backend -> RDS PostgreSQL/MySQL
```

Ưu điểm:

- Dữ liệu bền vững.
- Có backup/snapshot.
- Chạy ổn hơn SQLite cho nhiều user.

### Dùng EFS

Nếu vẫn muốn dùng file database hoặc file upload:

```text
ECS task -> mount EFS -> /app
```

Ưu điểm:

- File tồn tại ngoài vòng đời container.
- Task restart vẫn thấy file.

Nhược điểm:

- SQLite trên network filesystem cần cẩn thận về locking/concurrency.
- Với app production, RDS vẫn là lựa chọn tốt hơn cho database.

### Dùng S3

S3 phù hợp để lưu object/file, ví dụ:

- Avatar.
- File upload.
- Recording.
- Backup file.

S3 không phù hợp để mount trực tiếp làm database đang ghi liên tục như SQLite/MDB.

Nếu muốn backup file database lên S3, app hoặc cron job có thể định kỳ upload:

```text
/app/videocall.db -> s3://bucket/backups/videocall.db
```

Nhưng đây là backup kiểu snapshot, không phải database storage realtime.

Khuyến nghị:

```text
Database thật: dùng RDS.
File dùng chung: dùng S3.
File cần mount vào container: dùng EFS.
Demo nhanh: SQLite trong container được, nhưng chấp nhận mất data khi task chết.
```

## 26. Tóm Tắt Dễ Nhớ

```text
Registrar giữ quyền sở hữu domain.
Nameserver cho internet biết DNS của domain nằm ở đâu.
Route 53 hosted zone chứa DNS records.
A Alias trỏ domain về ALB.
ACM cấp certificate để ALB chạy HTTPS.
ALB nhận HTTPS/WSS và forward HTTP vào ECS frontend.
Frontend Nginx proxy API/WebSocket sang backend.
Backend không cần public ra internet.
Task definition là bản thiết kế.
Task ARN là task đang chạy.
VPC là mạng riêng.
Subnet là vùng mạng nhỏ trong VPC.
Security group là firewall inbound/outbound.
Container filesystem không phải nơi lưu database bền vững.
```
