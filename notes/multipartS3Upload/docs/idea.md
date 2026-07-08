# Upload file lớn lên S3 bằng pre-signed URL và Multipart Upload

## Bài toán

Khi client cần upload file lên S3, backend không nên nhận file rồi upload lại lên S3, đặc biệt với file lớn. Cách hợp lý hơn là backend chỉ cấp quyền upload tạm thời thông qua pre-signed URL, còn client upload trực tiếp lên S3.

Với file nhỏ, có thể dùng một pre-signed PUT URL duy nhất. Với file lớn, nên dùng S3 Multipart Upload kết hợp pre-signed URL cho từng part.

## Khi nào dùng cách nào?

- File nhỏ, ví dụ dưới khoảng 50-100 MB: dùng một pre-signed PUT URL thường là đủ.
- File lớn, ví dụ video, file zip, backup, hoặc file từ vài trăm MB đến vài GB: dùng Multipart Upload.

Không nên upload file lớn đi qua backend vì dễ gây timeout, tốn băng thông server, khó retry, và làm backend phải xử lý dữ liệu nhị phân không cần thiết.

## Ý tưởng chính

S3 Multipart Upload cho phép chia một file lớn thành nhiều phần nhỏ. Mỗi part được upload độc lập lên S3, có thể upload song song và retry riêng nếu lỗi. Sau khi tất cả part upload thành công, backend gọi API complete để S3 ghép các part thành object cuối cùng.

Một số giới hạn quan trọng:

- Tối đa 10,000 part cho một multipart upload.
- Mỗi part phải có kích thước tối thiểu 5 MiB, trừ part cuối cùng.
- Mỗi part có thể lớn tối đa 5 GiB.
- Sau khi upload thành công một part, S3 trả về `ETag`; client cần lưu lại `ETag` này để gửi khi complete upload.

## Flow đề xuất

1. Client gọi backend để khởi tạo upload, gửi các thông tin như `filename`, `size`, `contentType`.
2. Backend kiểm tra quyền, sinh `key`, rồi gọi `CreateMultipartUpload` lên S3.
3. Backend trả về cho client `uploadId`, `key`, và cấu hình như `partSize`.
4. Client chia file thành nhiều part theo `partSize`.
5. Client gọi backend để xin pre-signed URL cho từng part hoặc một batch nhiều part.
6. Backend tạo pre-signed URL cho từng `partNumber` và trả về cho client.
7. Client upload từng part trực tiếp lên S3 bằng các URL đã nhận.
8. S3 trả về `ETag` cho mỗi part upload thành công.
9. Client gửi `uploadId`, `key`, và danh sách `{ partNumber, ETag }` về backend.
10. Backend gọi `CompleteMultipartUpload` để hoàn tất upload.

## API backend nên có

```text
POST /uploads/init
POST /uploads/presign-parts
POST /uploads/complete
POST /uploads/abort
```

Vai trò của từng API:

- `POST /uploads/init`: khởi tạo multipart upload, tạo object key, trả về `uploadId`.
- `POST /uploads/presign-parts`: tạo pre-signed URL cho các part cần upload.
- `POST /uploads/complete`: nhận danh sách part đã upload và gọi S3 complete upload.
- `POST /uploads/abort`: hủy multipart upload khi client không muốn tiếp tục hoặc upload lỗi.

## Ví dụ

File cần upload: 1 GB

Part size: 10 MB

Số part: khoảng 100 part

Client có thể upload song song 3-5 part mỗi lần. Nếu một part upload lỗi, client chỉ cần retry part đó, không cần upload lại toàn bộ file.

## Lưu ý vận hành

Backend chỉ nên quản lý quyền, object key, `uploadId`, metadata, và trạng thái upload. File data nên đi trực tiếp từ client lên S3.

Nếu multipart upload bị bỏ dở, các part đã upload vẫn có thể phát sinh storage cost. Vì vậy cần có API `abort` và nên cấu hình S3 lifecycle rule để tự động cleanup các incomplete multipart uploads.
