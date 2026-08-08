# JWT Auth — Phase 1 + Phase 2

Implement theo [`idea.md`](./idea.md):

- **Phase 1** — Login, Access Token, Refresh Token, Middleware xác thực.
- **Phase 2** — Refresh Rotation (Bài 3), Token Version (Bài 4), Blacklist (Bài 5),
  Logout (Bài 8), Logout All (Bài 9).

Stack: Go 1.24 · Gin · GORM · PostgreSQL · Redis · React 18 + TypeScript (Vite).

---

## Chạy thử

```bash
cp .env.example .env        # rồi đổi JWT_SECRET: openssl rand -base64 48
make up                     # postgres :5433 + redis :6380
make api                    # backend :8080 (tự chạy migration lúc khởi động)

make install                # cài dependency FE (chỉ lần đầu)
make web                    # frontend :5173
```

Postgres/Redis dùng port 5433/6380 vì 5432/6379 thường đã bị service khác chiếm.

---

## API

| Method | Path                           | Auth | Mô tả                                              |
| ------ | ------------------------------ | :--: | -------------------------------------------------- |
| POST   | `/api/v1/auth/register`        |  —   | Tạo tài khoản, trả luôn cặp token                  |
| POST   | `/api/v1/auth/login`           |  —   | Đăng nhập                                          |
| POST   | `/api/v1/auth/refresh`         |  —   | Rotate: cấp cặp access + refresh mới               |
| POST   | `/api/v1/auth/logout`          |  ✓   | Xoá refresh token + blacklist access token         |
| POST   | `/api/v1/auth/logout-all`      |  ✓   | `token_version++`, thu hồi mọi phiên               |
| POST   | `/api/v1/auth/change-password` |  ✓   | Đổi mật khẩu, thu hồi mọi phiên, cấp lại token     |
| GET    | `/api/v1/me`                   |  ✓   | Thông tin user đang đăng nhập                      |
| GET    | `/api/v1/sessions`             |  ✓   | Danh sách thiết bị đang đăng nhập                  |
| GET    | `/health`                      |  —   | Health check                                       |

Access token gửi qua header `Authorization: Bearer <token>`.

Mã lỗi 401 mà client cần phân biệt:

| `error`                  | Ý nghĩa                            | Client nên làm       |
| ------------------------ | ---------------------------------- | -------------------- |
| `token_expired`          | Access token hết hạn               | gọi `/refresh`, retry |
| `token_revoked`          | Đã logout, jti nằm trong blacklist | về màn đăng nhập     |
| `token_version_mismatch` | Đã logout-all / đổi mật khẩu       | về màn đăng nhập     |

---

## Thiết kế

**Access token (15 phút)** — stateless. Middleware chỉ verify chữ ký + hạn, **không
query DB**. Đây chính là điểm khác biệt với session (Bài 1).

**Refresh token (7 ngày)** — stateful. `jti` của nó là key trong Redis
(`refresh:{jti}`, TTL = hạn token). Còn key = token còn sống → thu hồi được ngay.
PostgreSQL (`refresh_tokens`) chỉ lưu metadata để audit và liệt kê thiết bị.

**Lưu token ở FE** — access token nằm trong biến memory (mất khi reload, nhưng
XSS không đọc được qua `localStorage`); refresh token nằm trong `localStorage`
để giữ đăng nhập, đổi lại nó thu hồi được ở server.

**Auto-refresh** — khi API trả `401 token_expired`, HTTP client tự gọi
`/auth/refresh` rồi retry đúng request đó một lần. Nhiều request 401 cùng lúc
được gộp vào một lần refresh duy nhất.

**Rotation (Bài 3)** — mỗi lần `/refresh`, jti cũ bị `DEL` khỏi Redis trước khi
phát jti mới. Refresh token vì thế dùng được đúng một lần: kẻ trộm dùng lại
token cũ chỉ nhận 401. Xoá trước rồi mới phát mới — nếu bước sau lỗi thì user
phải đăng nhập lại, chấp nhận được; ngược lại sẽ có cửa sổ hai refresh token
cùng hiệu lực.

**Blacklist (Bài 5, 8)** — logout ghi `blacklist:{jti}` với TTL đúng bằng thời
gian sống còn lại của access token, nên blacklist không phình vô hạn. Không có
bước này thì access token vẫn sống thêm tới 15 phút sau khi bấm "đăng xuất".

**Token version (Bài 4, 9)** — `users.token_version` được nhét vào claim `ver`.
Logout-all chỉ cần `token_version++` là mọi access token đang lưu hành chết
theo, không cần biết chúng là những jti nào.

**Cái giá của khả năng thu hồi** — Bài 1 nói middleware không cần chạm DB, nhưng
muốn logout có hiệu lực tức thì thì phải tra state. Ở đây state nằm ở Redis
(1 `EXISTS` blacklist + 1 `GET` version); PostgreSQL chỉ bị đụng khi cache
`user:{id}:ver` miss. Mọi thao tác đổi `token_version` đều xoá cache ngay, nên
thu hồi không phải chờ TTL: request kế tiếp miss và đọc version mới từ DB. Xoá
chứ không ghi đè vì cache rỗng chỉ tốn thêm một lượt đọc DB, còn cache giữ
version cũ thì token đã thu hồi vẫn qua được. Redis chết thì middleware trả 503
chứ không cho qua — fail closed.

```
Login   ──►  access (memory) + refresh (localStorage) + Redis refresh:{jti}

Request ──►  Bearer access
              │ 1. verify signature + exp      (stateless)
              │ 2. EXISTS blacklist:{jti}      (Redis)
              │ 3. ver == token_version?       (Redis cache → DB khi miss)
              └──►  handler
                    │ 401 token_expired          → refresh rồi retry
                    │ 401 token_revoked          → về màn login
                    └ 401 token_version_mismatch → về màn login

Refresh ──►  DEL refresh:{jti cũ}  ──►  SET refresh:{jti mới}  ──►  cặp token mới

Logout  ──►  SET blacklist:{access jti} EX <hạn còn lại>
             DEL refresh:{jti}  +  revoked_at trong DB

LogoutAll ──►  token_version++  ──►  DEL cache user:{id}:ver
               DEL toàn bộ refresh:{jti} của user (qua set user:{id}:refresh)
```

### Redis keys

| Key                    | Kiểu   | TTL                   | Dùng để                       |
| ---------------------- | ------ | --------------------- | ----------------------------- |
| `refresh:{jti}`        | hash   | = hạn refresh token   | refresh token còn sống        |
| `user:{id}:refresh`    | set    | = hạn refresh token   | logout-all không cần `SCAN`   |
| `blacklist:{jti}`      | string | = hạn còn lại access  | access token bị thu hồi sớm   |
| `user:{id}:ver`        | string | 10 phút               | cache `token_version`         |

---

## Cấu trúc

```
cmd/api/               entrypoint, wiring, graceful shutdown
internal/auth/
    handler/           HTTP <-> service, map lỗi domain sang status code
    service/           business logic
    repository/        users, refresh_tokens (Postgres) + token store (Redis)
    middleware/        RequireAuth
    model/             entity + DTO
internal/user/         User entity
pkg/                   jwt, redis, postgres, password, config
migrations/            SQL, nhúng vào binary bằng go:embed
web/                   React + TypeScript (Vite)
```

---

## Đã kiểm chứng

**Phase 1**

- `go test ./...` — 6 test cho `pkg/jwt` (round-trip, sai type, hết hạn, sai
  secret, `alg=none`, jti duy nhất).
- Browser: đăng ký → dashboard → `GET /me` 200 → reload giữ nguyên phiên → logout.
- Auto-refresh: hạ `ACCESS_TOKEN_TTL=5s`, đợi hết hạn, gọi `/me` → network log
  cho thấy `401 → POST /auth/refresh 200 → GET /me 200`.

**Phase 2**

- Rotation: refresh trả token khác token cũ; token cũ → 401; token mới → 200.
- Blacklist: sau logout, `blacklist:{jti}` tồn tại với TTL 899s, `/me` trả
  `401 token_revoked` ngay lập tức.
- Logout-all: `token_version` 1 → 2; access token của cả macbook lẫn ipad đều
  `401 token_version_mismatch`; set `user:{id}:refresh` bị xoá (4 member → 0),
  4/4 hàng `refresh_tokens` được đánh dấu `revoked_at`.
- Change-password: sai mật khẩu cũ → 401; thiết bị đổi mật khẩu vẫn 200; thiết
  bị khác bị đá ra; mật khẩu cũ không đăng nhập được nữa.
- Browser: rotate hiển thị `jti cũ → jti mới`, "dùng lại token cũ" → 401, đổi
  mật khẩu đẩy `ver` 1 → 2, và khi logout-all từ thiết bị khác thì tab đang mở
  tự về màn đăng nhập ở request kế tiếp (đúng một 401, không có vòng lặp refresh).

---

## Chưa có (theo roadmap)

| Phase | Nội dung                                                                 |
| ----- | ------------------------------------------------------------------------ |
| 3     | RS256 + key rotation (`kid`), multi-device, RBAC                         |
| 4     | Rate limit `/login` `/refresh`, phát hiện refresh token reuse, audit log |

Các điểm mở rộng đã chừa sẵn chỗ trong code:

- `pkg/jwt/jwt.go` — đổi `SigningMethodHS256` sang RS256 là xong Bài 10.
- `internal/auth/service/token_guard.go` — thêm chốt chặn mới (RBAC, `kid`) vào
  đúng một chỗ, middleware không phải sửa.
- `refresh_tokens` đã lưu `device_id` + `user_agent` + `ip`, đủ cho Bài 7.
- Rotation đã xoá jti cũ; Phase 4 chỉ cần thêm: jti cũ bị dùng lại → coi là
  token bị đánh cắp → logout-all cả user.
