* Golang
* PostgreSQL
* Redis
* JWT (RS256)
* Gin hoặc Fiber
* sqlc/GORM (đều được)
* Clean Architecture

---

# Tổng architecture

```text
                    +------------------+
                    |      Client      |
                    +------------------+
                              |
                 Authorization: Bearer
                              |
                     +----------------+
                     |   API Gateway  |
                     +----------------+
                              |
                     +----------------+
                     | Auth Service   |
                     +----------------+
                      |            |
              PostgreSQL      Redis
```

Auth Service sẽ chịu trách nhiệm

* Login
* Refresh
* Logout
* Logout All
* Verify JWT
* Change Password
* Key Rotation
* Permission

---

# Folder structure

```text
cmd/

internal/

    auth/
        handler/
        service/
        repository/
        middleware/
        model/

    user/

pkg/

    jwt/
    redis/
    postgres/
    password/
    crypto/
    config/

keys/

    private.pem
    public.pem

migrations/
```

---

# Database

## users

```sql
id

email

password_hash

token_version

created_at
```

token_version

chính là Logout All.

---

## refresh_tokens

```sql
id

user_id

jti

device_id

expires_at

created_at
```

Lưu metadata để audit, quản lý thiết bị và hỗ trợ đăng xuất từng thiết bị. Giá trị refresh token thực tế có thể chỉ lưu trong Redis hoặc lưu bản băm (hash) nếu muốn tăng bảo mật.

---

# Redis

```
refresh:{jti}

value

user_id

device

expire
```

TTL

=

refresh expire.

---

# API

```text
POST /login

POST /refresh

POST /logout

POST /logout-all

POST /change-password

GET /me
```

---

# Bài 1

## JWT vs Session

Implement

Login

↓

Generate JWT

↓

Verify JWT Middleware

Bạn sẽ hiểu

* JWT không cần query DB mỗi request
* chỉ verify signature

---

# Bài 2

## Access + Refresh

Login

↓

Generate

```
Access

Refresh
```

Access

15 phút

Refresh

7 ngày

Refresh API

↓

Verify Refresh

↓

Generate Access mới

---

# Bài 3

## Refresh Rotation

Login

↓

```
Refresh A
```

Refresh

↓

Delete

```
Refresh A
```

↓

Generate

```
Refresh B
```

Redis

```
DEL refresh:A

SET refresh:B
```

Nếu hacker

```
Refresh A
```

↓

401

---

# Bài 4

## Token Version

JWT

```
sub

version
```

Middleware

```
Verify Signature

↓

Read version in JWT

↓

Read token_version DB

↓

Compare
```

Nếu

```
JWT=3

DB=4
```

↓

Reject.

---

# Bài 5

## Blacklist

Redis

```
blacklist:{jti}
```

TTL

=

expire.

Middleware

```
Verify JWT

↓

Check blacklist

↓

Reject
```

---

# Bài 6

## Whitelist

Redis

```
access:{jti}
```

Middleware

```
JWT

↓

Redis

↓

Exists?

↓

OK
```

So sánh:

Blacklist lưu token bị cấm.

Whitelist chỉ chấp nhận token còn tồn tại.

---

# Bài 7

## Multi Device

Redis

```
user:10

iphone

macbook

ipad
```

Refresh

mỗi device

↓

một JTI riêng.

Logout iPhone

↓

Delete

```
refresh:iphone
```

MacBook

không ảnh hưởng.

---

# Bài 8

## Logout

Flow

```
Delete Refresh

↓

Blacklist Access

↓

Done
```

---

# Bài 9

## Logout All

Update

```
token_version++
```

Delete

```
refresh:user
```

Tất cả Access Token sẽ bị từ chối ở lần kiểm tra tiếp theo vì `token_version` không còn khớp.

---

# Bài 10

## HS256 → RS256

Tạo

```
private.pem

public.pem
```

Login

↓

Private Sign

Middleware

↓

Public Verify

Sau đó

tách

```
Auth Service

↓

Product Service

↓

Order Service
```

Tất cả service

Verify

bằng

```
public.pem
```

---

# Bài 11

## Key Rotation

JWT Header

```
kid=2
```

Server

```
kid

↓

Load Public Key

↓

Verify
```

Trong memory

```
Key1

Key2

Key3
```

Private Key mới

↓

Sign

Public Key cũ

↓

vẫn Verify được token cũ.

---

# Bài 12

## Authorization

Middleware

↓

JWT

↓

User

↓

Permission

Ví dụ

```
admin

manager

cashier
```

Middleware

```
RequireRole(admin)
```

hoặc

```
RequirePermission(product:create)
```

---

# Giai đoạn cuối: kết hợp tất cả

Khi hoàn thành, flow sẽ như sau:

```text
Login
│
├── Verify password
├── Generate Access Token (RS256)
├── Generate Refresh Token
├── Save Refresh Token (Redis)
└── Return tokens

↓

Authenticated Request

Verify Signature
        │
Check Blacklist
        │
Check Token Version
        │
Load User
        │
Authorization
        │
Business Logic

↓

Refresh

Verify Refresh
        │
Check Redis
        │
Delete old Refresh
        │
Generate new Refresh
        │
Generate new Access
        │
Return

↓

Logout

Delete Refresh
        │
Blacklist Access

↓

Logout All

token_version++
        │
Delete all Refresh Tokens
```

## Nếu muốn hiểu sâu như một backend middle/senior, mình khuyên làm theo roadmap này:

* **Phase 1 (Cơ bản):** Login, Access Token, Refresh Token, Middleware xác thực.
* **Phase 2 (Security):** Refresh Token Rotation, `jti`, Blacklist, Logout, Logout All.
* **Phase 3 (Production):** RS256, Key Rotation (`kid`), Multi-device Login, Authorization (RBAC).
* **Phase 4 (Nâng cao):** Rate limiting cho `/login` và `/refresh`, phát hiện refresh token bị tái sử dụng (refresh token reuse detection), audit log, thiết kế theo hướng microservice với Auth Service phát hành token và các service khác chỉ verify bằng public key.
