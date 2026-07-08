# 📞 Hướng dẫn Flow chạy Video Call App

Tài liệu này giải thích **từ A→Z** cách project chạy: từ lúc mở UI, gọi API nào, mở WebSocket ra sao, rồi thiết lập kết nối WebRTC peer-to-peer như thế nào.

> Đọc kèm: [WEBRTC_ARCHITECTURE_NOTES.md](WEBRTC_ARCHITECTURE_NOTES.md) để hiểu sâu về WebRTC.

---

## 1. Kiến trúc tổng quan

```
┌────────────┐   HTTP REST (auth)    ┌─────────────────┐
│            │ ────────────────────▶ │                 │
│  Frontend  │                       │  Backend (Go)   │
│  React/Vite│   WebSocket (signaling)│  gorilla/ws     │
│            │ ◀───────────────────▶ │  + SQLite       │
└─────┬──────┘                       └─────────────────┘
      │                                       ▲
      │  Media KHÔNG đi qua backend            │ chỉ làm "người đưa thư"
      │  (chỉ signaling đi qua WS)             │ chuyển tiếp SDP/ICE
      ▼
┌────────────┐   WebRTC P2P (UDP/DTLS-SRTP)   ┌────────────┐
│  Alice     │ ◀═══════════════════════════▶ │   Bob      │
│  (browser) │   video + audio đi thẳng       │  (browser) │
└────────────┘                                └────────────┘
```

**Điểm mấu chốt:** Backend chỉ lo **auth** (REST) và **signaling** (WebSocket — trung chuyển tin nhắn để 2 bên "bắt tay"). Khi đã kết nối, **video/audio đi thẳng peer-to-peer qua UDP**, không qua server → độ trễ thấp.

### Các thành phần code chính

| Vai trò | File |
|---------|------|
| Entry backend, routes, CORS | `backend/main.go` |
| Auth (register/login/me/users) + JWT | `backend/internal/auth/handler.go` |
| WebSocket Hub (signaling relay) | `backend/internal/signaling/hub.go` |
| DB init + schema `users` | `backend/internal/database/db.go` |
| Routing UI + guard đăng nhập | `frontend/src/App.tsx` |
| State auth, gọi API login/register | `frontend/src/context/AuthContext.tsx` |
| Kết nối WebSocket, nhận/gửi signal | `frontend/src/hooks/useWebSocket.ts` |
| **Toàn bộ logic WebRTC** | `frontend/src/hooks/useWebRTC.ts` |
| Màn hình chính, ghép mọi thứ lại | `frontend/src/pages/Dashboard.tsx` |
| Cấu hình URL API/WS + ICE servers | `frontend/src/config.ts` |

---

## 2. Chạy project local

```bash
# 1. Cài dependencies (Go modules + npm)
make install

# 2. Chạy song song backend (:8080) + frontend (:5173)
make dev

# 3. Mở trình duyệt
open http://localhost:5173
```

Hoặc chạy riêng: `make dev-be` / `make dev-fe`. Chạy bằng Docker: `make up` (frontend → `:3000`).

> **Test giữa 2 thiết bị cùng WiFi:** truy cập `http://<IP-máy-bạn>:5173` từ điện thoại. `config.ts` tự phát hiện hostname để trỏ backend đúng (xem mục 8).

---

## 3. Flow đăng nhập (REST API)

Khi mở UI, `App.tsx` đọc state từ `AuthContext`:

1. **`AuthProvider` khởi động** (`AuthContext.tsx:21`): đọc `localStorage` xem có `token` + `user` đã lưu chưa.
   - Có → set luôn vào state (không cần gọi lại API), `loading = false`.
   - Chưa → hiển thị màn Login.
2. `App.tsx` route theo state:
   - Chưa có `user` → `/login` (hoặc `/register`).
   - Có `user` → `/` = `Dashboard`.

### API được gọi

| Hành động | Request | Response |
|-----------|---------|----------|
| Đăng ký | `POST /api/auth/register` `{username, email, password, display_name}` | `{token, user}` |
| Đăng nhập | `POST /api/auth/login` `{username, password}` | `{token, user}` |
| Lấy user hiện tại | `GET /api/auth/me` (header `Authorization: Bearer <jwt>`) | `user` |
| Danh sách user | `GET /api/users` (Bearer) | `[user...]` |

**Backend xử lý** (`auth/handler.go`):
- `Register`: validate → `bcrypt` hash password → `INSERT` vào SQLite → sinh JWT (HS256, hạn 24h) → trả `{token, user}`.
- `Login`: tìm user theo username → `bcrypt.CompareHashAndPassword` → sinh JWT → trả về.
- JWT chứa claims: `user_id`, `username`, `exp`.

**Frontend lưu lại** (`AuthContext.tsx:44-48`): set `token`/`user` vào state **và** `localStorage` → lần sau mở lại không cần đăng nhập.

---

## 4. Mở WebSocket (signaling channel)

Ngay khi vào `Dashboard`, hook `useWebSocket(token)` chạy (`Dashboard.tsx:10`):

1. `useWebSocket` mở kết nối: `new WebSocket("ws://host:8080/ws?token=<jwt>")` (`useWebSocket.ts:22`).
   - **Token truyền qua query param**, không phải header (giới hạn của WebSocket browser API).
2. **Backend xác thực** (`hub.go:HandleWebSocket`, dòng 122):
   - Lấy `token` từ query → verify JWT bằng cùng secret → lấy `user_id`, `username`.
   - Query DB lấy `display_name`.
   - `upgrader.Upgrade()` nâng HTTP → WebSocket.
   - Tạo `Client{UserID, Conn, Send chan}` → đẩy vào `hub.register`.
3. **Hub đăng ký client** (`hub.go:Run`, dòng 65): thêm vào `map[userID]*Client`, rồi gọi `broadcastOnlineUsers()`.
4. Mỗi client chạy 2 goroutine:
   - `readPump`: đọc tin nhắn từ browser → gắn `From`/`FromName` → `routeMessage`.
   - `writePump`: đọc từ channel `Send` → ghi xuống WebSocket.

### Danh sách user online

Mỗi khi có người **connect/disconnect**, Hub gửi message `type: "users-list"` tới **tất cả** client (`hub.go:91`).

Frontend nhận trong `useWebSocket.ts:34`: nếu `type === "users-list"` → cập nhật `onlineUsers` (hiển thị ở sidebar); còn lại → đẩy vào mảng `signalMessages` cho hook WebRTC xử lý.

> **Auto-reconnect:** WS đóng → tự kết nối lại sau 3 giây (`useWebSocket.ts:48`).

---

## 5. Flow thiết lập cuộc gọi WebRTC ⭐

Đây là phần cốt lõi. Toàn bộ nằm trong `useWebRTC.ts`. Giả sử **Alice gọi Bob**.

### Bức tranh tổng: "Signaling handshake"

```
Alice                          Backend (WS Hub)                    Bob
  │                                  │                              │
  │ 1. getUserMedia (camera/mic)     │                              │
  │ 2. tạo RTCPeerConnection         │                              │
  │─── "call" ──────────────────────▶│──── "call" ─────────────────▶│  (hiện modal)
  │─── "offer" (SDP) ───────────────▶│──── "offer" ────────────────▶│
  │                                  │                              │ 3. bấm Accept
  │                                  │                              │    getUserMedia + PC
  │◀── "answer" (SDP) ───────────────│◀─── "answer" ────────────────│
  │◀═▶ "ice-candidate" (nhiều lần) ══│═══▶ "ice-candidate" ═════════▶│
  │                                  │                              │
  │═══════════ WebRTC P2P kết nối (UDP) — video/audio đi thẳng ═════▶│
```

### Bước chi tiết

**A. Alice bấm gọi** → `UserList` gọi `startCall(bobId)` (`useWebRTC.ts:161`):
1. `getMediaStream()`: `navigator.mediaDevices.getUserMedia({video, audio})` → xin quyền camera/mic, lấy `localStream`.
2. `createPeerConnection(bobId)`: tạo `new RTCPeerConnection(WEBRTC_CONFIG)` (config có ICE servers — xem mục 6). Gắn các track local vào PC (`pc.addTrack`). Đăng ký các event handler (xem bên dưới).
3. Gửi signal `type: "call"` để Bob hiện modal "cuộc gọi đến".
4. `pc.createOffer()` → `pc.setLocalDescription(offer)` → gửi signal `type: "offer"` kèm **SDP** (mô tả codec, độ phân giải, khả năng của Alice).

**B. Bob nhận** (`useWebRTC.ts:270` — `useEffect` xử lý `signalMessages`):
- Nhận `"call"` → set `incomingCall`, `callStatus = "receiving"` → `Dashboard` hiện `IncomingCall` modal.
- Nhận `"offer"` khi **chưa** có PeerConnection → lưu tạm vào `pendingOfferRef` (chờ Bob bấm Accept).

**C. Bob bấm Accept** → `acceptCall()` (`useWebRTC.ts:190`):
1. `getMediaStream()` + `createPeerConnection(aliceId)`.
2. `pc.setRemoteDescription(offer)` (từ `pendingOfferRef`).
3. `pc.createAnswer()` → `pc.setLocalDescription(answer)` → gửi `type: "answer"` kèm SDP về Alice.
4. Xử lý các ICE candidate đã đến sớm (`pendingCandidatesRef`) — xem lưu ý dưới.
5. Gửi `type: "call-accepted"`.

**D. Alice nhận `"answer"`** (`useWebRTC.ts:316`): `pc.setRemoteDescription(answer)`. → Giờ 2 bên đã biết SDP của nhau.

**E. Trao đổi ICE candidate (song song, tự động):**
- Mỗi khi PeerConnection tìm được 1 "đường đi mạng" khả dĩ (IP:port), `pc.onicecandidate` bắn ra (`useWebRTC.ts:123`) → gửi signal `type: "ice-candidate"`.
- Bên kia nhận (`useWebRTC.ts:324`): nếu đã có `remoteDescription` → `pc.addIceCandidate()`; nếu chưa → xếp vào `pendingCandidatesRef` để xử lý sau.
- ICE thử lần lượt: kết nối trực tiếp (host) → qua STUN (khám phá IP public khi có NAT) → qua TURN (relay, nếu có).

**F. Kết nối thành công:**
- `pc.oniceconnectionstatechange` = `"connected"` → `callStatus = "connected"` (`useWebRTC.ts:150`).
- `pc.ontrack` bắn ra remote track (`useWebRTC.ts:115`) → gắn vào `remoteStream` → `VideoCall` render video của bên kia.
- **Từ đây video/audio đi thẳng UDP giữa 2 browser, không qua backend nữa.**

### Kết thúc / từ chối cuộc gọi

| Hành động | Hàm | Signal gửi đi |
|-----------|-----|---------------|
| Bob từ chối | `rejectCall()` | `call-rejected` → Alice `endCall()` |
| Cúp máy | `endCall()` | `call-ended` → bên kia cũng `endCall()` |
| Mất kết nối ICE | tự động | `iceConnectionState = failed/disconnected` → `endCall()` |

`endCall()` (`useWebRTC.ts:66`): đóng `RTCPeerConnection`, stop tất cả track (tắt đèn camera), reset toàn bộ state về `idle`.

**Toggle mic/cam:** `toggleAudio`/`toggleVideo` chỉ set `track.enabled = false` (không stop track) → tạm tắt mà không mất kết nối.

---

## 6. Cấu hình ICE / STUN / TURN

`config.ts` build ra `WEBRTC_CONFIG` truyền vào `RTCPeerConnection`:

- **STUN** (mặc định Google `stun.l.google.com:19302`): giúp browser khám phá IP public của mình khi ở sau NAT. Đủ cho phần lớn trường hợp cùng mạng hoặc NAT đơn giản.
- **TURN** (tùy chọn): server relay media khi P2P trực tiếp bất khả thi (NAT đối xứng, firewall chặt). Cấu hình qua env `VITE_TURN_URLS` / `TURN_USERNAME` / `TURN_CREDENTIAL`.
- **`iceTransportPolicy`**: `"all"` (mặc định) hoặc `"relay"` (ép đi qua TURN — để test).

> Chưa dựng TURN → 2 máy ở mạng khác nhau/NAT chặt có thể gọi được nhưng **không lên hình**. Xem [TURN_COTURN_AWS_SETUP.md](TURN_COTURN_AWS_SETUP.md).

---

## 7. Các loại signal message

Tất cả đi chung struct (`hub.go:24` / `types.ts:23`):

```ts
{ type, from, to, payload, fromName }
```

| `type` | Ý nghĩa | `payload` |
|--------|---------|-----------|
| `call` | Bắt đầu gọi (hiện modal) | `null` |
| `offer` | SDP của bên gọi | SDP |
| `answer` | SDP của bên nhận | SDP |
| `ice-candidate` | 1 đường đi mạng ứng viên | ICE candidate |
| `call-accepted` | Đã bấm nhận | `null` |
| `call-rejected` | Từ chối | `null` |
| `call-ended` | Cúp máy | `null` |
| `users-list` | (server→client) danh sách online | `[user...]` |

Backend `routeMessage` (`hub.go:213`) chỉ **chuyển tiếp 1-1** theo `msg.To` — hoàn toàn không hiểu nội dung SDP/ICE, chỉ làm "người đưa thư".

---

## 8. Cấu hình URL theo môi trường

`config.ts` tự chọn địa chỉ backend:

- **Dev (port 5173):** API → `http://<hostname>:8080`, WS → `ws://<hostname>:8080/ws`. Dùng `hostname` (không hardcode `localhost`) nên truy cập qua IP LAN từ điện thoại vẫn trỏ đúng backend.
- **Production:** dùng **same-origin** (`window.location.origin`) → API/WS đi qua cùng domain (nginx/ALB reverse-proxy `/api` và `/ws` về backend).
- Có thể override bằng `window.__APP_CONFIG__` (runtime, xem `public/env-config.js`) hoặc biến `VITE_*` lúc build.

---

## 9. Tóm tắt 1 dòng cho từng giai đoạn

1. **Mở UI** → `AuthContext` đọc localStorage → có token thì vào Dashboard, không thì Login.
2. **Login** → `POST /api/auth/login` → nhận JWT, lưu localStorage.
3. **Vào Dashboard** → mở `WebSocket /ws?token=jwt` → nhận `users-list` (ai đang online).
4. **Bấm gọi** → xin camera/mic → tạo `RTCPeerConnection` → gửi `call` + `offer(SDP)` qua WS.
5. **Bên kia Accept** → tạo PC → trả `answer(SDP)`.
6. **2 bên trao ICE candidate** qua WS → ICE tìm được đường → **kết nối UDP P2P**.
7. **Video/audio chảy thẳng peer-to-peer**, backend đứng ngoài.
8. **Cúp máy** → gửi `call-ended` → đóng PC, stop track, reset về `idle`.
