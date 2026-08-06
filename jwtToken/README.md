# JWT Auth — Phase 1

Implement **Phase 1** của [`idea.md`](./idea.md): Login, Access Token, Refresh Token, Middleware xác thực.

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

| Method | Path                 | Auth | Mô tả                                     |
| ------ | -------------------- | :--: | ----------------------------------------- |
| POST   | `/api/v1/auth/register` |  —   | Tạo tài khoản, trả luôn cặp token         |
| POST   | `/api/v1/auth/login`    |  —   | Đăng nhập                                 |
| POST   | `/api/v1/auth/refresh`  |  —   | Đổi refresh token lấy access token mới    |
| POST   | `/api/v1/auth/logout`   |  —   | Xoá refresh token của thiết bị hiện tại   |
| GET    | `/api/v1/me`            |  ✓   | Thông tin user đang đăng nhập             |
| GET    | `/api/v1/sessions`      |  ✓   | Danh sách thiết bị đang đăng nhập         |
| GET    | `/health`               |  —   | Health check                              |

Access token gửi qua header `Authorization: Bearer <token>`.

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

```
Login  ──►  access (memory) + refresh (localStorage) + Redis refresh:{jti}
Request ──►  Bearer access ──►  verify signature ──►  handler
                    │ 401 token_expired
                    └──►  POST /auth/refresh ──►  access mới ──►  retry
Logout ──►  DEL refresh:{jti}  +  revoked_at trong DB
```

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

- `go test ./...` — 6 test cho `pkg/jwt` (round-trip, sai type, hết hạn, sai
  secret, `alg=none`, jti duy nhất).
- End-to-end qua trình duyệt: đăng ký → dashboard → `GET /me` 200 → refresh →
  reload giữ nguyên phiên → logout → refresh token cũ bị từ chối 401.
- Auto-refresh: hạ `ACCESS_TOKEN_TTL=5s`, đợi token hết hạn, gọi `/me` →
  network log cho thấy `401 → POST /auth/refresh 200 → GET /me 200`.

---

## Chưa có (theo roadmap)

| Phase | Nội dung                                                              |
| ----- | --------------------------------------------------------------------- |
| 2     | Refresh token rotation, blacklist access token, logout-all (`token_version`) |
| 3     | RS256 + key rotation (`kid`), RBAC                                    |
| 4     | Rate limit `/login` `/refresh`, phát hiện refresh token reuse, audit log |

Các điểm mở rộng đã chừa sẵn chỗ trong code:

- `pkg/jwt/jwt.go` — đổi `SigningMethodHS256` sang RS256 là xong Bài 10.
- `internal/auth/middleware/auth.go` — có comment đánh dấu chỗ chèn check
  blacklist và `token_version`.
- Redis đã có set `user:{id}:refresh` để logout-all không cần `SCAN`.
- Cột `users.token_version` đã tồn tại và đã được nhét vào claim `ver`.
