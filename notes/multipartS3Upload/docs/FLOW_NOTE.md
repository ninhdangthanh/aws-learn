# Multipart S3 Upload — Flow Note

Flow gồm 4 API + 1 API huỷ. FE upload từng part trực tiếp lên S3 qua presigned URL,
backend chỉ điều phối (init / presign / complete / abort).

## Các bước

| Bước | API | Request | Response |
|------|-----|---------|----------|
| 1 | `POST /uploads/init` | `filename`, `size`, `contentType` | `key`, `uploadId`, `partSize` |
| 2 | `POST /uploads/presign-parts` | `key`, `uploadId`, `partNumbers[]` | `parts[]` (mỗi part: `partNumber` + `url`) |
| 3 | *(FE → S3 trực tiếp)* | PUT từng part lên presigned URL | header `ETag` |
| 4 | `POST /uploads/complete` | `key`, `uploadId`, `parts[]` (`partNumber` + `etag`) | `key`, `location` |
| — | `POST /uploads/abort` | `key`, `uploadId` | `status: aborted` |

### 1. Init
- Gửi thông tin file → backend tạo `key` (prefix + date + random + tên file) và
  gọi `CreateMultipartUpload` để lấy `uploadId`.
- Trả về `partSize` (cấu hình qua env `PART_SIZE_BYTES`) để FE dùng cắt file.

### 2. Presign parts
- FE tự tính số part: `numParts = ceil(fileSize / partSize)`.
- Gửi **danh sách `partNumbers`** = `[1, 2, ..., numParts]` (KHÔNG phải chỉ gửi số lượng).
- `partNumber` bắt đầu từ **1** (S3 yêu cầu 1–10000).
- Có thể xin tất cả 1 lần hoặc theo batch (khi retry).

### 3. Upload từng part (FE → S3)
- FE `PUT` từng chunk lên presigned URL tương ứng.
- S3 trả `ETag` ở **HTTP response header** (không phải body).

### 4. Complete
- Gửi `key`, `uploadId`, và list `parts` gồm `partNumber` + `etag`.
- Backend tự **sort parts theo partNumber** (S3 yêu cầu tăng dần) → FE không cần sắp thứ tự,
  nhưng phải gửi **đủ tất cả** các part.

### Abort
- Khi user huỷ, FE gọi `abort` với `key` + `uploadId` để S3 dọn các part dang dở (tránh tính phí).

## ⚠️ Các lỗi thường gặp

1. **CORS `ExposeHeaders: ["ETag"]`** trên bucket — lỗi phổ biến NHẤT.
   Không có thì `fetch`/`axios` đọc header `ETag` ra `null` ở bước 3.
2. **Không strip dấu ngoặc kép của ETag.** S3 trả `"abc123..."` (kèm dấu `"`),
   gửi nguyên si sang `complete`, đừng bỏ dấu ngoặc.
3. **partNumber từ 1**, không phải 0.
4. **S3 Lifecycle rule `AbortIncompleteMultipartUpload`** (vd 7 ngày) để tự dọn
   khi user đóng tab giữa chừng mà không gọi abort.

## Tham chiếu code
- Handler: `backend/internal/handler/handler.go` (4 endpoint)
- Config partSize: env `PART_SIZE_BYTES` (`backend/.env.example`)
