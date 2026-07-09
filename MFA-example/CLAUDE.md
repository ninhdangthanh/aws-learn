# CLAUDE.md — MFA-example

Demo MFA đầy đủ: password (yếu tố 1) + TOTP hoặc Passkey/WebAuthn (yếu tố 2).
Đây là project **học/demo**, không phải code production.

> `README.md` mô tả kiến trúc và các sequence diagram — đọc nó để hiểu ý đồ.
> File này chỉ ghi những gì không suy ra được từ code, và những chỗ code đang sai.

---

## Chạy

```bash
cd backend
npm install
node server.js  # http://localhost:3000
```

Express phục vụ luôn `frontend/` qua `express.static`, nên **không có dev server riêng cho frontend**.
Không có test, không có linter, không có build step. Sửa file là chạy lại `node server.js`.

---

## Bố cục

| Đường dẫn | Vai trò |
|---|---|
| `backend/server.js` | Toàn bộ backend trong 1 file: routes + store + logic MFA |
| `frontend/index.html` | 5 view (`.view`), chuyển view bằng class `active` |
| `frontend/app.js` | State + fetch + WebAuthn, vanilla JS, không module/bundler |
| `frontend/styles.css` | Style |

`frontend/app.js` load qua `<script src>` thường (không phải ESM), và phụ thuộc biến toàn cục
`SimpleWebAuthnBrowser` do `<script>` unpkg đặt vào. `logout()` được gọi từ `onclick=` trong HTML
nên **phải giữ nó ở global scope** — đừng bọc file vào IIFE/module.

---

## Mô hình xác thực — thứ cần nắm trước khi sửa bất cứ gì

Có **ba** loại trạng thái, và **hai** loại token khác nhau. Nhầm hai token này là lỗi hay gặp nhất.

| Token | Nằm ở | Nghĩa | Dùng cho |
|---|---|---|---|
| `tempToken` | `tempSessions[tempToken] = username` | Đã đúng password, **chưa** qua MFA | Chỉ để nộp MFA |
| `token` | `sessions[token] = username` | Đã xác thực đầy đủ | Mọi API còn lại |

Luồng: `POST /api/login` kiểm tra password rồi rẽ nhánh theo việc user **đã đăng ký MFA chưa**:

- Đã có MFA (`mfaEnabled || passkeys.length > 0`) → trả `{ requireMfa: true, tempToken, hasTotp, hasPasskey }`.
- Chưa có MFA → trả thẳng `{ requireMfa: false, token }` (session **đầy đủ**).

Điểm dễ hiểu sai: user chưa đăng ký MFA vẫn nhận được `token` đầy đủ. Cửa chặn không nằm ở login mà
nằm ở `GET /api/dashboard` — nó trả **403** nếu `!mfaEnabled && passkeys.length === 0`. Tức là
"có token" ≠ "được vào dashboard". Đừng "sửa" login để chặn ở đó; thiết kế cố ý đẩy user sang màn
hình `view-security-setup` để bắt đăng ký MFA.

---

## API và cách truyền credential (KHÔNG nhất quán — đây là thực tế của code)

| Endpoint | Credential truyền ở đâu |
|---|---|
| `POST /api/register` | — |
| `POST /api/login` | — |
| `POST /api/mfa/generate` | `token` trong **body** |
| `POST /api/mfa/verify` | `token` **hoặc** `tempToken` trong **body** |
| `POST /api/passkey/generate-registration-options` | `Authorization: Bearer` |
| `POST /api/passkey/verify-registration` | `Authorization: Bearer` |
| `POST /api/passkey/generate-authentication-options` | `tempToken` trong **body** |
| `POST /api/passkey/verify-authentication` | `tempToken` trong **body** |
| `GET /api/dashboard` | `Authorization: Bearer` + phải đã đăng ký MFA |
| `GET /api/me` | `Authorization: Bearer` |

Nhóm TOTP dùng body, nhóm passkey-setup dùng header. Khi thêm endpoint mới, **theo nhóm tương ứng**
thay vì tự chọn, nếu không frontend sẽ lệch.

`/api/mfa/verify` phân biệt hai pha bằng việc field nào có mặt: có `tempToken` → pha đăng nhập;
có `token` → pha thiết lập lần đầu (đặt `mfaEnabled = true`).

---

## Ràng buộc bất biến

- **Store nằm trong RAM.** `users`, `sessions`, `tempSessions` mất sạch khi restart server. Test thủ công
  phải đăng ký lại từ đầu sau mỗi lần restart.
- **`rpID` và `origin` phải khớp nhau.** `rpID = 'localhost'`, `origin = 'http://localhost:3000'`.
  Nếu mở app qua `127.0.0.1` thay vì `localhost`, WebAuthn sẽ fail vì origin không khớp. Đổi cổng thì
  phải đổi cả `origin` (dòng 21–23 `server.js`) và `PORT` (cuối file).
- **Password lưu plaintext** (`server.js:48`, có comment "In a real app, hash this!"). Đây là chủ ý của
  demo để giữ code ngắn. **Đừng tự ý thêm bcrypt** trừ khi được yêu cầu rõ ràng.
- **`counter` của passkey** được cập nhật sau mỗi lần xác thực để chống replay. Đừng bỏ dòng
  `passkey.counter = authenticationInfo.newCounter`.
- `user.currentChallenge` bị **ghi đè** giữa registration và authentication (dùng chung một field).
  Nghĩa là không thể chạy song song hai luồng WebAuthn cho cùng một user.

---

## Ghim version: cả hai đầu phải ở SimpleWebAuthn v9

Backend ghim `@simplewebauthn/server@9.0.3` (`backend/package.json`), và frontend **phải** ghim
major tương ứng — `frontend/index.html:189`:

```html
<script src="https://unpkg.com/@simplewebauthn/browser@9/dist/bundle/index.umd.min.js"></script>
```

Bỏ `@9` khỏi URL là unpkg trả bản mới nhất (hiện là 13.x). Từ v11, `startRegistration` /
`startAuthentication` đổi chữ ký sang `{ optionsJSON }`, còn `app.js` vẫn truyền options trực tiếp
theo kiểu v9 → passkey hỏng ngay. **Nâng version thì phải nâng cả hai đầu cùng lúc.**

Ba điểm dưới đây từng là lỗi, đã sửa — ghi lại để đừng vô tình quay về code cũ:

- **`await` là bắt buộc.** Ở v9, `generateRegistrationOptions` và `generateAuthenticationOptions`
  đều trả `Promise`. Quên `await` thì `options.challenge` là `undefined` và `res.json(options)`
  serialize Promise thành `{}`. Xem `server.js:183` và `server.js:264` — hai route đó phải là `async`.
- **So khớp `credentialID` bằng `isoBase64URL.fromBuffer()`** (`server.js:290`). v9 trả
  `credentialID: Uint8Array`, mà `Uint8Array.prototype.toString('base64url')` **bỏ qua** tham số và
  trả `"1,2,3,..."` → không bao giờ khớp `response.id`.
- Phép so ở `server.js:236` (dedupe lúc đăng ký) dùng `.toString()` cả hai vế nên vẫn đúng — cùng
  định dạng chuỗi dấu phẩy. Không cần sửa, nhưng đừng nhầm nó với trường hợp trên.

Luồng TOTP và passkey đã chạy: `generate-registration-options` và `generate-authentication-options`
trả `challenge` thật, đăng ký TOTP → đăng nhập → `/api/dashboard` trả 200.

---

## Hooks đang bật cho thư mục này

`.claude/settings.json` có 4 hook. Chi tiết trong [`HOOKS.md`](./HOOKS.md). Hai cái ảnh hưởng trực tiếp
tới việc sửa code ở đây:

- **guard-secrets** sẽ **từ chối** thao tác `Write`/`Edit` nếu nội dung chứa secret viết cứng
  (password, api key, TOTP secret base32, private key). Dùng `process.env.X` hoặc sinh runtime.
- **check-js-syntax** chạy `node --check` sau mỗi lần sửa `.js`; lỗi cú pháp sẽ được báo lại ngay.

Hook chỉ áp dụng cho file **bên trong** `MFA-example/`.
