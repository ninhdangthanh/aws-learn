# TURN Coturn AWS Setup

## Vì Sao Cần File Này

Nếu app chạy được khi test 2 browser cùng máy nhưng fail khi laptop + điện thoại khác mạng, vấn đề thường nằm ở WebRTC media path:

```text
Browser A <-> Browser B
```

STUN chỉ giúp browser tìm public IP/port. Nhiều mạng 4G/5G, CGNAT, symmetric NAT hoặc firewall vẫn không cho P2P thành công.

TURN giải quyết bằng cách relay media:

```text
Browser A <-> TURN server <-> Browser B
```

## Kiến Trúc Khuyến Nghị

Không đặt TURN sau ALB. ALB phù hợp HTTP/HTTPS/WebSocket, không phù hợp TURN UDP relay.

Hướng đơn giản nhất:

```text
Browser
  -> turn:turn.example.com:3478 UDP/TCP
  -> EC2 public chạy coturn
```

App web/API vẫn đi qua ALB:

```text
Browser
  -> https://call.example.com
  -> ALB
  -> ECS frontend/backend
```

Media relay đi qua TURN riêng:

```text
Browser A
  -> TURN EC2
  -> Browser B
```

## 1. Tạo EC2 Cho TURN

Gợi ý:

- Region: cùng region app, ví dụ `ap-southeast-1`.
- Instance type demo: `t3.micro` hoặc `t3.small`.
- OS: Ubuntu 22.04/24.04 LTS.
- Gắn Elastic IP để IP không đổi khi restart.

Security group inbound:

```text
TCP 22             từ IP của bạn
UDP 3478           từ 0.0.0.0/0
TCP 3478           từ 0.0.0.0/0
UDP 49152-65535    từ 0.0.0.0/0
```

Nếu dùng `turns:` TLS:

```text
TCP 5349           từ 0.0.0.0/0
```

Demo ban đầu có thể dùng `turn:` port `3478` trước. Sau đó thêm `turns:` nếu muốn TLS cho TURN.

## 2. Tạo DNS Cho TURN

Trong Route 53 hosted zone, tạo:

```text
A record
turn.example.com -> Elastic IP của EC2 TURN
```

Ví dụ:

```text
turn.ninh-video-call-demo.food -> 13.x.x.x
```

## 3. Cài Coturn

SSH vào EC2:

```bash
sudo apt update
sudo apt install -y coturn
```

Bật service:

```bash
sudo sed -i 's/#TURNSERVER_ENABLED=1/TURNSERVER_ENABLED=1/' /etc/default/coturn
```

## 4. Cấu Hình Coturn

Backup file cũ:

```bash
sudo cp /etc/turnserver.conf /etc/turnserver.conf.bak
```

Tạo config:

```bash
sudo tee /etc/turnserver.conf >/dev/null <<'EOF'
listening-port=3478
fingerprint
lt-cred-mech

realm=turn.example.com
server-name=turn.example.com

user=videocall:CHANGE_ME_STRONG_PASSWORD

external-ip=YOUR_ELASTIC_IP

min-port=49152
max-port=65535

no-cli
no-multicast-peers
no-loopback-peers
stale-nonce=600

log-file=/var/log/turnserver.log
simple-log
EOF
```

Thay:

```text
turn.example.com
YOUR_ELASTIC_IP
CHANGE_ME_STRONG_PASSWORD
```

Restart:

```bash
sudo systemctl restart coturn
sudo systemctl status coturn --no-pager
```

Xem log:

```bash
sudo tail -f /var/log/turnserver.log
```

## 5. Cập Nhật SSM Parameters Cho ECS Frontend

App đã đọc TURN runtime config qua `/env-config.js`.

Cập nhật SSM:

```bash
aws ssm put-parameter \
  --name /videocall/TURN_URLS \
  --type SecureString \
  --value "turn:turn.example.com:3478?transport=udp,turn:turn.example.com:3478?transport=tcp" \
  --overwrite \
  --region ap-southeast-1

aws ssm put-parameter \
  --name /videocall/TURN_USERNAME \
  --type SecureString \
  --value "videocall" \
  --overwrite \
  --region ap-southeast-1

aws ssm put-parameter \
  --name /videocall/TURN_CREDENTIAL \
  --type SecureString \
  --value "CHANGE_ME_STRONG_PASSWORD" \
  --overwrite \
  --region ap-southeast-1
```

Giữ:

```text
ICE_TRANSPORT_POLICY=all
```

Khi cần debug ép tất cả media đi qua TURN, đổi thành:

```text
ICE_TRANSPORT_POLICY=relay
```

Nếu `relay` chạy được thì TURN đúng. Sau khi xác nhận, có thể để `all` để browser tự chọn đường tốt nhất.

## 6. Redeploy ECS Service

Vì frontend container sinh `/env-config.js` lúc container start, sau khi đổi SSM cần force deploy:

```bash
aws ecs update-service \
  --cluster videocall-cluster \
  --service videocall-service \
  --force-new-deployment \
  --region ap-southeast-1
```

Sau khi task mới healthy, mở browser:

```text
https://call.example.com/env-config.js
```

Phải thấy:

```js
TURN_URLS: "turn:turn.example.com:3478?transport=udp,..."
TURN_USERNAME: "videocall"
TURN_CREDENTIAL: "..."
ICE_TRANSPORT_POLICY: "all"
```

## 7. Test TURN

### Browser Console

Frontend sẽ log:

```text
WebRTC ICE config
```

Cần thấy:

```text
turnServers > 0
hasTurnCredentials: true
```

Khi gọi, console sẽ log candidate:

```text
ICE candidate gathered: relay udp
```

Nếu không có `relay`, browser chưa lấy được TURN candidate.

### chrome://webrtc-internals

Mở trên laptop:

```text
chrome://webrtc-internals
```

Gọi với điện thoại khác mạng.

Kiểm tra:

- `iceConnectionState` chuyển `connected` hoặc `completed`.
- Selected candidate pair có candidate type `relay` khi mạng cần TURN.
- Nếu `ICE_TRANSPORT_POLICY=relay`, selected pair bắt buộc phải là `relay`.

### Coturn Log

Trên EC2:

```bash
sudo tail -f /var/log/turnserver.log
```

Khi có call đi qua TURN, log sẽ có allocation/session traffic.

## 8. Lỗi Thường Gặp

### Không thấy relay candidate

Kiểm tra:

- `https://call.example.com/env-config.js` có TURN chưa.
- ECS task frontend đã redeploy chưa.
- Security group EC2 TURN đã mở UDP/TCP 3478 chưa.
- DNS `turn.example.com` trỏ đúng Elastic IP chưa.
- Coturn service đang chạy chưa.

### relay candidate có nhưng call vẫn fail

Kiểm tra:

- UDP relay range `49152-65535` đã mở trên security group chưa.
- `external-ip` trong `/etc/turnserver.conf` đúng Elastic IP chưa.
- Username/password trong SSM đúng với `user=...` trong coturn chưa.
- Browser console có `ICE candidate error` không.

### Dùng mạng công ty vẫn fail

Một số mạng chặn UDP. Cần có TCP TURN:

```text
turn:turn.example.com:3478?transport=tcp
```

Nếu vẫn bị chặn, cân nhắc thêm `turns:` qua TCP 5349.

## 9. Có Cần `turns:` Không?

Không bắt buộc cho bước fix đầu tiên. Trang web HTTPS vẫn có thể dùng `turn:` UDP/TCP.

Nhưng `turns:` hữu ích khi:

- Network chặn UDP/TCP 3478.
- Muốn TURN qua TLS TCP 5349.
- Muốn production cứng hơn.

Khi thêm `turns:`, cần certificate TLS trên coturn và mở TCP 5349.

## Checklist Fix

1. Tạo EC2 public cho coturn.
2. Gắn Elastic IP.
3. Tạo DNS `turn.example.com`.
4. Mở SG: UDP/TCP 3478 và UDP relay range.
5. Cài và cấu hình coturn.
6. Update SSM `TURN_URLS`, `TURN_USERNAME`, `TURN_CREDENTIAL`.
7. Redeploy ECS frontend.
8. Mở `/env-config.js` xác nhận TURN đã tới browser.
9. Test laptop + điện thoại khác mạng.
10. Kiểm tra `chrome://webrtc-internals` và coturn logs.
