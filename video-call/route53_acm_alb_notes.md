# Ghi Chú Về Domain, Route 53, ACM Và ALB

Tài liệu này tóm tắt cách domain, Route 53, ACM certificate và Application Load Balancer phối hợp với nhau trong project video call.

https://ninh-video-call-demo.food
(NÀY MUA Ở NGOÀI, 42k TRÊN TRANG inet.com)

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

Nói ngắn gọn:

```text
Registrar: ai sở hữu domain?
Route 53: domain trỏ đi đâu?
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

## 17. Tóm Tắt Dễ Nhớ

```text
Registrar giữ quyền sở hữu domain.
Nameserver cho internet biết DNS của domain nằm ở đâu.
Route 53 hosted zone chứa DNS records.
A Alias trỏ domain về ALB.
ACM cấp certificate để ALB chạy HTTPS.
ALB nhận HTTPS/WSS và forward HTTP vào ECS frontend.
Frontend Nginx proxy API/WebSocket sang backend.
Backend không cần public ra internet.
```
