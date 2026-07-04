# JWT And Session Middle Backend Notes

File này gom kiến thức JWT/session ở mức Middle Backend: access token, refresh token, blacklist, token versioning, rotation, revoke, multi-device session và các trade-off production.

JWT không chỉ là "ký token rồi verify". Production auth cần nghĩ về revoke, logout, đổi mật khẩu, refresh token bị leak, multi-device session, key rotation và Redis/DB trade-off.

---

## 1. JWT Là Gì?

JWT là token có 3 phần:

```text
header.payload.signature
```

Payload thường chứa claims:

```json
{
  "sub": "user_123",
  "jti": "token_abc",
  "token_version": 7,
  "session_id": "sess_456",
  "iat": 1760000000,
  "exp": 1760000900,
  "iss": "auth-service",
  "aud": "api"
}
```

Ý nghĩa claim hay gặp:

| Claim | Ý nghĩa |
|---|---|
| `sub` | Subject, thường là user id |
| `jti` | JWT ID, unique id của token |
| `iat` | Issued at |
| `exp` | Expiration time |
| `iss` | Issuer |
| `aud` | Audience |
| `session_id` | Phiên đăng nhập/device session |
| `token_version` | Version để revoke token hàng loạt |

Lưu ý: JWT payload thường chỉ base64url encoded, không phải encrypted. Không nhét password, secret, API key hoặc PII nhạy cảm vào payload.

---

## 2. Access Token Và Refresh Token

### Access token

Access token dùng để gọi API.

Best practice:

* TTL ngắn, thường 5-15 phút.
* Dạng JWT để resource server verify nhanh.
* Chứa claims tối thiểu: user id, session id, scope/role cần thiết, token version.

### Refresh token

Refresh token dùng để lấy access token mới.

Best practice:

* TTL dài hơn, ví dụ ngày/tuần.
* Lưu server-side state trong DB/Redis để revoke/rotate.
* Nên lưu hash của refresh token, không lưu raw token.
* Lưu bằng HttpOnly Secure cookie nếu browser app phù hợp.

Flow:

```text
Login
-> issue access_token ngắn hạn
-> issue refresh_token dài hạn

Access token hết hạn
-> client gửi refresh_token
-> server verify refresh token
-> issue access_token mới
```

---

## 3. Stateless JWT vs Stateful Session vs Hybrid

### Stateless JWT thuần

Server chỉ verify signature và `exp`.

Ưu điểm:

* Không cần lookup DB/Redis mỗi request.
* Scale ngang dễ.
* Hợp microservices/API gateway.

Nhược điểm:

* Khó revoke token trước khi hết hạn.
* User đổi password/logout vẫn có thể còn token cũ sống tới `exp`.
* Claim thay đổi chậm nếu token TTL dài.

### Stateful session

Server lưu session trong DB/Redis.

```text
session:{session_id} -> user_id, device_id, expires_at, revoked=false
```

Ưu điểm:

* Revoke dễ.
* Kiểm soát multi-device tốt.
* Có audit/session metadata.

Nhược điểm:

* Mỗi request cần lookup state.
* Redis/DB trở thành dependency auth path.

### Hybrid

Access token là JWT ngắn hạn, nhưng server vẫn giữ một số state để revoke khi cần:

* `jti` blacklist trong Redis;
* `token_version` trong DB/cache;
* session table để quản lý device;
* refresh token rotation state.

Đây là pattern thực tế hơn cho production.

---

## 4. JWT Blacklist Theo `jti`

Dùng khi access token là JWT nhưng vẫn cần revoke trước khi hết hạn.

Use case:

* logout current device;
* ban account;
* phát hiện token leak;
* revoke một access token cụ thể.

Pattern Redis:

```text
blacklist:jti:{jti} -> 1
TTL = exp(token) - now
```

Request validation:

```text
verify signature
check exp/iss/aud
extract jti
GET blacklist:jti:{jti}
  nếu tồn tại -> reject
  nếu không -> continue
```

Vì mỗi token có TTL riêng, nên dùng `String` key có TTL riêng tốt hơn gom nhiều `jti` vào một `Set`.

Trade-off:

* Mỗi request phải check Redis, JWT không còn thuần stateless.
* Nếu Redis down, phải có policy fail open hay fail closed.
* Nếu Redis eviction làm mất blacklist key, có thể thành security bug.
* Token TTL ngắn giúp blacklist tự nhỏ lại.

---

## 5. Token Versioning

Token versioning dùng để revoke hàng loạt token cũ.

DB:

```text
users
id
token_version
password_changed_at
```

JWT payload:

```json
{
  "sub": "user_123",
  "token_version": 7
}
```

Validation:

```text
verify JWT
load user.token_version từ DB/cache
if token.token_version < user.token_version -> reject
```

Use case:

* logout all devices;
* đổi password;
* admin revoke user;
* role/permission thay đổi nghiêm trọng.

Trade-off:

* Cần lookup DB/cache mỗi request hoặc cache user version.
* Revoke được theo user/global version, không revoke chính xác một token riêng như blacklist `jti`.
* Nếu cache stale, token cũ có thể còn sống thêm một khoảng ngắn.

So sánh nhanh:

| Cách | Hợp với | Trade-off |
|---|---|---|
| `jti` blacklist | Revoke một token cụ thể | Redis lookup, nhiều key |
| `token_version` | Logout all/đổi password/revoke user | DB/cache lookup, revoke theo nhóm |
| Access token TTL ngắn | Giảm rủi ro token cũ | User cần refresh thường xuyên |

---

## 6. Refresh Token Rotation

Refresh token rotation nghĩa là mỗi lần refresh, server cấp refresh token mới và vô hiệu hóa token cũ.

Flow:

```text
client gửi refresh_token_A
server verify A còn active
server mark A used/revoked
server issue access_token mới + refresh_token_B
client lưu B
```

Nếu refresh token A bị dùng lại:

```text
refresh_token_A đã used
-> reuse detected
-> revoke toàn bộ session family
-> bắt user login lại
```

Schema mẫu:

```sql
CREATE TABLE refresh_tokens (
    token_id TEXT PRIMARY KEY,
    token_hash TEXT NOT NULL,
    user_id TEXT NOT NULL,
    session_id TEXT NOT NULL,
    family_id TEXT NOT NULL,
    used_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

Điểm quan trọng:

* Không lưu raw refresh token.
* Rotation phải atomic để tránh race khi client refresh song song.
* Reuse detection nên revoke cả `family_id`.
* Log/audit khi reuse xảy ra.

---

## 7. Multi-Device Session

Một user có nhiều session/device.

Schema ý tưởng:

```sql
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    device_id TEXT,
    user_agent TEXT,
    ip_address TEXT,
    last_active_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL
);
```

JWT có thể chứa:

```json
{
  "sub": "user_123",
  "session_id": "sess_456",
  "token_version": 7
}
```

Use case:

* logout current device: revoke `session_id`;
* logout all devices: tăng `token_version` hoặc revoke toàn bộ sessions;
* show active devices;
* detect suspicious login.

Trade-off:

| Lựa chọn | Ưu điểm | Rủi ro/chi phí |
|---|---|---|
| Lưu session trong DB | Audit tốt, query lịch sử/device rõ, source of truth bền | Mỗi request lookup DB sẽ nặng nếu không cache |
| Cache session active trong Redis | Check nhanh, TTL tự hết hạn | Redis mất/evict key có thể logout nhầm hoặc tạo rủi ro nếu fail open |
| JWT chứa `session_id` | Revoke được từng device/session | Cần lookup session state nếu muốn revoke tức thì |
| Chỉ dùng `token_version` | Logout all devices đơn giản | Không revoke riêng một device/token cụ thể |

Pattern thực tế thường là DB làm source of truth cho sessions, Redis cache session/token version cho auth path nhanh hơn.

---

## 8. Blacklist vs Whitelist Session

### Blacklist

Mặc định token hợp lệ, chỉ reject token bị revoke.

Hợp khi:

* access token TTL ngắn;
* số token bị revoke ít;
* muốn giữ phần lớn lợi ích stateless.

### Whitelist/session lookup

Mỗi request phải check session còn active.

Hợp khi:

* cần revoke tức thì;
* security cao;
* cần quản lý device/session chặt.

Trade-off:

| Cách | Ưu điểm | Rủi ro/chi phí |
|---|---|---|
| Blacklist | Nhẹ hơn, giữ phần lớn lợi ích stateless của JWT | Revoke phụ thuộc blacklist key còn tồn tại; Redis eviction/down cần policy rõ |
| Whitelist/session lookup | Kiểm soát chặt, revoke tức thì, quản lý device tốt | Mỗi request phụ thuộc DB/Redis; tăng latency và dependency auth path |
| TTL access token ngắn | Giảm nhu cầu blacklist nhiều token | User/client phải refresh thường xuyên hơn |
| Kết hợp blacklist + token version + session | Linh hoạt cho revoke từng token, từng device, hoặc toàn user | Phức tạp hơn, cần design cache/DB consistency và observability |

Cách chọn nhanh:

* API bình thường, token TTL ngắn, revoke ít: blacklist theo `jti` là đủ.
* App cần quản lý thiết bị/logout từng device: session lookup hoặc `session_id` trong JWT.
* Đổi mật khẩu/logout all devices: `token_version`.
* Hệ thống security cao: whitelist/session lookup, chấp nhận thêm dependency.

---

## 9. Redis Và DB Nên Chia Vai Trò Thế Nào?

Redis hợp cho:

* blacklist `jti` với TTL;
* cache `user_id -> token_version`;
* cache session active;
* rate limit login;
* short-lived auth state.

DB hợp cho:

* refresh token source of truth;
* session audit;
* device management;
* password_changed_at;
* security forensic.

Rule thực tế:

> Redis giúp auth path nhanh hơn, nhưng dữ liệu security quan trọng cần DB source of truth hoặc policy rõ nếu Redis mất data.

---

## 10. Token Storage Ở Client

Browser:

* HttpOnly Secure cookie giảm rủi ro token bị JavaScript đọc khi XSS.
* Nếu dùng cookie, cần SameSite/CSRF protection phù hợp.
* LocalStorage dễ bị XSS đọc token hơn.

Mobile:

* dùng secure storage/keychain/keystore;
* refresh token bảo vệ kỹ hơn access token;
* có cơ chế revoke khi device lost nếu cần.

Backend không nên log access token/refresh token raw.

---

## 11. Key Rotation

JWT ký bằng secret/private key. Production cần nghĩ key rotation.

Pattern:

* JWT header có `kid`.
* Server giữ nhiều public keys/secret versions.
* Token mới ký bằng key mới.
* Token cũ vẫn verify bằng key cũ tới khi hết TTL.
* Sau khi token cũ hết hạn, retire key cũ.

Nếu key leak:

* rotate key ngay;
* revoke session/token nếu cần;
* audit log;
* giảm TTL access token giúp giảm blast radius.

---

## 12. Lỗi Thiết Kế Thường Gặp

* Access token TTL quá dài.
* Nghĩ JWT stateless nhưng vẫn muốn revoke tức thì mà không có blacklist/version.
* Lưu raw refresh token trong DB.
* Không rotate refresh token.
* Không detect refresh token reuse.
* Không có `jti`, khó blacklist token cụ thể.
* Không có `iss`/`aud`, token có thể bị dùng nhầm service.
* Lưu role/permission lâu trong JWT nhưng không có strategy khi permission đổi.
* Log token raw.
* Redis blacklist bị eviction nhưng không có cảnh báo/policy.
* Dùng cùng secret ở mọi môi trường.

---

## 13. Câu Trả Lời Phỏng Vấn Mẫu

### JWT stateless có revoke được không?

> JWT thuần stateless thì khó revoke trước khi hết hạn, vì server chỉ verify signature và exp. Production thường dùng access token TTL ngắn kết hợp refresh token rotation, hoặc thêm state như `jti` blacklist trong Redis, `token_version` trong DB/cache, hoặc session whitelist nếu cần revoke tức thì.

### JWT blacklist dùng Redis thế nào?

> Mỗi JWT nên có `jti`. Khi revoke token, set key `blacklist:jti:{jti} = 1` với TTL bằng thời gian còn lại của token. Mỗi request verify signature/exp xong sẽ check Redis. Dùng String key có TTL riêng tốt hơn Set vì mỗi token hết hạn khác nhau.

### Token versioning dùng khi nào?

> Token versioning hợp để logout all devices, đổi password hoặc admin revoke user. JWT chứa `token_version`; backend so với version hiện tại trong DB/cache. Nếu token mang version thấp hơn thì reject. Nó revoke theo nhóm token của user, còn `jti` blacklist revoke chính xác từng token.

### Refresh token rotation là gì?

> Mỗi lần dùng refresh token, server cấp refresh token mới và vô hiệu hóa token cũ. Nếu token cũ bị dùng lại, đó là dấu hiệu token leak, nên revoke cả session family và bắt user đăng nhập lại. Refresh token nên lưu hash trong DB, không lưu raw token.

### Dùng blacklist hay token version?

> Nếu cần revoke một token cụ thể như logout current session, dùng `jti` blacklist hoặc session revoke. Nếu cần revoke toàn bộ token của user như đổi password/logout all, dùng `token_version`. Thực tế thường kết hợp access token ngắn hạn, refresh token rotation, blacklist/session cho revoke cụ thể và token version cho revoke hàng loạt.
