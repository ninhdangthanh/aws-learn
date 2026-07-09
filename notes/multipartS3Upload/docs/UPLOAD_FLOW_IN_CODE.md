# Flow Multipart Upload

Tài liệu này mô tả flow upload từ frontend, từ lúc user chọn file đến khi S3 hoàn tất multipart upload.

## Tổng Quan

Project này upload file theo kiểu multipart upload với presigned URL:

```text
Browser/React  ->  Go API: init, presign, complete, abort
Browser/React  ->  S3: PUT từng part trực tiếp bằng presigned URL
Go API         ->  S3: CreateMultipartUpload, PresignUploadPart, CompleteMultipartUpload, AbortMultipartUpload
```

Backend không nhận file bytes. Backend chỉ điều phối upload và tạo presigned URL. File bytes đi trực tiếp từ browser lên S3.

## File Chính

- `frontend/src/App.tsx`: UI chọn file, bấm upload, abort, hiển thị progress.
- `frontend/src/multipartUpload.ts`: logic điều phối chính của multipart upload ở frontend.
- `frontend/src/api.ts`: client gọi API backend bằng axios.
- `backend/internal/handler/handler.go`: HTTP handlers cho `/uploads/*`.
- `backend/internal/s3svc/s3svc.go`: wrapper gọi AWS S3 SDK.
- `backend/internal/config/config.go`: config part size, bucket, region, CORS origin, expiry.

## Flow Chi Tiết

### 1. User Chọn File

Code ở `frontend/src/App.tsx`.

Input file:

```tsx
<input
  type="file"
  disabled={uploading}
  onChange={(e) => {
    setFile(e.target.files?.[0] ?? null);
    setPhase("idle");
    setLoaded(0);
    setTotal(0);
    setMessage("");
  }}
/>
```

Khi user chọn file:

- `File` object được lưu vào state `file`.
- `phase` reset về `idle`.
- Progress và message được reset.
- Chưa có request upload nào được gửi.

### 2. User Bấm Upload

Code ở `frontend/src/App.tsx`, function `handleUpload`.

```tsx
const result = await multipartUpload(file, {
  concurrency: 4,
  maxRetries: 3,
  signal: controller.signal,
  onProgress: (l, t) => {
    setLoaded(l);
    setTotal(t);
  },
});
```

Tại đây FE bắt đầu upload:

- Tạo `AbortController` để có thể cancel upload.
- Set `phase = "uploading"`.
- Gọi `multipartUpload(file, options)`.
- `concurrency: 4` nghĩa là upload tối đa 4 parts song song.
- `maxRetries: 3` nghĩa là mỗi part retry tối đa 3 lần.

### 3. FE Gọi Backend Init Upload

Code ở `frontend/src/multipartUpload.ts`.

```ts
const { key, uploadId, partSize } = await api.init(
  file.name,
  file.size,
  file.type || "application/octet-stream",
);
```

API client ở `frontend/src/api.ts`. File này tạo axios instance dùng chung:

```ts
const http = axios.create({
  baseURL: API_BASE,
  headers: { "Content-Type": "application/json" },
});
```

Sau đó `api.init` gọi backend qua `http.post`:

```ts
init(filename: string, size: number, contentType: string) {
  return postJSON<InitResponse>("/uploads/init", { filename, size, contentType });
}
```

`postJSON` dùng axios và trả về `res.data`:

```ts
const res = await http.post<T>(path, body);
return res.data;
```

Backend handler ở `backend/internal/handler/handler.go`, function `Init`:

```go
key := h.buildKey(req.Filename)
uploadID, err := h.s3.CreateMultipartUpload(c.Request.Context(), key, contentType)
```

Backend làm những việc sau:

- Validate request body.
- Tạo `contentType` fallback là `application/octet-stream` nếu FE không gửi.
- Tạo object key bằng `buildKey`.
- Gọi S3 `CreateMultipartUpload`.
- Trả về:
  - `key`: object key trên S3.
  - `uploadId`: id của multipart upload session.
  - `partSize`: kích thước mỗi part mà FE sẽ dùng để cắt file.

Default `partSize` ở `backend/internal/config/config.go`:

```go
PartSize: getenvInt64("PART_SIZE_BYTES", 10*1024*1024)
```

Mặc định là 10 MiB.

### 4. FE Chia File Thành Parts

Code ở `frontend/src/multipartUpload.ts`.

```ts
const plans: PartPlan[] = [];
for (let start = 0, n = 1; start < file.size; start += partSize, n++) {
  plans.push({ partNumber: n, start, end: Math.min(start + partSize, file.size) });
}
```

FE tạo danh sách parts:

- `partNumber`: số thứ tự part, bắt đầu từ 1 theo yêu cầu của S3.
- `start`: byte bắt đầu.
- `end`: byte kết thúc.

Ví dụ file 25 MiB, `partSize = 10 MiB`:

```text
part 1: 0 MiB  -> 10 MiB
part 2: 10 MiB -> 20 MiB
part 3: 20 MiB -> 25 MiB
```

Nếu file rỗng, code vẫn tạo 1 part rỗng:

```ts
if (plans.length === 0) {
  plans.push({ partNumber: 1, start: 0, end: 0 });
}
```

### 5. FE Xin Presigned URL Cho Các Parts

Code ở `frontend/src/multipartUpload.ts`.

```ts
const { parts: presigned } = await api.presignParts(
  key,
  uploadId,
  plans.map((p) => p.partNumber),
);
```

API client ở `frontend/src/api.ts`, vẫn đi qua helper `postJSON` dùng axios:

```ts
presignParts(key: string, uploadId: string, partNumbers: number[]) {
  return postJSON<{ parts: PresignedPart[] }>("/uploads/presign-parts", {
    key,
    uploadId,
    partNumbers,
  });
}
```

Backend handler ở `backend/internal/handler/handler.go`, function `PresignParts`:

```go
for _, n := range req.PartNumbers {
  if n < 1 || n > 10000 {
    c.JSON(http.StatusBadRequest, gin.H{"error": "partNumber must be between 1 and 10000"})
    return
  }
  url, err := h.s3.PresignUploadPart(c.Request.Context(), req.Key, req.UploadID, n)
  ...
}
```

Backend:

- Validate `partNumber` từ 1 đến 10000.
- Gọi S3 presign cho từng part.
- Trả về danh sách `{ partNumber, url }`.

S3 service code ở `backend/internal/s3svc/s3svc.go`:

```go
req, err := s.presign.PresignUploadPart(ctx, &s3.UploadPartInput{
  Bucket:     aws.String(s.bucket),
  Key:        aws.String(key),
  UploadId:   aws.String(uploadID),
  PartNumber: aws.Int32(partNumber),
}, s3.WithPresignExpires(s.expiry))
```

### 6. FE Upload Parts Trực Tiếp Lên S3

Code ở `frontend/src/multipartUpload.ts`.

Worker pool:

```ts
await Promise.all(Array.from({ length: Math.min(concurrency, plans.length) }, worker));
```

Mỗi worker lấy part tiếp theo:

```ts
const plan = plans[idx];
const blob = file.slice(plan.start, plan.end);
const etag = await putPartWithRetry(
  (forceRefresh) => resolveUrl(plan.partNumber, forceRefresh),
  blob,
  maxRetries,
  opts.signal,
  (loaded) => {
    loadedPerPart[idx] = loaded;
    emitProgress();
  },
);
```

FE cắt file bằng `file.slice(start, end)` để lấy `Blob` của part.

Upload thật sự nằm trong function `putPart`:

```ts
const xhr = new XMLHttpRequest();
xhr.open("PUT", url, true);
xhr.send(blob);
```

`url` là presigned URL của S3. Request này đi trực tiếp:

```text
Browser -> S3
```

Không đi qua backend.

### 7. FE Đọc ETag Sau Mỗi Part

Code ở `frontend/src/multipartUpload.ts`.

```ts
const etag = xhr.getResponseHeader("ETag");
if (!etag) {
  reject(new Error("missing ETag header - check the bucket CORS ExposeHeaders"));
  return;
}
resolve(etag);
```

S3 trả về `ETag` cho mỗi uploaded part. FE cần giữ lại `ETag` để complete upload sau này.

Bucket CORS phải expose `ETag`, nếu không browser upload có thể thành công nhưng JS không đọc được header này.

S3 bucket CORS cần có:

```json
[
  {
    "AllowedOrigins": ["http://localhost:5173"],
    "AllowedMethods": ["PUT"],
    "AllowedHeaders": ["*"],
    "ExposeHeaders": ["ETag"],
    "MaxAgeSeconds": 3000
  }
]
```

### 8. Progress Upload

Code ở `frontend/src/multipartUpload.ts`.

```ts
xhr.upload.onprogress = (e) => {
  if (e.lengthComputable) onProgress(e.loaded);
};
```

Mỗi part có progress riêng trong array:

```ts
const loadedPerPart = new Array<number>(plans.length).fill(0);
```

Mỗi lần progress thay đổi, FE cộng tất cả part:

```ts
opts.onProgress(
  loadedPerPart.reduce((a, b) => a + b, 0),
  total,
);
```

`App.tsx` nhận progress và update UI:

```tsx
onProgress: (l, t) => {
  setLoaded(l);
  setTotal(t);
}
```

### 9. Retry Nếu Upload Part Fail

Code ở `frontend/src/multipartUpload.ts`, function `putPartWithRetry`.

```ts
let attempt = 0;
while (true) {
  try {
    const url = await getUrl(attempt > 0);
    return await putPart(url, blob, signal, onProgress);
  } catch (err) {
    if (signal?.aborted) throw err;
    attempt++;
    if (attempt > maxRetries) throw err;
    onProgress(0);
    await sleep(300 * 2 ** (attempt - 1));
  }
}
```

Nếu part fail:

- Chỉ retry part đó.
- Không upload lại cả file.
- Reset progress của part đó về 0.
- Đợi exponential backoff: 300ms, 600ms, 1200ms...
- Khi retry thì xin presigned URL mới.

Presigned URL mới được xin ở:

```ts
const resolveUrl = async (partNumber: number, forceRefresh: boolean): Promise<string> => {
  const cached = urlByPart.get(partNumber);
  if (cached && !forceRefresh) return cached;
  const { parts } = await api.presignParts(key, uploadId, [partNumber]);
  ...
};
```

Lý do: presigned URL có expiry. Default expiry ở backend:

```go
PresignExpiry: time.Duration(getenvInt64("PRESIGN_EXPIRY_SECONDS", 900)) * time.Second
```

Mặc định là 15 phút.

### 10. Complete Multipart Upload

Sau khi tất cả parts upload thành công:

```ts
completed[idx] = { partNumber: plan.partNumber, etag };
```

FE gọi complete:

```ts
const result = await api.complete(key, uploadId, completed);
```

API client ở `frontend/src/api.ts`, vẫn đi qua helper `postJSON` dùng axios:

```ts
complete(key: string, uploadId: string, parts: CompletedPart[]) {
  return postJSON<{ key: string; location: string }>("/uploads/complete", {
    key,
    uploadId,
    parts,
  });
}
```

Backend handler ở `backend/internal/handler/handler.go`, function `Complete`:

```go
parts := make([]s3svc.CompletedPart, len(req.Parts))
for i, p := range req.Parts {
  parts[i] = s3svc.CompletedPart{PartNumber: p.PartNumber, ETag: p.ETag}
}
sort.Slice(parts, func(i, j int) bool { return parts[i].PartNumber < parts[j].PartNumber })

location, err := h.s3.CompleteMultipartUpload(c.Request.Context(), req.Key, req.UploadID, parts)
```

Backend:

- Nhận `{ partNumber, etag }` từ FE.
- Sort parts theo `partNumber`, vì S3 yêu cầu thứ tự tăng dần.
- Gọi S3 `CompleteMultipartUpload`.
- Trả về `{ key, location }`.

S3 service code ở `backend/internal/s3svc/s3svc.go`:

```go
out, err := s.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
  Bucket:          aws.String(s.bucket),
  Key:             aws.String(key),
  UploadId:        aws.String(uploadID),
  MultipartUpload: &types.CompletedMultipartUpload{Parts: completed},
})
```

### 11. Upload Done

Sau khi `multipartUpload` return thành công, `App.tsx` set UI:

```tsx
setPhase("done");
setMessage(`Uploaded to: ${result.key}`);
```

### 12. Abort Upload

User bấm nút Abort trong `App.tsx`:

```tsx
function handleAbort() {
  abortRef.current?.abort();
}
```

`AbortController` sẽ abort các XHR đang upload:

```ts
signal.addEventListener("abort", () => xhr.abort(), { once: true });
```

Trong `multipartUpload`, nếu có lỗi hoặc abort:

```ts
} catch (err) {
  await api.abort(key, uploadId).catch(() => {});
  throw err;
}
```

FE gọi backend `/uploads/abort`.

Backend handler ở `backend/internal/handler/handler.go`, function `Abort`:

```go
if err := h.s3.AbortMultipartUpload(c.Request.Context(), req.Key, req.UploadID); err != nil {
  c.JSON(http.StatusBadGateway, gin.H{"error": "abort failed: " + err.Error()})
  return
}
```

S3 service code ở `backend/internal/s3svc/s3svc.go`:

```go
_, err := s.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
  Bucket:   aws.String(s.bucket),
  Key:      aws.String(key),
  UploadId: aws.String(uploadID),
})
```

Abort giúp S3 xoá các uploaded parts chưa complete, tránh tốn storage cost.

## API Summary

### `POST /uploads/init`

FE gửi:

```json
{
  "filename": "video.mp4",
  "size": 26214400,
  "contentType": "video/mp4"
}
```

Backend trả:

```json
{
  "key": "uploads/2026/07/08/random-video.mp4",
  "uploadId": "...",
  "partSize": 10485760
}
```

### `POST /uploads/presign-parts`

FE gửi:

```json
{
  "key": "uploads/2026/07/08/random-video.mp4",
  "uploadId": "...",
  "partNumbers": [1, 2, 3]
}
```

Backend trả:

```json
{
  "parts": [
    { "partNumber": 1, "url": "https://s3..." },
    { "partNumber": 2, "url": "https://s3..." },
    { "partNumber": 3, "url": "https://s3..." }
  ]
}
```

### Direct PUT To S3

FE gửi trực tiếp:

```http
PUT https://s3-presigned-url-for-part

<binary bytes của part>
```

S3 trả:

```http
200 OK
ETag: "..."
```

### `POST /uploads/complete`

FE gửi:

```json
{
  "key": "uploads/2026/07/08/random-video.mp4",
  "uploadId": "...",
  "parts": [
    { "partNumber": 1, "etag": "\"etag-1\"" },
    { "partNumber": 2, "etag": "\"etag-2\"" },
    { "partNumber": 3, "etag": "\"etag-3\"" }
  ]
}
```

Backend trả:

```json
{
  "key": "uploads/2026/07/08/random-video.mp4",
  "location": "https://bucket.s3.region.amazonaws.com/uploads/..."
}
```

### `POST /uploads/abort`

FE gửi:

```json
{
  "key": "uploads/2026/07/08/random-video.mp4",
  "uploadId": "..."
}
```

Backend trả:

```json
{
  "status": "aborted"
}
```

## Flow Rút Gọn

```text
User chọn file
  -> App.tsx lưu File vào state
  -> User bấm Upload
  -> multipartUpload(file)
  -> POST /uploads/init
  -> Backend gọi S3 CreateMultipartUpload
  -> FE chia file thành parts
  -> POST /uploads/presign-parts
  -> Backend tạo presigned UploadPart URL
  -> Browser PUT từng part trực tiếp lên S3, concurrency = 4
  -> FE lấy ETag từng part
  -> POST /uploads/complete với danh sách { partNumber, etag }
  -> Backend gọi S3 CompleteMultipartUpload
  -> Upload hoàn tất
```

## Điểm Cần Nhớ

- Backend không nhận file bytes.
- File bytes đi trực tiếp từ browser lên S3.
- FE dùng axios để gọi các API backend `/uploads/*`.
- Backend cần AWS credentials để gọi S3 control-plane API.
- Browser cần S3 bucket CORS cho `PUT` và expose `ETag`.
- Mỗi part fail thì retry riêng part đó.
- Khi retry, FE xin presigned URL mới cho part đó.
- Nếu abort hoặc fatal error, FE gọi `/uploads/abort` để cleanup multipart upload trên S3.
- Part number của S3 bắt đầu từ 1 và tối đa 10000.
- Default part size là 10 MiB.
- Default presigned URL expiry là 15 phút.
