# Tính năng Windows Service (POS chạy Windows)

## 1. Bối cảnh & vai trò

Máy POS tại cửa hàng chạy Windows, trên đó có một agent (Windows Service) chạy nền. Agent này chịu trách nhiệm:

- Đăng ký/kích hoạt máy với hệ thống
- Tự động tải & cài đặt các app POS (KDS, Dispatch, Taker, TakeAwayMonitor, TakerClient, PriceDisplay)
- Tự cập nhật chính bản thân nó
- Gửi log về server

Phần em làm là toàn bộ API backend phục vụ agent đó, nằm trong `cpos-microservice-system` (Go + Gin + MongoDB). Phần agent (installer, script chạy lúc install) do team khác làm — nhưng đọc contract API là suy ra được agent phải làm gì.

**Sơ đồ:**

```text
[CMS admin]  --tạo activation code, cấu hình version/lịch-->  [MongoDB]
                                                                  ^
                                                                  |
[Windows Service agent trên máy POS] --REST /v1/windows-device/*--> [system-service]
                                     <-- version + download URL + hash + lịch --
                                     --tải file--> [cms-order-pos: /app-version-v2, /windows-service-update]
```

---

## 2. Data model (4 collection MongoDB)

### `windows_service_activation_code` — `models/windows_service_activation_code.go`

Mã kích hoạt do CMS phát ra, gắn sẵn client + store + danh sách app.

| Field | Ý ngh |
| --- | --- |
| `code` | mã kích hoạt |
| `client_uuid`, `store_uuid`, `store_code` | máy này thuộc tenant/ |
| `apps` | danh sách app sẽ auto |
| `activated_at`, `activated_device` | dấu vết đã dùng (dùng |

### `windows_device` — `models/windows_device.go`

Máy POS đã được kích hoạt.

Key fields: `uuid`, `client_uuid`, `client_name`, `mac_address` (định danh thiết bị), `store_uuid`/`store_code`, `os`, `ip`, `location`, `auto_start_apps`, `approved_at`, `is_active`/`is_delete` (soft delete).

Index: `uuid` unique, `mac_address` unique, cùng các index phụ clactive, `is_delete`, `approved_at`.

### `windows_service_update` — `models/windows_service_update.go`

Bản build của chính con Windows Service (để nó tự update).

`version`, `file`, `file_hash`, `time` (mốc bắt đầu được phép update, `download_url` (không lưu DB — ghép runtime từ config).

### `windows_service_log` — `models/window_service_log.go`

Log hoạt động agent gửi về: `action`, `app_name`, `version_name`, a, `error`, `mac_address`, `client_uuid`, `store_code`, `created_at`. Index compound (`client_uuid`, `store_code`).

> **Chi tiết kỹ thuật đáng nói:** cả 3 model đều tạo index lazy + `sync.Once` ngay trong hàm lấy collection — index chỉ tạo 1 lần mỗi process, không cần migration script riêng.

---

## 3. API endpoints — `routes/windowsDevice.go`

Tất cả nằm dưới group `/v1/windows-device`:

| Method | Path | Ha | Mục đích |
| --- | --- | --- | --- |
| POST | `/add-new-device` | `windowsDeviceController` | Đăng ký máy (legacy, code ghi rõ `Todo: Not use in the future`) |
| POST | `/request-approve` | `windowsDeviceController.RequestApprove` | Kích hoạt máy bằng activation code |
| POST | `/get-app-version` | `AppVersionV2Controller.GetLatestVersion` | Lấy version app + URL + hash + lịch |
| POST | `/get-app-update-schedule` | `AppVersionV2Controller.GetUpdateSchedule` | Lấy lịch update (không dùng nữa, đã gộp vào `get-app-version`) |
| POST | `/windows-service-update` | `WindowServiceController` | Agent tự check update chính nó |
| POST | `/windows-service-log` | `WindowServiceController` | Nhận log từ agent |

Header dùng chung: `X-CLIENT-ID` (tenant), `X-STORE-CODE`, `X-DEVICE-ID` (MAC address).

### Về authentication — điểm quan trọng cần nắm chắc

Trong `routes/router.go:51`, `windowsDeviceAPIs(apiV1)` được gọi **trước** dòng `apiAuth := apiV1.Use(middleware.Authentication())` (dòng 53). Trong Gin, group con chụp lại chain middleware tại thời điểm tạo → nhóm API nàication.

Đây là chủ ý: Windows Service chạy nền lúc máy khởi động, khôkhông có `X-USER-TOKEN` để verify qua gRPC Auth service. Thayvào đó danh tính dựa trên `X-CLIENT-ID` + `mac_address` + activation code.

Middleware thực sự áp dụng: `Newrelic()` (APM), `RequestLog()` (log toàn bộ request/response, chạy async trong goroutine), `gin.Recovery()`.

---

## 4. Các flow chính

### Flow A — Kích hoạt máy (`/request-approve`)

Agent gửi: `{ device_id (MAC), os, activation_code }`

1. Tiêu thụ mã kích hoạt atomically:

   ```go
   FindOneAndUpdate(
     {code: req.ActivationCode, activated_at: nil, is_active:1,
     {$set: {activated_at: now, activated_device: req.DeviceID}}
   )
   ```

   Filter `activated_at: nil` + `FindOneAndUpdate` là thao tác atomic ở tầng MongoDB → hai máy cùng nhập một mã thì chỉ một máy thắng, máy còn lại nhận `403` / code `1015 Activation Code is invalid`. Đây là chỗ chống hành find rồi update sẽ có lỗ hổng.

2. Lấy Client theo `client_uuid` trong mã → không có thì `403` / `1016 Client not found`.

3. Upsert device theo `{client_uuid, mac_address}`:
   - `$set`: `store_uuid`, `store_code`, `auto_start_apps`, `updated_at`
   - `$setOnInsert`: `uuid`, `client_name`, `created_at`, `approved_at`,
   - `FindOneAndUpdate` + `SetUpsert(true)` + `SetReturnDocument(After)` → trả về document sau update.

   **Ý nghĩa thiết kế:** idempotent. Cài lại máy / đổi cửa hàng / đổi bộ app chỉ cần cấp mã mới → record cũ được update chứ không sinh bản ghi trùng, `uuid` và `created_at` giữ nguyên (nhờ `$setOnInsert`).

4. Trả `200` kèm nguyên device → agent lưu lại `store_code`, `auto_start_apps`.

So với flow cũ (`/add-new-device`): trước đây máy tự đăng ký, `approved_at = nil`, rồi admin vào CMS duyệt thủ công; nếu quá 30 phút chưa duyệt thì timeout. Flow mới bằng activation code duyệt luôn tại thời điset trong `$setOnInsert`), bỏ được bước thao tác tay của admin — đây chính là lý do route cũ được đánh dấu "not use in the future". Code xử lý trạng thái chờ duyệt vẫn còn ở `GetLatestVersion` để tương thích ngược với máy cũ.

### Flow B — Lấy version app (`/get-app-version`) — endpoint phức t

`controllers/appVersionV2.go:26`

1. Tìm device theo `{client_uuid (header), mac_address}` — Findelete → máy bị xóa mềm sẽ `404`.
2. Kiểm tra trạng thái duyệt:
   - `approved_at == nil` và quá 30 phút từ `created_at` → `403` / 1ice has timed out
   - `approved_at == nil` (còn trong 30 phút) → `422` / `1013 Device is waiting for approval` → agent hiểu là cứ retry
   - Hai mã lỗi tách bạch để agent biết khi nào nên retry, khiviên cửa hàng.
3. Lấy `app_version_v2` theo `{client_uuid, store_uuid}` → cấu hình version theo từng cửa hàng, không phải toàn hệ thống. Cho phép rollout theo store.
4. Gom `app_version_manager_uuid` của 6 app → query 1 lần `$in` v(tránh N+1).
5. Nếu số bản ghi trả về ≠ số UUID yêu cầu → `400` / `MissingVersion` (fail-fast, không cho agent cài nửa vời).
6. Lấy thêm:
   - `notify_version_v2` → `download_time`, `install_time` (giờ được phép tải / giờ được phép cài)
   - collection config key `windows_service_cron_schedule` → JSOemote config: server điều khiển tần suất cron của agent màkhông cần deploy lại agent.
7. Ghép response, `version_url` = `downloadAppDomainV2` + `"/"` + ftart_apps, `store_code`.

**Response mẫu:**

```json
{
  "kds": {"version_code":"...","version_name":"...","version_","file_hash":"..."},
  "dispatch": {...}, "taker": {...}, "takeaway_monitor": {...},
  "takerclient": {...}, "price_display": {...},
  "auto_start_apps": ["kds","taker"],
  "schedule": {"download_time":"02:00","install_time":"03:00"
  "cronjob_schedule": {...},
  "store_code": "JP001"
}
```

`file_hash` là mấu chốt: agent so hash local với hash server → biết có cần tải không, và verify file sau khi tải (chống file hỏng/bị can thiệp).

### Flow C — Agent tự update (`/windows-service-update`)

`controllers/windowService.go:60`

Agent gửi: `{ file_hash }` // hash của binary agent đang chạy

1. `GetLatest()` — sort `_id: -1`, filter `is_active`/`is_delete` → bản mới nhất theo thứ tự insert (ObjectId tăng dần theo thời gian), không so sánh semv
2. Ghép `download_url` từ config `downloadWindowsService`.
3. Nếu `file_hash` rỗng (lần đầu, agent chưa biết hash mình) → trả version ngay, bỏ qua lịch.
4. Nếu có `file_hash` → chỉ trả bản mới khi thỏa cả 3:
   - `file_hash` client ≠ `file_hash` server (so sánh lowercase, tránh sai khác hoa/thường)
   - `now > start_time`
   - `now < start_time + 24h`

   `start_time` parse từ field `time` theo timezone `Asia/Tokyo` (`constant.JP_TIMEZONE`), format `2006-01-02 15:04`.
5. Không thỏa → `200` với message `"There is no update"` (chứ khô

**Ý nghĩa cửa sổ 24h:** admin đặt mốc thời gian, toàn bộ fleet chốc đó. Máy nào đang bận/tắt qua khỏi cửa sổ thì bỏ qua đợt này — tránh update ngoài tầm kiểm soát vào giờ cao điểm bán hàng. Timezone cố định Nhật vì thị trường chính là JP.

**Design decision đáng nói:** mọi lỗi (không tìm thấy bản update, parse time lỗi) đều trả `200` + `"no update"` thay vì `5xx`. Lý do: agent chạy nền không người trực, fail-safe — thà không update còn hơn đẩy agent và

### Flow D — Gửi log (`/windows-service-log`)

Agent POST `{action, app_name, version_name, app_file_url, app` bổ sung `client_uuid`, `store_code`, `mac_address` từ header (không tin body) + `created_at`, rồi insert.

Đây là log audit cho toàn bộ vòng đời cài đặt: download → verify hash → install → start, kèm lỗi nếu có. Khi cửa hàng báo "app không lên", supporttra collection này theo (`client_uuid`, `store_code`) — đúng comp

---

## 5. Vòng đời đầy đủ trên một máy POS

1. Nhân viên cài Windows Service (installer — phần em không làm)
2. Nhập activation code do CMS cấp
   → `POST /request-approve` → device được tạo + approved ngay
3. Agent chạy cron (tần suất do `cronjob_schedule` server đẩy x

```text
   ├─ POST /get-app-version
   │    → so file_hash local vs server
   │    → đến download_time thì tải từ downloadAppDomainV2
   │    → verify hash
   │    → đến install_time thì cài
   │    → auto-start các app trong auto_start_apps
   │    → POST /windows-service-log ở mỗi bước
   └─ POST /windows-service-update (gửi hash của chính nó)
        → nếu trong cửa sổ 24h & hash khác → tải bản mới → tự thay thế
```

---

## 6. Những điểm thiết kế đáng nêu khi phỏng vấn

**Điểm mạnh / chủ ý:**

1. Atomic one-time activation code bằng `FindOneAndUpdate` với filter `activated_at: nil` — chống double-activation ở tầng DB, không cần lock.
2. Upsert idempotent theo (`client_uuid`, `mac_address`) với $setcài lại máy không sinh rác, giữ nguyên identity.
3. Hash-based diffing thay vì so version string — vừa quyết định có cần tải không, vừa verify integrity sau tải.
4. Rollout có kiểm soát: version theo từng store (app_versionre_uuid), lịch download/install riêng, cửa sổ update 24h.
5. Remote config (`cronjob_schedule`) — đổi hành vi agent mà không cần release agent mới.
6. Download URL không lưu DB, ghép runtime từ config theo môit data chạy được ở dev/test/prod và nhiều region(JP/PHI/SIN/ID/AA).
7. Fail-safe cho máy không người trực: endpoint self-update l
8. Mã lỗi nghiệp vụ riêng (1013/1014/1015/1016) để agent phân biệt "chờ và retry" với "dừng, báo người".
9. Không tin body cho thông tin định danh — `client_uuid`/store từ header.

**Hạn chế / điều em sẽ cải thiện (nên chủ động nói, phỏng vấn r**

1. Không có authentication thực sự trên nhóm API này — ai biết `X-CLIENT-ID` + một MAC hợp lệ đều gọi được `/get-app-version`. Cải thiện: sau khi activate thì cấp device token/API key, hoặc mTLS, hoặc ít nhấđã có sẵn `X-API-KEY` trong codebase nhưng chưa gắn vào groupnày).
2. `mac_address` unique toàn cục nhưng filter upsert lại là (clnếu cùng một MAC xuất hiện ở client khác sẽ dính duplicate key error. Nên đổi unique index thành compound (`client_uuid`, `mac_address`).
3. Upsert không lọc `is_delete` (khác với `FindOne`/`Find` vốn tự ckhi activate lại sẽ được `$set` nhưng `is_delete` vẫn = 1, và `$setOnInsert` không chạy → máy "kích hoạt thành công" nhưng `/get-app-version` trả `404`. Nên đưa `is_active`/`is_delete` vào `$set`.
4. Nil pointer ở `appVersionV2.go:117-121`: lúc build danh sáchon.TakeAwayMonitor != nil, nhưng trong switch lại truy cậpthẳng `appVersion.TakeAwayMonitor.AppVersionManagerUUID` → panic nếu nil (được `gin.Recovery()` đỡ thành `500`). Nên guard nhất quán hoặc dùng map thay switch.
5. `GetLatest()` của `windows_service_update` không filter `client_uuid` dù model có field đó → bản update agent hiện là global cho mọi tenant. Có chủ ýhay là thiếu sót thì cần làm rõ.
6. `windows_service_log` không có TTL index — log thiết bị tăng vô hạn. Nên thêm TTL (vd. 90 ngày) hoặc archive.
7. `GetLatest()` sort theo `_id` — phụ thuộc thứ tự insert, không thứ tự là rollout sai bản.
8. Hard-code timezone Nhật — hệ thống multi-region, nên lấy timezone theo client/store.
9. `RequestLog` middleware log toàn bộ header — với API có tokeng tin trong log.

---

## 7. Câu hỏi phỏng vấn hay gặp & cách trả lời

**"Làm sao đảm bảo một activation code chỉ dùng được một lần khi nhiều máy gọi đồng thời?"**

→ `FindOneAndUpdate` với `activated_at: nil` trong filter. MongoDdocument, máy thứ hai không match được filter nên nhận nil →trả `403`. Không cần distributed lock.

**"Sao không dùng version number mà dùng file hash?"**

→ Hash vừa để so sánh "có khác không", vừa để verify file sauó thể bị build lại cùng số version; hash thì không nói dối.

**"Agent gọi API liên tục thì tải server thế nào?"**

→ Tần suất do server điều khiển qua `cronjob_schedule`; các query đều đi qua index (`mac_address`, `client_uuid`); `get-app-version` gom 6 version bằng mộquery `$in` thay vì 6 query.

**"Nếu update lỗi giữa chừng thì sao?"**

→ Agent log về `/windows-service-log` kèm error; do so sánh bằng hash nên lần cron sau nó tự phát hiện file chưa đúng và tải lại — về bản chất là idempotent retry, không cần state machine phía server.

**"Tại sao nhóm API này không qua JWT?"**

→ Service chạy nền lúc boot, không có user session. Trade-off có ý thức; và em nêu luôn hướng cải thiện bằng device token sau activation.

---

## 8. Về phần em không làm (trả lời trung thực)

"Phần installer/script chạy lúc install và bản thân con Windo làm, em phụ trách toàn bộ API backend. Nhưng qua contract API thì em nắm được agent phải: lưu activation code & MAC, chạy cron theo `cronjob_schedule`, so hash để quyết định tải, verify hash sau tải, tôn trọng `download_time`/`install_time`, auto-start các app trong auto_stavề server."

---

## 9. Bản song song qua gRPC — `grpc/service/app_version_v2.go`

Đây là phần note đang thiếu hẳn. Ngoài REST cho agent, service này còn expose `AppVersionV2.GetLatestScheduleVersion` qua gRPC (`grpc/proto/app_version_v2/`) cho service nội bộ khác gọi, không phải cho máy POS.

Điểm khác biệt so với REST `/get-app-version`:

| | REST `/get-app-version` | gRPC `GetLatestScheduleVersion` |
| --- | --- | --- |
| Người gọi | Agent trên máy POS | Microservice nội bộ (CMS) |
| Định danh store | Suy từ `mac_address` → device → `store_uuid` | Truyền thẳng `req.StoreUuid` |
| `client_uuid` | HTTP header `X-CLIENT-ID` | gRPC metadata `x-client-id` (`metadata.FromIncomingContext`) |
| Kiểm tra device/approve | Có | Không (không có khái niệm device) |
| Trả về | + `file_hash`, `auto_start_apps`, `cronjob_schedule` | + `schedule_time` (`notifyVersionV2.StartTime`) |

**Nói được gì từ đây:**

- Cùng một nguồn dữ liệu (`app_version_v2` + `app_version_manager_v2` + `notify_version_v2`) phục vụ hai kênh với hai đối tượng khác nhau — thiết bị ngoài internet đi REST, service nội bộ đi gRPC (nhanh hơn, có contract chặt qua protobuf).
- Điểm yếu chủ động nêu: logic gom version bị duplicate gần như nguyên xi giữa `controllers/appVersionV2.go:59-132` và `grpc/service/app_version_v2.go:37-89`. Nếu refactor, em sẽ tách một service layer (vd. `AppVersionService.Resolve(clientUUID, storeUUID)`) để cả REST controller và gRPC handler cùng gọi — vừa hết trùng lặp, vừa unit test được (hiện controller gắn chặt với `gin.Context` nên rất khó test).
- Lưu ý: gRPC bản này cũng dính đúng bug nil-pointer ở switch (`app_version_v2.go:81-86`) — chứng tỏ duplicate code làm bug nhân đôi. Đây là ví dụ rất "ăn điểm" khi phỏng vấn hỏi "vì sao nên tránh copy-paste?"
