# Leaky Bucket

Leaky Bucket xem request như nước đổ vào xô. Xô rò nước ra với tốc độ cố định. Nếu request đến nhanh hơn tốc độ rò và xô đầy, request mới bị reject.

Khác với token bucket, leaky bucket thiên về làm mượt output rate.

---

## Ý tưởng

```text
incoming requests -> bucket/queue -> process at fixed rate
```

Nếu queue còn chỗ, request được nhận. Nếu queue đầy, reject.

---

## Ưu điểm

* Làm mượt traffic.
* Bảo vệ worker/downstream service khỏi burst.
* Hợp với job processing hoặc endpoint cần tốc độ xử lý ổn định.
* Dễ giải thích bằng queue/backpressure.

---

## Nhược điểm

* Có thể tăng latency nếu request phải chờ trong queue.
* Nếu chỉ reject khi queue đầy, user có thể thấy response chậm trước khi bị reject.
* Không phù hợp nếu muốn cho burst lớn đi qua ngay.

---

## Dùng khi nào?

Phù hợp cho:

* bảo vệ worker xử lý report/export;
* gửi email/SMS/OTP theo tốc độ cố định;
* gọi third-party API có quota đều;
* upload/resize image pipeline;
* queue consumer cần kiểm soát throughput.

Không lý tưởng cho:

* API cần latency cực thấp;
* endpoint muốn cho burst ngắn đi qua ngay;
* login brute force cần tracking chính xác theo account.

---

## Leaky bucket vs token bucket

| Điểm so sánh | Token Bucket | Leaky Bucket |
|---|---|---|
| Burst | Cho burst nếu còn token | Làm mượt, queue đầy thì reject |
| Mục tiêu | Giữ average rate | Giữ output rate đều |
| Use case | API gateway, plan quota | Worker/downstream protection |
| Latency | Thường reject nhanh | Có thể queue/wait |

---

## Câu trả lời phỏng vấn

> Leaky bucket dùng bucket như một queue chảy ra với tốc độ cố định. Nó hợp khi tôi muốn làm mượt traffic và bảo vệ downstream, ví dụ gửi SMS, resize ảnh hoặc xử lý export. Nếu cần cho user burst ngắn nhưng vẫn giới hạn average rate, token bucket thường hợp hơn.
