# Học Asynq cơ bản

Ví dụ này minh họa cách dùng [Asynq](https://github.com/hibiken/asynq) để đưa công việc nền vào Redis và xử lý bằng worker.

## Thành phần

- `cmd/producer`: chương trình tạo task và enqueue vào Redis.
- `cmd/worker`: chương trình chạy worker, lắng nghe queue và xử lý task.
- `task`: package định nghĩa task type, payload, hàm tạo task và handler.

## Luồng chạy

1. Producer tạo payload Go struct.
2. Payload được marshal sang JSON.
3. Producer gọi `client.Enqueue(...)` để đẩy task vào Redis.
4. Worker lấy task từ Redis theo queue priority.
5. Worker route task theo `Type...` và gọi handler tương ứng.
6. Handler unmarshal JSON, validate dữ liệu và xử lý nghiệp vụ.

## Chạy ví dụ

Chạy Redis:

```bash
docker run --rm -p 6379:6379 redis:7
```

Chạy worker ở terminal thứ nhất:

```bash
cd asynq
go run ./cmd/worker
```

Chạy producer ở terminal thứ hai:

```bash
cd asynq
go run ./cmd/producer
```

Nếu Redis không chạy ở `127.0.0.1:6379`, truyền biến môi trường:

```bash
REDIS_ADDR=localhost:6379 go run ./cmd/worker
REDIS_ADDR=localhost:6379 go run ./cmd/producer
```

## Các struct quan trọng

### `EmailDeliveryPayload`

Payload cho task gửi email.

- `UserID`: ID người nhận.
- `Email`: địa chỉ email người nhận.
- `TemplateID`: mã template email.
- `Subject`: tiêu đề email.
- `RequestedBy`: nguồn tạo task, hữu ích khi debug/audit.

### `ImageResizePayload`

Payload cho task resize ảnh.

- `SourceURL`: URL ảnh gốc.
- `Width`, `Height`: kích thước ảnh cần tạo.
- `Format`: định dạng đầu ra, ví dụ `jpg`, `jpeg`, `png`, `webp`.
- `OwnerID`: ID chủ sở hữu ảnh.

### `ImageProcessorConfig`

Cấu hình cho processor xử lý ảnh.

- `OutputDir`: thư mục dự kiến ghi ảnh sau khi resize.

## Ghi chú khi học

- `asynq.NewTask(type, payload, opts...)`: tạo task. Option đặt ở đây là mặc định của task.
- `client.Enqueue(task, opts...)`: đưa task vào Redis. Option ở đây có thể override option mặc định.
- `asynq.Queue(...)`: chọn queue. Ví dụ này có `critical`, `default`, `low`.
- `asynq.ProcessIn(...)`: hẹn giờ xử lý task trong tương lai.
- `asynq.Unique(...)`: chống enqueue trùng trong một khoảng thời gian.
- `asynq.MaxRetry(...)`: số lần retry tối đa khi handler trả lỗi.
- `asynq.Timeout(...)`: thời gian tối đa cho một lần xử lý task.
- `asynq.SkipRetry`: dùng khi lỗi không nên retry, ví dụ payload sai định dạng.

## Ý tưởng mở rộng

- Thay phần log gửi email bằng SMTP thật.
- Thay phần log resize ảnh bằng thư viện xử lý ảnh thật.
- Thêm `TaskID(...)` để chủ động đặt ID task.
- Thêm Asynqmon để quan sát queue qua giao diện web.
