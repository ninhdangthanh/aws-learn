# Hướng dẫn Passkey Example — Flow chạy từ A đến Z

Tài liệu này giải thích **từng bước** ứng dụng chạy như thế nào: từ lúc mở UI, frontend gọi API nào, và backend xử lý gì ở mỗi endpoint.

> Passkey (WebAuthn) là đăng nhập **không mật khẩu**. Thay vì password, mỗi thiết bị giữ một **private key** (trong secure enclave — TouchID/FaceID/Windows Hello), còn server chỉ giữ **public key**. Server gửi một "thử thách" (challenge), thiết bị ký bằng private key, server verify chữ ký bằng public key.

**Cấu trúc tài liệu:**
- **Phần I — Mô tả & Cơ chế hoạt động**: hiểu passkey vận hành thế nào và tại sao nó an toàn (mức khái niệm).
- **Phần II — Code Implementation**: đi vào từng endpoint, từng dòng code cụ thể của FE/BE.

---
---

# PHẦN I — MÔ TẢ & CƠ CHẾ HOẠT ĐỘNG

## 1. Chạy project

```bash
cd backend
npm install
node server.js
```

Mở trình duyệt tại **http://localhost:3000**.

Lưu ý: Backend (`server.js`) vừa là API server, vừa serve luôn file tĩnh của frontend. Nên chỉ cần chạy 1 server duy nhất ở port 3000.

---

## 2. Bức tranh tổng thể

```
┌──────────────┐   HTTP/JSON    ┌──────────────┐
│  Frontend    │ ─────────────► │  Express     │
│  (app.js)    │ ◄───────────── │  (server.js) │
└──────┬───────┘                └──────────────┘
       │ WebAuthn Web API
       ▼
┌──────────────────────────────┐
│ Trình duyệt + Authenticator  │
│ (TouchID / FaceID / PIN)     │
└──────────────────────────────┘
```

**Backend lưu trữ trong RAM** (mất khi restart server — chỉ để demo):

| Biến | Nội dung |
|------|----------|
| `users[username]` | `{ id, currentChallenge, passkeys: [...] }` — thông tin user + danh sách passkey đã đăng ký |
| `sessions[token]` | map từ token → username (danh sách phiên đăng nhập hợp lệ) |

Mỗi passkey lưu: `credentialID`, `credentialPublicKey` (public key), `counter` (chống clone), `transports`.

---

## 3. Ba trạng thái của người dùng

```
[Chưa đăng nhập] ──register──► [Đã có user, chưa có passkey] ──setup passkey──► [Đã xác thực đầy đủ]
       │                                                                              ▲
       └──────────────────── sign in bằng passkey ───────────────────────────────────┘
```

- **Chưa đăng nhập**: không có token → chỉ thấy màn hình Login/Register.
- **Chưa có passkey**: đã tạo username, có token tạm, nhưng `/api/dashboard` trả `403` cho tới khi setup passkey xong.
- **Đã xác thực**: có ít nhất 1 passkey → vào được dashboard.

---

## 4. Tại sao private key làm được điều này? (Mật mã bất đối xứng)

Password truyền thống là **mật mã đối xứng**: cả bạn và server đều phải biết chung một bí mật (mật khẩu). Điểm yếu: server phải lưu bí mật đó → lộ database là lộ hết.

Passkey dùng **cặp khóa bất đối xứng** — hai khóa toán học gắn liền nhau:

| Khóa | Ai giữ | Làm được gì |
|------|--------|-------------|
| **Private key** (khóa riêng) | Chỉ thiết bị của bạn (trong secure enclave) | **Ký** dữ liệu |
| **Public key** (khóa công khai) | Server lưu công khai | **Verify** chữ ký |

Tính chất then chốt (dựa trên bài toán toán học như đường cong elliptic — ECDSA):

1. Từ private key → tính ra public key rất dễ. Nhưng **từ public key → không thể suy ngược ra private key** (bất khả thi về mặt tính toán).
2. Chỉ **private key** mới tạo được chữ ký hợp lệ cho một mẩu dữ liệu.
3. **Public key** verify được chữ ký đó là thật, nhưng **không tạo giả được** chữ ký mới.

→ Vì vậy server chỉ cần giữ public key. Kẻ tấn công lấy được public key cũng **không ký giả** được, vì không có private key.

---

## 5. Chữ ký số + Challenge = chống replay

Chỉ "có private key" thì chưa đủ. Nếu server luôn hỏi cùng một câu, kẻ tấn công nghe lén được một chữ ký rồi **phát lại (replay)** là xong.

Cơ chế **challenge–response** giải quyết:

```
Server:  "Hãy ký giúp tôi chuỗi ngẫu nhiên NÀY: 8f3a...c1"   (challenge, dùng 1 lần)
Thiết bị: ký(8f3a...c1) bằng private key  →  chữ ký = 9b2e...
Server:  verify(chữ ký, public key, challenge)  →  hợp lệ ✔
```

- Mỗi lần đăng nhập, challenge **khác nhau** (ngẫu nhiên) → chữ ký cũng khác → không thể phát lại chữ ký cũ.
- Server lưu challenge vào `user.currentChallenge`, verify xong là challenge coi như hết hạn.

Đây chính là lý do private key "làm được điều kỳ diệu": nó **chứng minh danh tính mà không tiết lộ bản thân nó**. Server tin bạn vì chỉ private key của bạn mới ký đúng challenge, nhưng private key chưa bao giờ rời thiết bị.

---

## 6. Phía DƯỚI passkey hoạt động thế nào (tầng hệ điều hành/phần cứng)

Khi frontend gọi `startRegistration()` / `startAuthentication()`, chuỗi thực sự xảy ra:

```
Trình duyệt (WebAuthn API navigator.credentials)
        │
        ▼
Hệ điều hành (Authenticator platform: macOS/Windows/Android)
        │
        ▼
Secure Enclave / TPM  ── chip bảo mật riêng, tách biệt khỏi hệ điều hành
        │
        ├─ Sinh & lưu private key (KHÔNG BAO GIỜ export ra ngoài)
        ├─ Yêu cầu sinh trắc (TouchID/FaceID) hoặc PIN để "mở khóa" quyền dùng key
        └─ Thực hiện phép ký NGAY BÊN TRONG chip
```

Điểm cốt lõi:
- **Private key sinh ra và nằm chết trong chip bảo mật**, phần mềm (kể cả trình duyệt, hệ điều hành) không đọc được nó.
- Sinh trắc học **không phải để gửi đi đâu cả** — nó chỉ là "chìa khóa cục bộ" để cho phép chip dùng private key. Vân tay/khuôn mặt không bao giờ rời thiết bị, không gửi lên server.
- Mỗi passkey **gắn với đúng một domain** (`rpID`). Trình duyệt sẽ từ chối dùng passkey của `localhost` cho một site khác → **chống phishing** tận gốc (khác hẳn password, người dùng có thể bị lừa gõ vào site giả).

---

## 7. Phía BACKEND hoạt động thế nào để có cơ chế này (khái niệm)

Backend (`server.js`) đóng vai **Relying Party (RP)** — bên "dựa vào" kết quả xác thực. Nó dùng thư viện `@simplewebauthn/server` để làm 2 việc mật mã, còn lại là quản lý state.

**Khi ĐĂNG KÝ (enrollment) — lưu public key:**
```
generate-registration-options:
    generateRegistrationOptions(...)
        → sinh challenge ngẫu nhiên
        → LƯU vào user.currentChallenge          (để verify ở bước sau)
verify-registration:
    verifyRegistrationResponse(...)
        → kiểm tra: challenge có khớp? origin có đúng? attestation hợp lệ?
        → TRÍCH XUẤT public key + credentialID + counter từ payload
        → LƯU vào user.passkeys[]                (đây là "danh tính" của user từ giờ)
```

**Khi ĐĂNG NHẬP (assertion) — verify chữ ký:**
```
generate-authentication-options:
    generateAuthenticationOptions(...)
        → sinh challenge MỚI
        → LƯU vào user.currentChallenge
verify-authentication:
    verifyAuthenticationResponse({ ..., authenticator: { credentialPublicKey, ... } })
        → dùng PUBLIC KEY verify chữ ký trên challenge
        → kiểm tra counter mới > counter cũ       (chống clone)
        → hợp lệ: cập nhật counter + cấp session token
```

Tóm lại backend **không bao giờ thấy private key**. Nó chỉ:
1. Ra đề (challenge) và nhớ đề đó.
2. Giữ public key.
3. Chấm bài: chữ ký này có phải do private key tương ứng ký đúng challenge của tôi không?

Ba thứ backend phải "canh" cho đúng, nếu sai là bị tấn công:
- **`expectedChallenge`** — phải đúng challenge vừa phát → chống replay.
- **`expectedOrigin` / `expectedRPID`** — phải đúng `http://localhost:3000` → chống chữ ký từ site khác.
- **`counter`** — phải tăng → chống key bị nhân bản.

---

## 8. Phía FRONTEND hoạt động thế nào để có cơ chế này (khái niệm)

Frontend (`app.js`) **không tự làm mật mã**. Nó chỉ là "người đưa thư" giữa backend và WebAuthn API của trình duyệt, qua thư viện `SimpleWebAuthnBrowser` (nạp từ `index.html`).

Vai trò của FE trong mỗi lần xác thực:
```
1. Xin "đề" từ backend         →  fetch(generate-*-options)
2. Chuyển đề cho trình duyệt   →  SimpleWebAuthnBrowser.startRegistration/startAuthentication(options)
       (thư viện dịch options → định dạng WebAuthn, gọi navigator.credentials.*)
       (trình duyệt + OS lo phần bật sinh trắc và ký — FE KHÔNG chạm vào private key)
3. Nhận "bài làm" (chữ ký)     →  attResp / asseResp
4. Nộp bài cho backend chấm    →  fetch(verify-*) kèm payload
5. Nhận token → lưu localStorage → điều hướng view
```

Những gì FE **cố tình KHÔNG làm** (và đó là điểm mạnh):
- Không đọc, không lưu, không truyền private key — nó không có quyền truy cập.
- Không xử lý sinh trắc học — đó là việc của OS.
- Không tự quyết định "đăng nhập thành công" — chỉ backend, sau khi verify chữ ký, mới cấp token.

FE chỉ giữ **session token** (kết quả sau xác thực) trong `localStorage` để các request sau (`/api/me`, `/api/dashboard`) đính kèm `Authorization: Bearer`. Token này là bằng chứng "đã xác thực rồi", còn passkey là thứ tạo ra bằng chứng đó.

---

## 9. Sơ đồ tổng hợp 3 tầng

```
┌─────────────── FRONTEND (app.js) ───────────────┐
│  Đưa thư: xin options → gọi startXxx → nộp lại   │
│  Giữ session token. KHÔNG chạm private key.      │
└───────────────┬─────────────────┬───────────────┘
                │ HTTP/JSON        │ WebAuthn API
                ▼                  ▼
┌── BACKEND (server.js) ──┐   ┌─ TRÌNH DUYỆT + OS + SECURE ENCLAVE ─┐
│ Ra challenge, nhớ nó    │   │ Giữ private key (không export)      │
│ Giữ public key          │   │ Sinh trắc mở khóa → KÝ challenge    │
│ Verify chữ ký + counter │   │ Trả về chữ ký (public key khi đăng  │
│ Cấp session token       │   │ ký)                                 │
└─────────────────────────┘   └─────────────────────────────────────┘
        (Relying Party)                  (Authenticator)
```

**Một câu chốt:** private key *ký* để chứng minh danh tính, public key *verify* mà không thể giả mạo, challenge làm mỗi lần ký là duy nhất, và secure enclave đảm bảo private key không bao giờ lộ — bốn thứ này ghép lại cho ta xác thực mạnh hơn mật khẩu mà chẳng cần lưu bí mật nào ở server.

---

## 10. Vì sao an toàn?

- **Không lưu mật khẩu** (kể cả hash) → lộ database cũng chỉ lộ public key vô dụng.
- **Challenge một lần** → chống replay: mỗi lần đăng ký/đăng nhập server sinh challenge mới, verify xong là bỏ.
- **Counter tăng dần** → chống clone key: nếu counter client gửi lên ≤ counter đã lưu, server nghi ngờ key bị nhân bản và từ chối.
- **Private key không rời thiết bị** → nằm trong secure enclave, chỉ mở được bằng sinh trắc/PIN.

> ⚠️ Đây là bản demo: dữ liệu lưu trong RAM, `rpID = localhost`, chưa có xử lý hết-hạn session hay chống dò username. Không dùng thẳng cho production.

---
---

# PHẦN II — CODE IMPLEMENTATION

Phần này đi vào từng endpoint và từng đoạn code thật của `frontend/app.js` và `backend/server.js` (kèm số dòng để tra cứu).

## 11. Khi mở UI lần đầu

**File:** `frontend/app.js` — sự kiện `DOMContentLoaded` (dòng 12)

```
Mở trang
  │
  ├─ Có token trong localStorage?
  │     ├─ CÓ  → gọi checkAuth()  → GET /api/me
  │     └─ KHÔNG → hiện màn hình Login/Register (view-auth)
```

**`GET /api/me`** (backend dòng 240): dùng token trong header `Authorization: Bearer <token>` để tra `sessions[token]` ra username, rồi trả về `{ username, passkeysEnabled }`.

Frontend dựa vào kết quả:
- `passkeysEnabled = false` → hiện màn hình **Setup Passkey**.
- `passkeysEnabled = true` → gọi `loadDashboard()` (vào dashboard luôn).
- Token sai/hết hạn (`401`) → `logout()`, quay về màn hình Login.

---

## 12. Flow ĐĂNG KÝ (Register + tạo Passkey)

Vì đây là app không mật khẩu, đăng ký gồm **2 giai đoạn**: tạo username, rồi bắt buộc gắn passkey.

### Giai đoạn A — Tạo user

```
User nhập username → bấm "Register"
        │
        ▼
POST /api/register   { username }
```

**Backend `/api/register`** (dòng 31):
1. Kiểm tra username có rỗng / đã tồn tại không.
2. Tạo `users[username] = { id, currentChallenge: null, passkeys: [] }`.
3. Tạo **token tạm** và lưu `sessions[token] = username` (để user có quyền thực hiện bước setup passkey ngay).
4. Trả về `{ message, token }`.

Frontend lưu token vào `localStorage` rồi chuyển sang màn hình **Setup Passkey**.

### Giai đoạn B — Tạo Passkey (enrollment)

Bấm "Create Passkey" → chạy 2 vòng gọi API (app.js dòng 71):

```
① POST /api/passkey/generate-registration-options   (Bearer token)
        ← server trả "options" (có challenge)
        │
② SimpleWebAuthnBrowser.startRegistration(options)
        → trình duyệt bật TouchID/FaceID/PIN
        → thiết bị tạo cặp private/public key, ký attestation
        ← trả credential payload
        │
③ POST /api/passkey/verify-registration   (Bearer token, payload)
        ← server verify & lưu public key → { verified: true }
```

**① `/api/passkey/generate-registration-options`** (dòng 53):
- Xác thực token → tìm user.
- Gọi `generateRegistrationOptions()` với `rpID`, `rpName`, `userID`, và:
  - `excludeCredentials`: các passkey đã có (để không đăng ký trùng thiết bị).
  - `authenticatorSelection`: ưu tiên `platform` (TouchID/FaceID) + `residentKey` (cho passwordless).
- **Lưu `options.challenge` vào `user.currentChallenge`** — đây là "thử thách" để chống replay.
- Trả `options` về frontend.

**② `startRegistration(options)`** (chạy ở trình duyệt): gọi WebAuthn API, người dùng xác thực sinh trắc → thiết bị sinh cặp khóa gắn với domain này, trả về credential (chứa public key + attestation).

**③ `/api/passkey/verify-registration`** (dòng 90):
- Gọi `verifyRegistrationResponse()` kiểm tra payload khớp với `expectedChallenge` (chính là `currentChallenge` đã lưu), `expectedOrigin`, `expectedRPID`.
- Nếu hợp lệ → lưu vào `user.passkeys`: `credentialID`, `credentialPublicKey`, `counter`, `transports`.
- Trả `{ verified: true }`.

Frontend nhận verified → gọi `loadDashboard()` → vào dashboard. **User giờ đã đăng ký đầy đủ.**

---

## 13. Flow ĐĂNG NHẬP (Sign In bằng Passkey)

```
User nhập username → bấm "Sign In with Passkey"
        │
① POST /api/passkey/generate-authentication-options   { username }
        ← server trả "options" (challenge + allowCredentials)
        │
② SimpleWebAuthnBrowser.startAuthentication(options)
        → trình duyệt bật TouchID/FaceID/PIN
        → thiết bị ký challenge bằng private key
        ← trả assertion response
        │
③ POST /api/passkey/verify-authentication   { username, response }
        ← server verify chữ ký → { verified: true, token }
```

**① `/api/passkey/generate-authentication-options`** (dòng 140):
- Tìm user theo username (không cần token — user chưa đăng nhập mà).
- Gọi `generateAuthenticationOptions()`, đưa các `credentialID` đã đăng ký vào `allowCredentials`.
- Lưu `options.challenge` mới vào `user.currentChallenge`.
- Trả `options` về.

**② `startAuthentication(options)`** (trình duyệt): người dùng xác thực sinh trắc → private key trên thiết bị **ký challenge** → trả assertion (chữ ký).

**③ `/api/passkey/verify-authentication`** (dòng 169):
- Tìm passkey khớp `credentialID` với `response.id`.
- Gọi `verifyAuthenticationResponse()` verify chữ ký bằng **public key đã lưu** + challenge + origin.
- Kiểm tra **counter** phải lớn hơn giá trị đã lưu (chống clone/replay).
- Nếu ok: cập nhật `counter`, tạo **token phiên chính thức**, lưu `sessions[token] = username`, trả `{ verified: true, token }`.

Frontend lưu token → `loadDashboard()`.

---

## 14. Vào Dashboard

**File:** `app.js` — `loadDashboard()` (dòng 194)

```
GET /api/dashboard   (Bearer token)
```

**Backend `/api/dashboard`** (dòng 221):
1. Verify token → ra username.
2. Nếu `user.passkeys.length === 0` → trả **`403`** ("phải setup passkey"). Frontend sẽ chuyển về màn hình Setup Passkey.
3. Ngược lại trả message chào mừng → frontend hiện `view-dashboard`.

Đây là ví dụ về **route được bảo vệ**: chỉ user có token hợp lệ *và* đã có passkey mới truy cập được.

---

## 15. Đăng xuất

`logout()` (app.js dòng 219): xóa token khỏi biến `currentToken` + `localStorage`, reset form, quay về màn hình Login.

> Lưu ý: bản demo này **không** xóa token khỏi `sessions` phía server khi logout — chỉ xóa ở client. Trong app thật cần thêm endpoint để hủy session server-side.

---

## 16. Bảng tóm tắt các API

| API | Auth? | Khi nào gọi | Backend làm gì |
|-----|-------|-------------|----------------|
| `POST /api/register` | Không | Bấm Register | Tạo user + token tạm |
| `POST /api/passkey/generate-registration-options` | Bearer | Bắt đầu tạo passkey | Sinh options + lưu challenge |
| `POST /api/passkey/verify-registration` | Bearer | Sau khi ký attestation | Verify & lưu public key vào `passkeys` |
| `POST /api/passkey/generate-authentication-options` | Không | Bấm Sign In | Sinh options + lưu challenge |
| `POST /api/passkey/verify-authentication` | Không | Sau khi ký challenge | Verify chữ ký, cập nhật counter, cấp token |
| `GET /api/me` | Bearer | Mở app (nếu có token) | Trả username + passkeysEnabled |
| `GET /api/dashboard` | Bearer | Vào dashboard | Kiểm tra token + passkey rồi trả nội dung |
