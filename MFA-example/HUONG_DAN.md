# Hướng dẫn Project MFA-example (TOTP + Passkey/WebAuthn)

Tài liệu này giải thích **flow chạy thực tế** của app: từ lúc mở UI, người dùng bấm nút gì, frontend gọi API nào, backend làm gì với mỗi request, và state (session) chuyển đổi ra sao.

---

## 1. Tổng quan kiến trúc

| Thành phần | File | Vai trò |
|---|---|---|
| **Frontend (SPA)** | `frontend/index.html`, `frontend/app.js`, `frontend/styles.css` | 1 trang HTML nhiều "view", chuyển view bằng JS. Không reload trang. |
| **Backend (API)** | `backend/server.js` | Express server, vừa serve file tĩnh của frontend vừa cung cấp REST API dưới `/api/*`. |
| **Thư viện MFA** | `speakeasy` + `qrcode` (TOTP), `@simplewebauthn/server` + `@simplewebauthn/browser` (Passkey) |

**Điểm quan trọng:** Backend serve luôn frontend (`express.static`), nên chỉ cần chạy 1 server ở `http://localhost:3000` là mở được UI.

### Data store (in-memory — mất khi restart server)

```js
users        = { username: { id, password, mfaEnabled, mfaSecret, currentChallenge, passkeys: [] } }
sessions     = { token: username }       // token đăng nhập ĐẦY ĐỦ (đã qua MFA)
tempSessions = { tempToken: username }   // token TẠM (đúng password nhưng CHƯA qua MFA)
```

Tất cả lưu trong RAM. Restart `node server.js` là mất sạch user → phải Register lại.

---

## 2. Ba trạng thái session (nắm cái này là hiểu cả app)

```
Chưa đăng nhập
   │  (nhập đúng password, ĐÃ bật MFA)      →  TempAuthenticated  (chỉ có tempToken)
   │  (nhập đúng password, CHƯA bật MFA)    →  FullyAuthenticated (có token thật)
TempAuthenticated
   │  (nhập đúng TOTP hoặc Passkey)         →  FullyAuthenticated (có token thật)
FullyAuthenticated
   │  (logout / restart server)            →  Chưa đăng nhập
```

- `tempToken`: chỉ dùng được để **verify MFA**, không vào được dashboard.
- `token` (thật): lưu trong `localStorage`, gửi kèm header `Authorization: Bearer <token>` cho các API cần bảo vệ.
- **Chốt chặn dashboard:** dù có token thật, nếu user **chưa bật ít nhất 1 MFA** (TOTP hoặc Passkey) thì `/api/dashboard` vẫn trả `403`. → Ép user phải setup MFA.

---

## 3. Flow chạy chi tiết (theo đúng thứ tự người dùng thao tác)

### Bước 0 — Mở app
1. Truy cập `http://localhost:3000` → tải `index.html` + `app.js`.
2. `app.js` chạy `DOMContentLoaded`:
   - Nếu `localStorage` đã có `token` → gọi `GET /api/me` để kiểm tra token còn sống không, rồi điều hướng (xem Bước 6).
   - Nếu chưa có token → hiện **view-auth** (màn hình Login/Register).

---

### Bước 1 — Đăng ký (Register)
- **User:** nhập username + password, bấm **Register**.
- **FE → BE:** `POST /api/register` body `{ username, password }`.
- **BE làm gì:**
  - Kiểm tra thiếu field → `400`.
  - Username đã tồn tại → `400`.
  - OK → tạo `users[username]` với `mfaEnabled: false`, `passkeys: []`, `id` random (crypto). Password lưu **thô** (demo — thực tế phải hash!).
- **FE:** hiện toast "Registered successfully". *Lưu ý: register xong KHÔNG tự đăng nhập*, user phải bấm **Sign In**.

---

### Bước 2 — Đăng nhập (First factor: password)
- **User:** nhập username + password, bấm **Sign In**.
- **FE → BE:** `POST /api/login` body `{ username, password }`.
- **BE kiểm tra:**

| Trường hợp | BE trả về | FE điều hướng tới |
|---|---|---|
| Sai user/password | `401` | Toast lỗi, ở lại màn login |
| Đúng password **+ đã bật MFA** (`mfaEnabled` hoặc có passkey) | Tạo `tempToken`, trả `{ requireMfa: true, tempToken, hasTotp, hasPasskey }` | **view-mfa-verify** (màn hình Security Check) |
| Đúng password **+ chưa bật MFA** | Tạo `token` thật, trả `{ requireMfa: false, token }` | **view-security-setup** (bắt đi setup MFA) |

- Ở màn **Security Check**, FE dùng `hasTotp`/`hasPasskey` để bật/tắt phần nhập TOTP và nút Passkey — chỉ hiện đúng phương thức mà user đã đăng ký.

---

### Bước 3 — Setup MFA (chỉ khi user chưa có MFA, đang ở view-security-setup)

Có 2 lựa chọn, dùng **token thật** (đã có sau khi login không-MFA).

#### 3a. Setup TOTP (Authenticator app)
1. **User:** bấm **Setup Authenticator** → FE gọi `POST /api/mfa/generate` body `{ token }`.
2. **BE:** verify token → sinh secret bằng `speakeasy.generateSecret()`, lưu vào `user.mfaSecret`, tạo QR code (data URL) từ `otpauth_url`. Trả `{ secret, qrCodeUrl }`.
3. **FE:** hiện QR + secret text (view-mfa-setup). User quét bằng Google Authenticator/Authy.
4. **User:** nhập 6 số → FE gọi `POST /api/mfa/verify` body `{ token, code }`.
5. **BE:** `speakeasy.totp.verify()` (cho lệch ±1 chu kỳ, `window: 1`). Đúng → set `user.mfaEnabled = true`, trả success.
6. **FE:** toast thành công → `checkTokenAndRedirect()` → vào dashboard.

---

#### 🔍 Vì sao Authenticator app tự sinh được mã đúng? (Cơ chế TOTP)

Trước khi đi vào code, hãy hiểu **tại sao Google Authenticator trên điện thoại (đang offline, không kết nối gì tới server) lại sinh ra đúng 6 số mà server chấp nhận?**

Câu trả lời ngắn: **cả hai bên cùng biết một bí mật (secret) và cùng nhìn vào đồng hồ (thời gian).** Không ai gửi mã qua mạng cả — mã được **tính ra độc lập ở 2 nơi** từ cùng input, nên nó trùng nhau.

##### (1) Chuẩn đằng sau: HOTP → TOTP

TOTP (RFC 6238) = HOTP (RFC 4226) nhưng thay "bộ đếm" bằng "thời gian".

```
code = HOTP(secret, counter)          // HOTP: counter tăng mỗi lần bấm nút
counter = floor(unix_time / 30)       // TOTP: counter = mốc thời gian 30 giây
code = HOTP(secret, floor(now / 30))  // TOTP
```

Nghĩa là: cứ **mỗi 30 giây** thì `counter` tăng 1 → mã đổi. Trong cùng một khung 30s, mọi thiết bị dùng cùng `secret` sẽ tính ra **y hệt** một mã.

##### (2) Ba nguyên liệu chung giữa App và Server

| Nguyên liệu | App có được từ đâu | Server có được từ đâu |
|---|---|---|
| **Secret** (bí mật chung) | Quét từ QR lúc setup | Sinh ra & lưu `user.mfaSecret` |
| **Thời gian hiện tại** | Đồng hồ điện thoại | Đồng hồ server |
| **Thuật toán** (HMAC-SHA1 + cắt số) | Chuẩn RFC, ai cũng cài giống nhau | Chuẩn RFC (thư viện `speakeasy`) |

→ Cùng secret + cùng thời gian + cùng thuật toán ⇒ **cùng kết quả**. Đó là lý do app không cần mạng.

##### (3) QR code thực chất chứa gì?

`secret.otpauth_url` (cái được mã hoá thành QR) có dạng chuẩn:

```
otpauth://totp/MFA%20Example%20App%20(ninh)?secret=JBSWY3DPEHPK3PXP&issuer=...
                                                    ▲
                                          Đây chính là SECRET (base32)
```

**QR code chỉ là cách "gõ hộ" cái secret vào app** cho nhanh, khỏi phải nhập tay chuỗi `JBSWY3DPEHPK3PXP`. (Chính vì vậy màn setup có hiện luôn "Secret Key" dạng text để nhập tay nếu không quét được.) Sau khi app đọc secret này, nó lưu lại và từ đó tự tính mã mỗi 30s — **không bao giờ liên lạc lại với server nữa.**

##### (4) Thuật toán sinh 6 số (bên trong `speakeasy`)

Cả app lẫn server đều chạy đúng các bước này:

```
1. T = floor(unix_time / 30)                      // đếm số khung 30 giây kể từ 1970
2. hash = HMAC-SHA1(key = secret, message = T)    // ra 20 byte
3. offset = 4 bit cuối của hash                    // "dynamic truncation"
4. lấy 4 byte tại offset → 1 số 31-bit
5. code = số đó % 1_000_000                         // lấy 6 chữ số cuối
6. pad '0' cho đủ 6 số  →  ví dụ "042731"
```

Vì mọi thứ deterministic (input giống nhau ra output giống nhau), nên điện thoại của bạn và server Node.js chạy ra cùng "042731" trong cùng khung 30 giây.

##### (5) Toàn cảnh 2 pha

```
── Pha SETUP (1 lần) ─────────────────────────────
Server: sinh secret ──► lưu user.mfaSecret
        │
        └─► nhét secret vào QR (otpauth_url)
App:    quét QR ──► lưu secret vào điện thoại
        (từ đây App & Server cùng giữ chung 1 secret)

── Pha DÙNG (mỗi lần login, OFFLINE) ─────────────
App:    code = TOTP(secret, now)  ──► hiện 6 số
User:   gõ 6 số vào web
Server: code' = TOTP(secret, now) ──► so code == code' (±window)
```

Điểm mấu chốt: **secret chỉ truyền 1 lần duy nhất lúc setup (qua QR)**. Sau đó mã 6 số bay qua mạng nhưng nó chỉ sống 30 giây và không suy ngược ra được secret → kẻ nghe lén bắt được 1 mã cũng vô dụng.

> **So sánh nhanh với Passkey:** TOTP là *"bí mật đối xứng dùng chung"* (cả 2 bên cùng biết secret — nếu DB server lộ, secret lộ). Passkey/WebAuthn là *"khoá bất đối xứng"* (điện thoại giữ private key, server chỉ giữ public key — server lộ cũng không giả mạo được). Đó là lý do Passkey được xem là an toàn hơn TOTP.

##### (6) Code implementation phía Backend

**Lúc setup — `/api/mfa/generate`:** sinh secret và nhét vào QR.

```js
const secret = speakeasy.generateSecret({ name: `MFA Example App (${username})` });
user.mfaSecret = secret.base32;                               // server LƯU secret dạng base32
const qrCodeUrl = await QRCode.toDataURL(secret.otpauth_url); // QR = mã hoá otpauth_url (chứa secret)
```

**Lúc verify — `/api/mfa/verify`:** tự tính lại mã rồi so.

```js
const verified = speakeasy.totp.verify({
    secret: user.mfaSecret,   // secret server đã lưu lúc setup
    encoding: 'base32',
    token: code,              // 6 số user gõ vào
    window: 1                 // cho phép lệch ±1 khung (±30s)
});
```

Backend **không so sánh chuỗi** với cái gì đã lưu sẵn cả. Nó tự làm lại đúng phép tính ở mục (4) với `secret` của user + thời gian hiện tại của server, ra một mã "chuẩn", rồi so mã user gõ có khớp không.

**`window: 1` để làm gì?** Đồng hồ điện thoại và server hiếm khi khớp tuyệt đối, và user cần vài giây để gõ. Nếu chỉ chấp nhận đúng khung hiện tại thì mã sắp hết hạn sẽ bị từ chối oan. `window: 1` bảo server thử thêm **1 khung trước và 1 khung sau** (tức kiểm tra 3 mã: `T-1`, `T`, `T+1`) → chấp nhận lệch tối đa ~±30 giây. Đánh đổi: cửa sổ hợp lệ rộng hơn một chút (bảo mật giảm nhẹ) để đổi lấy trải nghiệm mượt.

---

#### 3b. Setup Passkey (WebAuthn — Touch ID/Face ID/USB key)
1. **User:** bấm **Create Passkey** → FE gọi `POST /api/passkey/generate-registration-options` (header `Authorization: Bearer <token>`).
2. **BE:** `generateRegistrationOptions()` (kèm `excludeCredentials` để không đăng ký trùng), lưu `options.challenge` vào `user.currentChallenge`, trả options JSON.
3. **FE:** `SimpleWebAuthnBrowser.startRegistration(options)` → trình duyệt/OS bật popup xác thực sinh trắc học. Trả về credential payload (public key…).
4. **FE → BE:** `POST /api/passkey/verify-registration` (header Bearer + payload).
5. **BE:** `verifyRegistrationResponse()` đối chiếu `expectedChallenge` + `expectedOrigin` + `expectedRPID`. Đúng → lưu `{ credentialID, credentialPublicKey, counter, transports }` vào `user.passkeys`. Trả `{ verified: true }`.
6. **FE:** toast thành công, refresh trạng thái.

> **Chú ý về Passkey:** `rpID = 'localhost'`, `origin = 'http://localhost:3000'` (hard-code trong `server.js`). Phải mở đúng bằng `localhost:3000` thì WebAuthn mới hoạt động (khác host/origin sẽ fail verify).

---

### Bước 4 — Verify MFA khi đăng nhập (đang ở view-mfa-verify, có tempToken)

#### 4a. Verify bằng TOTP
- **User:** nhập 6 số → FE gọi `POST /api/mfa/verify` body `{ tempToken, code }`.
- **BE:** tra `tempSessions[tempToken]` → lấy user → `speakeasy.totp.verify`. Đúng →
  - Xoá `tempToken` khỏi `tempSessions`.
  - Sinh `token` thật, lưu vào `sessions`.
  - Trả `{ token }`.
- **FE:** lưu token vào `localStorage` → `loadDashboard()`.

> Endpoint `/api/mfa/verify` dùng chung cho cả **setup** (gửi `token`) và **login** (gửi `tempToken`). BE phân biệt bằng field nào có trong body.

#### 4b. Verify bằng Passkey
1. **User:** bấm **Use Passkey / Biometrics** → FE gọi `POST /api/passkey/generate-authentication-options` body `{ tempToken }`.
2. **BE:** tra tempToken → lấy passkey của user → `generateAuthenticationOptions()`, lưu challenge vào `user.currentChallenge`, trả options.
3. **FE:** `SimpleWebAuthnBrowser.startAuthentication(options)` → popup sinh trắc học → trả assertion.
4. **FE → BE:** `POST /api/passkey/verify-authentication` body `{ tempToken, response }`.
5. **BE:** tìm passkey khớp `response.id` → `verifyAuthenticationResponse()` với public key + counter + challenge + origin. Đúng →
   - Cập nhật `passkey.counter = newCounter` (**chống replay/clone** — counter mới phải lớn hơn cũ).
   - Xoá tempToken, sinh `token` thật, trả `{ verified: true, token }`.
6. **FE:** lưu token → vào dashboard.

---

### Bước 5 — Dashboard
- **FE:** `loadDashboard()` gọi `GET /api/me` (lấy trạng thái MFA để hiện badge) rồi `GET /api/dashboard` (đều kèm Bearer token).
- **BE `/api/dashboard`:** verify token trong `sessions`, **và** kiểm tra `mfaEnabled || passkeys.length > 0`. Chưa có MFA → `403` → FE đẩy ngược về view-security-setup.
- Dashboard hiện lời chào + 2 badge trạng thái (Authenticator / Passkeys: Active hoặc Not configured).

---

### Bước 6 — Quay lại app khi đã có token (reload trang / mở lại)
- `checkTokenAndRedirect()` gọi `GET /api/me`:
  - Token chết → `logout()` (xoá localStorage, về màn login).
  - Token sống + đã có MFA → vào dashboard.
  - Token sống + chưa có MFA → view-security-setup.

### Logout
- `logout()` chỉ xoá state phía client (localStorage + biến `state`) và về màn login. **Không có API logout** → token vẫn nằm trong `sessions` phía server cho tới khi restart (đây là hạn chế của bản demo).

---

## 4. Bảng tra nhanh API

| Method & Endpoint | Auth | Input | Backend làm gì |
|---|---|---|---|
| `POST /api/register` | — | `{username, password}` | Tạo user mới |
| `POST /api/login` | — | `{username, password}` | Check password → trả `token` (no MFA) hoặc `tempToken` (có MFA) |
| `POST /api/mfa/generate` | `token` trong body | `{token}` | Sinh secret + QR TOTP |
| `POST /api/mfa/verify` | `token` (setup) / `tempToken` (login) | `{token\|tempToken, code}` | Verify TOTP; setup→bật MFA, login→cấp token thật |
| `POST /api/passkey/generate-registration-options` | Bearer token | — | Tạo options đăng ký passkey |
| `POST /api/passkey/verify-registration` | Bearer token | credential payload | Lưu passkey vào user |
| `POST /api/passkey/generate-authentication-options` | `tempToken` trong body | `{tempToken}` | Tạo options xác thực passkey |
| `POST /api/passkey/verify-authentication` | `tempToken` trong body | `{tempToken, response}` | Verify passkey → cấp token thật |
| `GET /api/me` | Bearer token | — | Trả `{username, mfaEnabled, passkeysEnabled}` |
| `GET /api/dashboard` | Bearer token | — | Trả lời chào (chặn nếu chưa có MFA) |

---

## 5. Cách chạy

```bash
cd backend
npm install
node server.js
# Mở http://localhost:3000
```

Thử nhanh: **Register** → **Sign In** → (chưa có MFA nên vào Security Setup) → **Setup Authenticator** hoặc **Create Passkey** → vào Dashboard. Logout rồi Sign In lại để thấy màn **Security Check** (verify MFA).

---

## 6. Lưu ý / hạn chế của bản demo (đừng bê nguyên lên production)

- Password lưu **plaintext** → thực tế phải hash (bcrypt/argon2).
- Session lưu **in-memory**, không hết hạn, không có API logout phía server.
- Không có rate-limit cho việc thử password / thử mã TOTP.
- Passkey khoá cứng `rpID=localhost`, `origin=http://localhost:3000` → deploy domain khác phải sửa `server.js`.
- `mfa/verify` dùng chung cho setup và login — tiện demo nhưng nên tách endpoint cho rõ ràng.
