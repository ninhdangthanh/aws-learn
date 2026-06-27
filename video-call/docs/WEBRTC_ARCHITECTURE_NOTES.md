# Ghi Chú Kiến Trúc WebRTC

## Vấn Đề Đang Gặp

Sau khi deploy, app có thể:

- Login được.
- Thấy user online.
- Gọi và nhận cuộc gọi được.
- Chạy tốt khi test 2 browser trên cùng 1 máy.

Nhưng khi test bằng 1 laptop và 1 điện thoại thì:

- Không thấy hình remote.
- Không nghe được tiếng remote.

Đây gần như chắc chắn là vấn đề ở đường media WebRTC, không phải vấn đề login hay WebSocket signaling.

## Luồng Kiến Trúc Của Project

Project này có 3 luồng chính:

1. Frontend React
   - Hiển thị UI login, dashboard, user online, incoming call, video call.
   - Gọi API backend để login/register.
   - Mở WebSocket tới backend để signaling.
   - Dùng WebRTC API của browser để gửi/nhận video và audio.

2. Backend Go
   - Xử lý auth/API.
   - Quản lý WebSocket connections.
   - Route signaling message giữa user A và user B.
   - Backend không xử lý video/audio thật.

3. WebRTC Media
   - Video/audio thật sự đi trực tiếp giữa browser với browser nếu P2P connect được.
   - Nếu P2P không connect được, media cần đi qua TURN server.

## Signaling

Signaling là bước hai browser trao đổi thông tin để bắt đầu WebRTC.

Trong project này signaling đi qua WebSocket:

```text
Browser A
  -> WebSocket /ws
  -> Backend Go signaling hub
  -> WebSocket /ws
  -> Browser B
```

Signal messages gồm:

- `call`: báo cho người nhận có cuộc gọi tới.
- `call-accepted`: người nhận bấm nghe.
- `call-rejected`: người nhận từ chối.
- `offer`: SDP offer từ caller.
- `answer`: SDP answer từ callee.
- `ice-candidate`: các network candidate để ICE thử kết nối.
- `call-ended`: kết thúc cuộc gọi.

Signaling chỉ giúp 2 browser "thỏa thuận cách kết nối". Nó không chuyển video/audio.

## SDP Offer Và Answer

SDP là thông tin mô tả media session:

- Browser có gửi video/audio không.
- Codec nào được hỗ trợ.
- Network/ICE info liên quan.

Luồng cơ bản:

```text
Caller tạo RTCPeerConnection
Caller createOffer()
Caller gửi offer qua WebSocket

Callee nhận offer
Callee setRemoteDescription(offer)
Callee createAnswer()
Callee gửi answer qua WebSocket

Caller nhận answer
Caller setRemoteDescription(answer)
```

Sau đó ICE sẽ tìm đường media thật sự.

## ICE

ICE là cơ chế WebRTC dùng để tìm đường kết nối media tốt nhất giữa 2 browser.

ICE thử nhiều loại candidate:

- `host`: địa chỉ local trong máy/mạng nội bộ.
- `srflx`: public IP/port tìm được qua STUN.
- `relay`: địa chỉ relay qua TURN.

Nếu 2 browser cùng máy hoặc cùng mạng đơn giản, ICE có thể connect bằng `host` hoặc `srflx`.

Nếu 1 laptop + 1 điện thoại nằm sau NAT khác nhau, WiFi/4G/firewall khác nhau, `host` và `srflx` có thể fail. Lúc này cần `relay` candidate từ TURN.

## STUN

STUN giúp browser biết public IP/port của nó khi đi ra internet.

Trong project hiện tại, frontend có default STUN:

```ts
stun:stun.l.google.com:19302
stun:stun1.l.google.com:19302
stun:stun2.l.google.com:19302
```

STUN hữu ích nhưng không đảm bảo media connect được trên mọi network.

STUN không relay video/audio. Nó chỉ giúp browser phát hiện public address.

## TURN

TURN là server relay media khi P2P không thành công.

Khi có TURN:

```text
Laptop browser <-> TURN server <-> Phone browser
```

TURN tốn bandwidth server vì video/audio đi qua TURN.

Nhưng với production WebRTC, TURN gần như bắt buộc để call ổn định giữa các network khác nhau.

## Tại Sao Cùng Máy Thì Được Nhưng Laptop + Điện Thoại Thì Không

Khi test 2 browser trên cùng 1 máy:

- Hai browser có thể tìm đường local/loopback/host candidate để nối với nhau.
- NAT gần như không phải vấn đề.
- Vì vậy video/audio hiện được.

Khi test laptop + điện thoại:

- Laptop và điện thoại có thể nằm trên network path khác nhau.
- Điện thoại có thể dùng 4G, CGNAT, firewall, hoặc WiFi client isolation.
- STUN không đủ để mở đường P2P.
- ICE fail hoặc không chọn được candidate pair.
- Kết quả: signaling vẫn OK, nhưng media không có hình/tiếng.

## Config TURN Trong Project

Project đã có sẵn runtime config TURN trong:

```text
frontend/src/config.ts
frontend/env-config.template.js
frontend/Dockerfile
```

Frontend đọc các biến:

```text
STUN_URLS
TURN_URLS
TURN_USERNAME
TURN_CREDENTIAL
```

Vì vậy hướng sửa ngày mai là cấp TURN thật, rồi set env/SSM cho frontend.

Ví dụ:

```bash
TURN_URLS=turn:turn.example.com:3478?transport=udp,turn:turn.example.com:3478?transport=tcp,turns:turn.example.com:5349?transport=tcp
TURN_USERNAME=your_turn_user
TURN_CREDENTIAL=your_turn_password
```

## Nếu Deploy Trên AWS

ALB đang xử lý HTTP/HTTPS và WebSocket:

```text
Browser
  -> HTTPS/WSS
  -> ALB
  -> ECS frontend/backend
```

ALB phù hợp cho:

- Web app HTTPS.
- API.
- WebSocket signaling.

ALB không phải cách tốt để chạy TURN UDP media relay.

Nếu tự host TURN, nên dùng:

- EC2 public chạy coturn, hoặc
- NLB hỗ trợ TCP/UDP phía trước coturn.

Cần mở port:

```text
UDP/TCP 3478
TCP 5349 nếu dùng turns
UDP relay range, ví dụ 49152-65535 hoặc range đã cấu hình trong coturn
```

## Lựa Chọn TURN

Có 2 hướng:

1. Managed TURN provider
   - Dễ nhất.
   - Chỉ cần lấy TURN URL, username, credential.
   - Set vào env/SSM rồi redeploy.

2. Tự chạy coturn
   - Rẻ hơn nếu dùng ít.
   - Cần cấu hình security group, domain, TLS nếu dùng `turns:`.
   - Cần monitor bandwidth.

## Cách Check Bằng Browser

Trên Chrome laptop:

```text
chrome://webrtc-internals
```

Sau đó thực hiện call với điện thoại.

Cần xem:

- `iceConnectionState`: nếu `failed` hoặc `disconnected` thì ICE không connect.
- Selected candidate pair:
  - Nếu không có candidate pair thì ICE fail.
  - Nếu có `relay` thì media đang đi qua TURN.
  - Nếu chỉ có `host`/`srflx` và fail trên network khác, khả năng cao cần TURN.

## Việc Cần Làm Ngày Mai

1. Chọn TURN provider hoặc tự deploy coturn.
2. Tạo TURN credentials.
3. Set `TURN_URLS`, `TURN_USERNAME`, `TURN_CREDENTIAL` cho frontend deployment.
4. Redeploy frontend/ECS task để `env-config.js` có giá trị mới.
5. Test laptop + điện thoại.
6. Kiểm tra `chrome://webrtc-internals` xem có selected candidate type `relay` khi cần.

## Ghi Chú Quan Trọng

- Backend Go không cần chuyển video/audio.
- WebSocket `/ws` chỉ là signaling.
- Video/audio WebRTC không đi qua ALB/backend trừ khi xây SFU/media server riêng.
- Với app 1-1 video call, TURN là cách nhẹ nhất để production ổn định hơn.
- Nếu sau khi có TURN vẫn không có hình/tiếng, kiểm tra console log và `chrome://webrtc-internals` trước.
