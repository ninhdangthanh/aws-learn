# PART_SIZE_BYTES — tác dụng phía Backend

`PART_SIZE_BYTES` phía BE chỉ có **một tác dụng duy nhất**: là con số "gợi ý kích
thước mỗi part" mà backend **trả về cho FE** ở API `init`. Backend **KHÔNG tự cắt
file** — việc cắt file do FE làm.

## Luồng trong code

1. `internal/config/config.go:28` — đọc env, default **10 MiB** (`10*1024*1024`).
   Env sai định dạng → fallback về default.
2. `main.go:39` — inject vào handler: `handler.New(svc, cfg.KeyPrefix, cfg.PartSize)`.
3. `internal/handler/handler.go:72` — API `init` trả nguyên giá trị về client:
   `initResponse{..., PartSize: h.partSize}`.

## FE dùng partSize để làm gì

- Tính số part: `numParts = ceil(fileSize / partSize)`
- Cắt file thành các chunk `partSize` byte để PUT lên presigned URL

## ⚠️ Lưu ý

- **BE không validate part size theo giới hạn của S3.** S3 yêu cầu mỗi part (trừ
  part cuối) phải ≥ **5 MiB**. Default 10 MiB an toàn, nhưng nếu set
  `PART_SIZE_BYTES=1048576` (1 MiB) → FE cắt theo đó → S3 **reject lúc `complete`**
  với lỗi `EntityTooSmall`. Backend hiện KHÔNG chặn trường hợp này.
- Đây chỉ là giá trị "suggested". FE về lý thuyết có thể cắt kích thước khác, miễn
  là số `partNumbers` xin presign khớp với số part upload/complete. Đúng thiết kế
  thì FE nên tuân theo giá trị BE trả về.
- Part size lớn hơn → ít part, ít request/presign hơn; nhưng retry một part tốn
  băng thông hơn (upload lại cả chunk lớn). 10 MiB là mức cân bằng hợp lý.

## Tham chiếu
- `backend/internal/config/config.go` (đọc env, default 10 MiB)
- `backend/internal/handler/handler.go:72` (trả về client ở API init)
- `backend/.env.example` (khai báo `PART_SIZE_BYTES`)

---

# Không gọi abort API (reload / network fail) thì sao?

Nếu **không gọi `abort`** (user reload tab, tắt trình duyệt, mất mạng, JS crash...):

1. **Upload không tự huỷ.** `uploadId` cùng các part đã upload xong vẫn **nằm lại
   trong S3** ở trạng thái "in-progress multipart upload". Chúng **không hiện ra**
   khi `ListObjects` (vì chưa `complete`) → rất dễ bị "vô hình".
2. **S3 vẫn tính phí lưu trữ** cho các part dang dở — chi phí ẩn, bucket "trông
   trống" nhưng vẫn bị charge.
3. **Không tự hết hạn.** Part orphan tồn tại **vĩnh viễn** cho tới khi có thứ gì đó
   dọn. Presigned URL thì hết hạn (900s theo config), nhưng bản thân multipart
   upload thì KHÔNG.

## Cách xử lý (cần cả 2 tầng)

### 1. Bắt buộc: S3 Lifecycle Rule (dọn phía server)
Lưới an toàn thực sự, độc lập với FE — luôn nên bật vì `abort` từ FE không bao giờ
đáng tin 100%:

```json
{
  "Rules": [{
    "ID": "abort-incomplete-multipart",
    "Status": "Enabled",
    "Filter": { "Prefix": "uploads/" },
    "AbortIncompleteMultipartUpload": { "DaysAfterInitiation": 7 }
  }]
}
```
→ S3 tự động abort mọi upload dở dang sau 7 ngày.

### 2. Nên có: Cleanup / resume phía client
- **Best-effort abort:** dùng `navigator.sendBeacon()` trong `beforeunload` /
  `pagehide` để cố gửi abort khi đóng tab (không đảm bảo 100% nhưng bắt được phần
  lớn trường hợp reload/close).
- **Resume:** lưu `{ key, uploadId, parts đã xong }` vào `localStorage`. Khi quay
  lại có thể `ListParts` để biết part nào đã xong và upload tiếp phần thiếu thay vì
  làm lại từ đầu.

## Về code hiện tại
Backend chỉ có endpoint `abort` **chủ động** (`handler.go:161`) — KHÔNG có cơ chế
dọn tự động. Nếu FE không gọi abort thì **không có gì dọn** part orphan. → Lifecycle
rule là **bắt buộc**, không phải optional.

---

# Thanh progress trên UI (FE)

Đi từ dưới (đo byte) lên trên (vẽ thanh). Code đã triển khai đầy đủ trong
`frontend/src/multipartUpload.ts` + `frontend/src/App.tsx`.

## 1. Đo tiến độ thật của từng part — `XMLHttpRequest`
Phải dùng **`XMLHttpRequest`** chứ KHÔNG phải `fetch`, vì `fetch()` không báo được
tiến độ upload (request body). Chỉ XHR có `xhr.upload.onprogress`
(`multipartUpload.ts:156`):

```ts
xhr.upload.onprogress = (e) => {
  if (e.lengthComputable) onProgress(e.loaded); // byte đã gửi của part này
};
```

## 2. Cộng dồn tiến độ nhiều part song song
Upload 4 part song song (`concurrency: 4`) → không thể chỉ đếm 1 part. Dùng mảng
`loadedPerPart`, mỗi part ghi số byte của riêng nó rồi cộng tất cả
(`multipartUpload.ts:70-79`):

```ts
const loadedPerPart = new Array(plans.length).fill(0);
const emitProgress = () => {
  opts.onProgress(loadedPerPart.reduce((a, b) => a + b, 0), total); // tổng loaded / total
};
```
Part nào báo `onProgress(loaded)` → cập nhật ô của part đó → emit lại tổng. Khi part
xong set cứng `loadedPerPart[idx] = kích thước part` để tránh sai số (dòng 105).

## 3. Đẩy lên React state — `App.tsx:37`
```ts
onProgress: (l, t) => { setLoaded(l); setTotal(t); }
```

## 4. Tính % và vẽ thanh (CSS width)
```ts
const percent = total > 0 ? Math.min(100, (loaded / total) * 100) : 0;
```
```tsx
<div className="progress-bar">
  <div className="progress-fill" style={{ width: `${percent}%` }} />
</div>
```
Thanh = 2 div lồng nhau: div ngoài là track, div trong (`progress-fill`) có
`width = percent%`. `Math.min(100, …)` chặn vượt 100%.

## Lưu ý / nâng cấp
- **`e.lengthComputable`**: luôn check trước khi đọc `e.loaded`.
- **Reset khi retry**: part fail → retry gọi `onProgress(0)` (dòng 145) để trừ lại
  phần đếm hụt, tránh progress "nhảy lùi".
- **Progress theo từng part**: `multipartUpload.ts` đã expose callback
  `onPartStatus(partNumber, "uploading"|"done"|...)` (dòng 8) nhưng `App.tsx` CHƯA
  dùng. Muốn hiển thị lưới trạng thái từng chunk chỉ cần lắng nghe callback này,
  không phải sửa tầng upload.
- **Tốc độ + ETA**: lưu `loaded` + timestamp giữa 2 lần `onProgress`, tính
  `delta bytes / delta time`.

## Tham chiếu
- `frontend/src/multipartUpload.ts` (đo + cộng dồn progress, XHR)
- `frontend/src/App.tsx:21,100-108` (state, tính %, render thanh)

## Idea chính (tóm gọn)
**Dùng `XMLHttpRequest` để đo số byte đã thực sự gửi lên trong quá trình upload.**
Cần chính xác 3 điểm:

1. **Đo byte đi thẳng lên S3, KHÔNG qua backend.** Part được FE `PUT` trực tiếp lên
   S3 qua presigned URL. XHR đo chính request đó (byte browser → S3).

2. **Đo bằng `xhr.upload.onprogress`, KHÔNG phải `xhr.onprogress`:**

   | Sự kiện | Đo cái gì |
   |---------|-----------|
   | `xhr.onprogress` | byte **tải xuống** (response về) |
   | `xhr.upload.onprogress` | byte **tải lên** (request body) ✅ |

   `e.loaded` = đã gửi được bao nhiêu byte của part này.

3. **Phải XHR chứ không dùng `fetch`:** `fetch()` không có cơ chế báo tiến độ upload
   body → đây là lý do kỹ thuật DUY NHẤT khiến bước upload part không dùng fetch.

Toàn bộ = XHR đo byte upload từng part → cộng dồn → chia tổng size file → ra %.

⚠️ **`e.loaded` = byte đã rời browser / TCP nhận, KHÔNG phải "S3 đã lưu xong".** Nên
khi thanh đạt 100% nghĩa là "đã gửi hết lên", rồi mới chờ S3 trả `ETag` xác nhận.
Với file lớn, khoảng chờ giữa "100%" và "done" là bình thường.

---

# Có gộp chung API `init` với `presign-parts` không?

**Về kỹ thuật thì gộp được, nhưng KHÔNG nên** — tách ra là chủ đích thiết kế.

Sau khi `init` có `key` + `uploadId` và biết `size` + `partSize`, backend hoàn toàn
tính được số part (`numParts = ceil(size / partSize)`) → có thể sinh luôn tất cả
presigned URL và trả về trong 1 response. Nhưng làm vậy sẽ mất khả năng xử lý retry.

## Lý do cốt lõi để tách: part fail hoặc URL hết hạn

Presigned URL chỉ là "vé tạm thời" cho **một** thao tác PUT, có TTL (900s theo
config). Trong khi upload file lớn, một part rất dễ:
- **Upload lỗi** (mạng đứt giữa chừng), hoặc
- **URL hết hạn** trước khi tới lượt được upload (file nhiều GB, hàng nghìn part).

Khi đó client **chỉ cần gọi lại `presign-parts` với đúng part number bị lỗi** (vd
`partNumbers: [7]`) → nhận URL mới → PUT lại riêng part đó. Không đụng gì tới
`uploadId` hay các part khác đã xong.

Ba điều làm việc retry lẻ này an toàn:

1. **`uploadId` sống lâu** — tồn tại tới khi `complete` hoặc `abort`, KHÔNG hết hạn
   theo TTL như presigned URL. Nên xin URL mới bao nhiêu lần cũng được.
2. **Part number idempotent** — upload lại cùng `partNumber` sẽ **ghi đè** part cũ
   trên S3, không tạo trùng. ETag cuối cùng là cái dùng ở `complete`.
3. **Presigned URL độc lập với upload** — hết hạn thì xin vé mới, bản thân multipart
   upload (`uploadId`) không bị ảnh hưởng.

Đây chính là lý do `presignRequest.PartNumbers` là **mảng** (`handler.go:80`) chứ
không phải một số — phục vụ được cả 2 tình huống: xin tất cả lúc đầu, và xin lẻ vài
part khi retry. Comment ở `handler.go:88` nói rõ: *"The client can ask for all parts
at once or in batches (e.g. as it retries)."*

## Nếu gộp chung thì sao?

Mỗi lần một part hết hạn sẽ phải hoặc:
- **init lại cả upload** → lãng phí, mất các part đã lên, tạo `uploadId` orphan, hoặc
- **vẫn cần một endpoint riêng để xin lại URL** → rốt cuộc vẫn phải có `presign-parts`.

→ Tách ra là đúng. Gộp chỉ hợp lý khi file **nhỏ / ít part** và không cần retry độc
lập; khi đó có thể làm dạng tùy chọn (thêm `presignAll bool` vào `initRequest` để
`init` trả luôn danh sách `parts`), vẫn giữ `presign-parts` cho file lớn.

## Tham chiếu
- `backend/internal/handler/handler.go:77-90` (presignRequest nhận mảng partNumbers)
- `backend/internal/handler/handler.go:99` (validate 1 ≤ partNumber ≤ 10000)
