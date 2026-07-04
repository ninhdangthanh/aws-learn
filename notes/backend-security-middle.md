# Backend Security Middle Interview Notes

File này gom các kiến thức security backend ở mức Middle: đủ để giải thích trong phỏng vấn, nhận diện rủi ro production và biết nên đặt lớp phòng vệ ở đâu. Trọng tâm là phòng thủ hệ thống backend, không đi vào cách tấn công chi tiết.

---

## 1. Bức tranh tổng quan

Security backend không chỉ là "có JWT" hay "có HTTPS". Một service production thường cần nhiều lớp:

* Edge layer: CDN, WAF, DDoS protection, bot detection, rate limit.
* API gateway/reverse proxy: auth check, request size limit, timeout, routing, per-client quota.
* Application layer: authorization, input validation, idempotency, business rule, audit log.
* Data layer: parameterized query, least privilege DB user, encryption, backup, access control.
* Operation layer: secret management, dependency scanning, log redaction, alerting.

Câu trả lời phỏng vấn tốt:

> Tôi nhìn security theo nhiều lớp. Ở edge có WAF/rate limit để chặn traffic bất thường, ở gateway có auth/timeout/request limit, trong service có authorization/input validation/idempotency, còn ở data layer thì dùng prepared statement, quyền DB tối thiểu và không log dữ liệu nhạy cảm. Không có một cơ chế đơn lẻ nào đủ bảo vệ toàn bộ hệ thống.

---

## 2. Password Hashing, Salt và Rainbow Table

### Rainbow table là gì?

Rainbow table là bảng tính sẵn mapping giữa password phổ biến và hash tương ứng. Nếu hệ thống lưu password bằng hash yếu hoặc không có salt, attacker có thể so hash bị leak với bảng tính sẵn để tìm password gốc nhanh hơn brute force.

Ví dụ sai:

```text
password = "123456"
stored_hash = sha256("123456")
```

Nếu nhiều user dùng cùng password, hash giống nhau. Khi database bị leak, attacker chỉ cần lookup hash trong bảng có sẵn.

### Salt giải quyết gì?

Salt là chuỗi random riêng cho từng password. Hash sẽ tính từ password + salt:

```text
stored_hash = argon2(password, random_salt)
```

Hai user cùng password vẫn có hash khác nhau. Rainbow table tính sẵn gần như mất tác dụng vì attacker phải tính lại cho từng salt.

### Vì sao không dùng SHA256/MD5 để lưu password?

SHA256/MD5 quá nhanh. Password hashing cần thuật toán chậm và có thể chỉnh cost để làm brute force tốn tài nguyên hơn.

Nên dùng:

* `argon2id`: lựa chọn hiện đại, chống GPU tốt hơn nếu cấu hình memory cost hợp lý.
* `bcrypt`: rất phổ biến, ổn cho nhiều hệ thống production.
* `scrypt`: cũng là lựa chọn tốt nhưng ít phổ biến hơn bcrypt/argon2 trong backend thông dụng.

Không nên:

* tự thiết kế thuật toán hash;
* lưu plain text;
* dùng cùng một salt hardcode cho mọi user;
* log password hoặc password hash không cần thiết.

Câu trả lời phỏng vấn:

> Rainbow table nguy hiểm khi password hash không có salt hoặc dùng hash quá nhanh như SHA256. Trong backend tôi không lưu password plain text, mà dùng bcrypt hoặc argon2id. Mỗi password có salt riêng, cost được cấu hình đủ cao để login vẫn chấp nhận được nhưng brute force offline trở nên đắt hơn.

---

## 3. Brute Force Login Protection

Brute force login là việc thử nhiều cặp username/password để đoán credential. Backend cần bảo vệ cả password brute force, credential stuffing và user enumeration.

### Các lớp phòng vệ

* Rate limit theo IP: giới hạn request từ một địa chỉ IP trong khoảng thời gian.
* Rate limit theo account/email: tránh attacker đổi IP liên tục để thử cùng một account.
* Rate limit theo device/session fingerprint: hữu ích khi IP bị NAT hoặc proxy.
* Exponential backoff: sau nhiều lần sai, tăng thời gian chờ trước lần thử tiếp theo.
* CAPTCHA/challenge: chỉ bật khi nghi ngờ bot, không nên ép mọi login bình thường.
* MFA/passkey: giảm thiệt hại khi password bị lộ.
* Audit log và alert: phát hiện spike login fail bất thường.

### Có nên lock account không?

Lock account cứng có thể bị lợi dụng để DoS người dùng thật: attacker cố tình nhập sai nhiều lần để khóa tài khoản nạn nhân.

Cách cân bằng:

* không lock vĩnh viễn;
* dùng cooldown ngắn;
* yêu cầu step-up verification như OTP/email challenge;
* giữ message lỗi chung chung: "email hoặc mật khẩu không đúng";
* không tiết lộ email có tồn tại hay không.

### Redis pattern đơn giản

```text
login_fail:ip:{ip}              -> counter, TTL 5-15 phút
login_fail:account:{user_id}    -> counter, TTL 15-60 phút
login_challenge:{user_id}       -> require captcha/otp flag
```

Khi login sai:

1. Tăng counter theo IP và account.
2. Nếu vượt ngưỡng nhẹ, delay hoặc require CAPTCHA.
3. Nếu vượt ngưỡng cao, tạm chặn trong thời gian ngắn.
4. Log event để quan sát và alert.

Khi login đúng:

* reset counter phù hợp;
* rotate refresh token/session;
* ghi audit event nếu login từ thiết bị/vị trí lạ.

Câu trả lời phỏng vấn:

> Tôi không chỉ rate limit theo IP vì attacker có thể dùng nhiều IP. Tôi thường kết hợp IP, account và device fingerprint. Với login fail liên tục, hệ thống tăng delay, yêu cầu CAPTCHA hoặc OTP. Tôi tránh lock account vĩnh viễn vì có thể bị dùng để DoS người dùng thật.

### Bypass qua nhiều account hoặc nhiều IP

Rate limit theo một chiều rất dễ bị né:

* Nếu chỉ limit theo IP, attacker có thể dùng proxy/VPN/botnet để đổi IP.
* Nếu chỉ limit theo account, attacker có thể tạo nhiều account mới để spam login, scraping, referral abuse hoặc abuse promotion.
* Nếu chỉ limit theo user-agent, attacker có thể fake header.
* Nếu chỉ limit sau khi login, attacker vẫn có thể abuse endpoint public như register, forgot password, search hoặc product listing.

Vì vậy backend nên dùng rate limit tổng hợp theo nhiều dimension:

```text
rate:ip:{ip}
rate:account:{user_id}
rate:email_domain:{domain}
rate:device:{fingerprint}
rate:subnet:{ip_prefix}
rate:endpoint:{endpoint}:{ip_or_user}
rate:tenant:{tenant_id}
```

Với hành vi tạo nhiều account, cần thêm lớp chống abuse:

* verify email/phone với cooldown hợp lý;
* CAPTCHA/challenge ở register/login/forgot password khi có tín hiệu bất thường;
* device fingerprint hoặc session fingerprint để nhận ra nhiều account từ cùng thiết bị;
* velocity rule, ví dụ nhiều account mới cùng IP/subnet/device trong thời gian ngắn;
* giới hạn tính năng nhạy cảm cho account mới, ví dụ export, invite, promotion, gửi message;
* fraud score/risk score dựa trên pattern: IP reputation, ASN/datacenter IP, user-agent, timezone, request interval;
* audit log để điều tra abuse theo cụm account thay vì từng account rời rạc.

Điểm cần cẩn thận:

* Device fingerprint không nên coi là định danh tuyệt đối vì có thể sai hoặc ảnh hưởng privacy.
* CAPTCHA không nên bật cho mọi user bình thường vì làm giảm conversion.
* Block IP quá mạnh dễ ảnh hưởng user thật ở công ty, quán cafe, mạng mobile NAT.
* Với account mới, nên giảm quyền/rate dần theo trust level thay vì chặn cứng mọi thứ.

Câu trả lời phỏng vấn:

> Nếu user có thể tạo nhiều account hoặc dùng nhiều IP, rate limit một chiều sẽ không đủ. Tôi sẽ kết hợp limit theo IP, account, device fingerprint, endpoint và tenant; đồng thời thêm velocity rule để phát hiện nhiều account mới từ cùng thiết bị/subnet. Với tín hiệu rủi ro cao, hệ thống có thể bật CAPTCHA, step-up verification hoặc giảm quota cho account mới thay vì block cứng gây ảnh hưởng user thật.

---

## 4. Scraping và Bot Abuse

Scraping là việc client tự động crawl API/page để lấy dữ liệu hàng loạt. Không phải scraping nào cũng là tấn công, nhưng với backend production nó có thể gây:

* tăng traffic và cost;
* làm chậm DB/cache;
* lấy dữ liệu business quan trọng;
* phá rate limit nếu dùng nhiều IP;
* tạo tải giống DDoS nhẹ.

### Backend nên làm gì?

* Rate limit theo IP, user, API key, tenant và endpoint.
* Quota theo gói sử dụng: free/pro/internal/admin.
* Pagination bắt buộc, giới hạn `limit` tối đa.
* Không expose endpoint export dữ liệu lớn nếu không có async job và quyền rõ ràng.
* Detect pattern bất thường: request tuần tự toàn bộ ID, scan page quá nhanh, user-agent lạ, tỷ lệ cache miss cao.
* Challenge/CAPTCHA ở edge khi nghi ngờ bot.
* Dùng signed URL hoặc short-lived token cho tài nguyên nhạy cảm.
* Cache dữ liệu public để giảm tải origin.

### Robots.txt có đủ không?

Không. `robots.txt` chỉ là quy ước cho crawler tử tế, không phải cơ chế bảo mật. Bot độc hại có thể bỏ qua hoàn toàn.

### Chống scraping không nên làm quá tay

* Không block toàn bộ IP shared/NAT quá mạnh vì dễ ảnh hưởng user thật.
* Không dựa duy nhất vào user-agent vì dễ fake.
* Không đặt rate limit một chiều cho mọi endpoint; login, search, export, upload cần rule khác nhau.

Câu trả lời phỏng vấn:

> Với scraping, tôi không xem robots.txt là security. Tôi sẽ giới hạn pagination, đặt quota theo user/API key, rate limit theo endpoint, detect hành vi scan dữ liệu tuần tự và dùng challenge ở edge nếu nghi bot. Với dữ liệu lớn, tôi chuyển sang async export có quyền rõ ràng thay vì cho query trực tiếp kéo toàn bộ DB.

---

## 5. DDoS nhẹ và App-level DDoS

DDoS lớn thường cần CDN/WAF/provider-level protection. Nhưng backend vẫn phải chống app-level DDoS: request hợp lệ về mặt HTTP nhưng gây tốn tài nguyên.

Ví dụ:

* gọi endpoint search/filter rất nặng liên tục;
* request body cực lớn;
* mở nhiều connection chậm;
* spam upload file;
* tạo nhiều job async làm nghẽn queue;
* cache miss storm vào DB.

### Phòng vệ ở backend

* Request timeout ở gateway và app server.
* Body size limit cho JSON/form/upload.
* Rate limit theo endpoint và theo authenticated user/API key.
* Concurrency limit cho endpoint nặng.
* Queue/backpressure cho tác vụ tốn CPU/I/O.
* Circuit breaker với dependency đang chậm.
* Cache và TTL jitter cho dữ liệu đọc nhiều.
* Reject sớm request thiếu auth hoặc invalid input.
* Alert theo p95 latency, 4xx/5xx, request rate, queue depth, DB CPU.

### Fingerprinting trong DDoS protection là gì?

Fingerprinting là cách hệ thống tạo một "dấu vân tay" cho client/request bằng cách kết hợp nhiều tín hiệu. Mục tiêu là nhận ra traffic có cùng nguồn hoặc cùng automation pattern, kể cả khi attacker đổi IP hoặc tạo nhiều account.

Fingerprint không phải định danh tuyệt đối của một người dùng. Nó là tín hiệu rủi ro để rate limit, challenge hoặc block mềm.

Tín hiệu thường dùng:

```text
IP / subnet / ASN
User-Agent
Accept-Language
TLS fingerprint
HTTP header order
Cookie/session/device id
Account id / API key / tenant id
Request path/query pattern
Request rate / timing pattern
Failed login / cache miss / expensive endpoint pattern
```

Ví dụ:

```text
1000 IP khác nhau
nhưng cùng User-Agent lạ
cùng header order
cùng gọi /search?q=... với interval đều 200ms
cùng không giữ cookie

=> có thể gom thành một fingerprint/pattern
=> rate limit, challenge CAPTCHA hoặc block ở edge/gateway
```

### Fingerprinting config ở level nào?

| Level | Config được gì? | Hợp để làm gì? |
|---|---|---|
| CDN/WAF | IP reputation, ASN, country, bot score, TLS/header fingerprint, challenge/block rule | Chặn sớm trước khi vào backend |
| Load balancer/API Gateway | rate limit theo IP/API key/JWT claim, header/cookie/path rule, body size, timeout | Bảo vệ service theo route/API |
| Backend middleware | user_id, tenant_id, session_id, device_id, endpoint cost, behavior pattern | Rule có business context |
| Auth/session layer | trusted device, session fingerprint, new-device alert, step-up auth | Login, OTP, payment, đổi mật khẩu |
| Worker/queue layer | consumer concurrency, prefetch, job dedup, DLQ, downstream rate limit | Chống overload side effect/downstream |

Ví dụ key rate limit ở backend:

```text
ddos:ip:{ip}
ddos:subnet:{ip_prefix}
ddos:fingerprint:{hash(user_agent + header_order + accept_language)}
ddos:user:{user_id}:endpoint:{endpoint}
ddos:tenant:{tenant_id}:export
login:{account}:{ip_subnet}:{device_fingerprint}
```

Điểm cần cẩn thận:

* Không chỉ dựa vào IP vì attacker có thể dùng proxy/VPN/botnet.
* Không coi fingerprint là danh tính tuyệt đối vì có false positive.
* Không log dữ liệu fingerprint nhạy cảm ở dạng raw nếu không cần; nên hash/sanitize.
* Rule ở WAF/CDN giúp chặn sớm, nhưng backend vẫn cần business-context rate limit.
* Block cứng dễ ảnh hưởng user thật ở NAT/mobile network; nên có challenge, giảm quota hoặc step-up verification.

Câu trả lời phỏng vấn:

> DDoS lớn cần WAF/CDN, nhưng backend vẫn phải tự bảo vệ khỏi app-level DDoS. Tôi đặt timeout, body limit, rate limit theo endpoint, concurrency limit cho API nặng và dùng queue/backpressure cho tác vụ tốn tài nguyên. Với fingerprinting, tôi kết hợp IP/subnet, user-agent/header pattern, cookie/session, user/API key và behavior để phát hiện traffic automation thay vì chỉ limit theo IP. Mục tiêu là fail fast hoặc challenge sớm thay vì để request xấu giữ worker, connection pool hoặc DB quá lâu.

---

## 6. SQL Injection và NoSQL Injection

### SQL Injection

SQL injection xảy ra khi backend ghép input của user trực tiếp vào câu SQL.

Sai:

```sql
SELECT * FROM users WHERE email = '${email}' AND password = '${password}';
```

Đúng:

```sql
SELECT * FROM users WHERE email = $1;
```

Nguyên tắc:

* dùng parameterized query/prepared statement;
* không nối string từ input vào SQL;
* validate sort field/filter field bằng allowlist;
* DB user dùng quyền tối thiểu;
* log query cẩn thận, tránh log PII/token.

Điểm hay bị quên: `ORDER BY`, `LIMIT`, tên column, tên table thường không parameterize trực tiếp được. Với các field này phải dùng allowlist.

### NoSQL Injection với MongoDB

NoSQL injection xảy ra khi input được đưa thẳng vào query object.

Ví dụ rủi ro:

```json
{
  "email": { "$ne": null }
}
```

Nếu backend nhận raw JSON rồi đưa thẳng vào Mongo query, attacker có thể chèn operator như `$ne`, `$gt`, `$where`.

Phòng tránh:

* validate schema request bằng Zod/Joi/class-validator/go validator;
* ép kiểu field rõ ràng, ví dụ email phải là string;
* không nhận raw filter object từ client nếu không có allowlist;
* disable/không dùng operator nguy hiểm như `$where`;
* phân quyền endpoint search/filter.

Câu trả lời phỏng vấn:

> Với SQL tôi dùng parameterized query và allowlist cho sort/filter field. Với MongoDB, tôi không đưa raw JSON từ client vào query mà validate schema, ép kiểu rõ ràng và chỉ cho phép các operator/filter được định nghĩa trước.

---

## 7. CORS không phải Authentication

CORS là cơ chế của browser để kiểm soát website nào được phép gọi API từ frontend. CORS không chặn được curl, mobile app, server-to-server request hoặc bot.

Sai lầm thường gặp:

* nghĩ rằng set CORS là đã bảo vệ API;
* dùng `Access-Control-Allow-Origin: *` cùng credential;
* allow toàn bộ origin động theo request mà không kiểm tra allowlist;
* quên xử lý preflight `OPTIONS`;
* nhầm CORS với CSRF.

Nên làm:

* allowlist origin rõ ràng theo environment;
* nếu dùng cookie credential, không dùng wildcard origin;
* auth/authorization vẫn phải làm ở backend;
* với cookie auth, vẫn cần CSRF protection hoặc SameSite phù hợp.

Câu trả lời phỏng vấn:

> CORS chỉ là browser policy, không phải authentication. Tôi vẫn phải xác thực token/session ở backend. Production nên dùng allowlist origin theo environment; nếu gửi cookie thì không dùng wildcard và cần cấu hình SameSite/CSRF phù hợp.

---

## 8. API Key, Webhook Signature và Replay Attack

### API key

API key thường dùng cho server-to-server hoặc public developer API.

Nên có:

* prefix để nhận diện key, ví dụ `pk_live_`, `sk_live_`;
* lưu hash của API key, không lưu raw key;
* scope/permission rõ ràng;
* expiration/rotation;
* rate limit theo key;
* audit log key nào gọi endpoint nào.

### Webhook signature

Webhook cần xác minh request thật sự đến từ provider.

Pattern phổ biến:

```text
signature = HMAC_SHA256(secret, timestamp + "." + raw_body)
```

Backend nhận webhook:

1. Đọc raw body.
2. Kiểm tra timestamp còn trong cửa sổ hợp lệ, ví dụ 5 phút.
3. Tính lại HMAC bằng shared secret.
4. So sánh signature bằng constant-time compare.
5. Dùng event ID/idempotency key để chống xử lý trùng.

### Replay attack

Replay là attacker lấy lại request hợp lệ cũ và gửi lại. Phòng tránh bằng:

* timestamp ngắn hạn;
* nonce hoặc event ID;
* idempotency key;
* lưu processed event ID trong DB/Redis với TTL.

Câu trả lời phỏng vấn:

> Với webhook tôi không chỉ tin IP hay header. Tôi verify HMAC signature trên raw body, kiểm tra timestamp để chống replay và lưu event ID để xử lý idempotent vì webhook thường có retry và có thể gửi trùng.

---

## 9. File Upload Security

File upload là một trong những phần dễ lỗi nhất vì nó chạm nhiều lớp: HTTP body, storage, antivirus, metadata, authorization, CDN và background processing.

### Rủi ro phổ biến

* Upload file quá lớn làm đầy disk/memory hoặc giữ connection quá lâu.
* Giả mạo `Content-Type`, ví dụ gửi file executable nhưng khai là image.
* File extension nguy hiểm: `.php`, `.jsp`, `.sh`, `.exe`, `.html`, `.svg` chứa script.
* Path traversal qua filename như `../../etc/passwd`.
* Ghi đè file người khác nếu dùng filename gốc làm key.
* Public bucket vô tình lộ file riêng tư.
* Upload malware rồi phát tán qua CDN.
* Image processing bị tấn công bằng file ảnh lỗi hoặc decompression bomb.
* Metadata EXIF chứa thông tin nhạy cảm.
* User upload nhiều file để làm tăng storage cost.

### Nguyên tắc thiết kế an toàn

Không tin bất kỳ thông tin nào client gửi về file.

Backend cần kiểm tra:

* kích thước file tối đa;
* số lượng file trong một request;
* MIME type theo magic bytes, không chỉ theo header;
* extension trong allowlist;
* filename phải được normalize hoặc bỏ qua;
* quyền user có được upload vào resource đó không;
* quota theo user/tenant;
* virus/malware scan nếu file có thể được tải lại bởi người khác;
* image/video processing chạy trong worker cô lập, có timeout.

### Không dùng filename gốc làm storage key

Sai:

```text
uploads/{user_filename}
```

Đúng hơn:

```text
uploads/{tenant_id}/{yyyy}/{mm}/{uuid}.{safe_ext}
```

Filename gốc chỉ nên lưu như metadata để hiển thị, sau khi sanitize.

### Public vs private file

Không nên mặc định public mọi file upload.

* Public file: avatar, product image, asset có thể xem công khai.
* Private file: invoice, contract, user document, report, internal export.

Với private file:

* bucket/object nên private;
* tải file qua signed URL ngắn hạn;
* kiểm tra authorization trước khi cấp signed URL;
* không expose object key đoán được;
* log access với file nhạy cảm.

### Direct upload lên S3/Object Storage

Với file lớn, backend không nên nhận toàn bộ file rồi mới upload lên storage nếu không cần thiết. Pattern tốt hơn:

1. Client gọi backend xin upload URL.
2. Backend kiểm tra auth/quota/file metadata.
3. Backend tạo pre-signed upload URL với TTL ngắn, giới hạn size/content type nếu provider hỗ trợ.
4. Client upload trực tiếp lên object storage.
5. Storage phát event hoặc client gọi complete API.
6. Backend/worker scan file, validate lại metadata, tạo thumbnail nếu cần.
7. Chỉ mark file là `ready` sau khi scan/processing thành công.

Trạng thái nên có:

```text
pending_upload -> uploaded -> scanning -> ready
                              -> rejected
                              -> failed
```

Điểm quan trọng: file vừa upload xong chưa chắc đã được phép dùng ngay. Nếu chưa scan xong mà đã public URL, rủi ro rất lớn.

### Validate image upload

Với ảnh:

* kiểm tra magic bytes;
* decode ảnh bằng thư viện an toàn để chắc chắn file đọc được;
* giới hạn pixel count, không chỉ giới hạn byte size;
* strip EXIF nếu không cần;
* convert sang format an toàn như JPEG/WebP/PNG;
* tạo thumbnail ở worker với timeout;
* không phục vụ trực tiếp SVG user-upload nếu không sanitize kỹ vì SVG có thể chứa script.

### Request limit và streaming

Backend nên:

* đặt `MaxBytesReader` hoặc body size limit tương đương;
* dùng streaming thay vì đọc toàn bộ file vào memory;
* timeout upload;
* giới hạn concurrent upload;
* cleanup file tạm nếu request fail;
* không để multipart temp file lấp đầy disk.

### Authorization cho file

Không chỉ kiểm tra user đã login. Phải kiểm tra user có quyền với object cha hay không.

Ví dụ:

* user A không được upload ảnh vào product của tenant B;
* staff chỉ được upload report cho store mình quản lý;
* POS offline sync không được overwrite file của branch khác.

### Checklist trả lời phỏng vấn

Khi được hỏi "thiết kế upload file an toàn", có thể trả lời theo flow:

> Tôi không tin Content-Type hay filename từ client. Backend kiểm tra auth, quota, size limit, allowlist extension và MIME bằng magic bytes. Với file lớn tôi dùng pre-signed URL để upload trực tiếp lên S3, file ban đầu ở trạng thái pending/scanning. Worker sẽ scan virus, validate/decode ảnh, strip EXIF, tạo thumbnail rồi mới mark ready. Private file không public bucket mà cấp signed URL ngắn hạn sau khi kiểm tra quyền.

---

## 10. Secrets, Logs và PII

### Secrets

Không hardcode:

* DB password;
* JWT secret/private key;
* API key của provider;
* webhook secret;
* cloud access key.

Nên dùng:

* AWS Secrets Manager, SSM Parameter Store, Vault hoặc secret manager tương đương;
* phân quyền IAM tối thiểu;
* secret rotation;
* tách secret theo environment;
* không commit `.env` thật.

### Logging

Không log:

* password;
* access token/refresh token;
* API key;
* OTP;
* full credit card;
* private document URL;
* PII không cần thiết.

Nên log:

* request ID/correlation ID;
* user ID/tenant ID nếu phù hợp;
* auth failure reason ở mức an toàn;
* rate limit event;
* suspicious activity;
* webhook event ID;
* file upload status.

Câu trả lời phỏng vấn:

> Tôi coi log cũng là bề mặt rủi ro. Log nên đủ để debug và audit, nhưng phải redact token, password, API key và PII. Secrets thì lấy từ secret manager/IAM, không hardcode và không commit vào repo.

---

## 11. Security Checklist cho Backend Middle

Auth/session:

* JWT ngắn hạn, refresh token rotate được.
* Revoke token bằng `jti` blacklist hoặc token version.
* Cookie có `HttpOnly`, `Secure`, `SameSite`.
* Không lộ user enumeration ở login/register/reset password.

Authorization:

* Check quyền theo resource, không chỉ theo role.
* Tenant isolation rõ ràng.
* Admin/internal endpoint có guard riêng.

Input/API:

* Validate request body/query/path.
* Body size limit, timeout, pagination limit.
* CORS allowlist đúng.
* Idempotency cho write API dễ retry.

Database:

* Parameterized query.
* Allowlist sort/filter.
* DB user quyền tối thiểu.
* Không expose raw error SQL ra client.

Traffic abuse:

* Rate limit theo IP/user/API key/endpoint.
* Brute-force protection.
* Bot/scraping detection.
* WAF/CDN cho public app.

File upload:

* Size/quota/type validation.
* Không dùng filename gốc làm path.
* Private by default.
* Scan/process trước khi mark ready.
* Signed URL ngắn hạn cho private file.

Operation:

* Secret manager.
* Dependency scanning.
* Log redaction.
* Audit log cho hành động nhạy cảm.
* Alert khi login fail, rate limit, 5xx, latency spike.

---

## 12. Câu hỏi phỏng vấn hay gặp

### Rainbow table khác brute force thế nào?

Rainbow table dùng bảng hash tính sẵn để lookup nhanh password từ hash bị leak. Brute force là thử nhiều password để tìm match. Salt riêng từng password làm rainbow table mất hiệu quả; bcrypt/argon2 làm brute force tốn tài nguyên hơn.

### Rate limit theo IP có đủ chống brute force không?

Không. Attacker có thể dùng nhiều IP/proxy. Nên kết hợp IP, account, device fingerprint, user-agent pattern và challenge. Với account, cần tránh lock cứng gây DoS user thật.

### CORS có bảo vệ API khỏi Postman/curl không?

Không. CORS là browser policy. Backend vẫn phải xác thực và phân quyền mọi request.

### Làm sao chống scraping?

Giới hạn pagination, quota theo user/API key, rate limit theo endpoint, detect scan pattern, cache dữ liệu public, dùng challenge ở edge và không expose bulk export nếu không có quyền/async job.

### Upload file cần kiểm tra gì?

Kiểm tra auth, quyền resource, quota, size, extension allowlist, MIME bằng magic bytes, virus scan nếu file được chia sẻ, xử lý ảnh trong worker, không dùng filename gốc làm key, private file dùng signed URL ngắn hạn.

### DDoS thì backend làm được gì?

DDoS lớn cần WAF/CDN/provider, nhưng backend vẫn cần timeout, body limit, rate limit, concurrency limit, queue/backpressure, cache và alert để tránh app-level DDoS làm nghẽn worker, DB hoặc queue.
