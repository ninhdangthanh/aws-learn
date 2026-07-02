# Sliding Window Log

Sliding Window Log lưu timestamp của từng request trong một khoảng thời gian trượt. Khi request mới tới, limiter xóa các timestamp đã cũ, đếm số timestamp còn lại, rồi quyết định allow/reject.

Ví dụ limit `5 requests/10 seconds`:

```text
now = 12:00:10
valid window = 12:00:00 -> 12:00:10
```

Chỉ những request nằm trong 10 giây gần nhất được tính.

---

## Ưu điểm

* Chính xác hơn fixed window.
* Không bị burst mạnh ở ranh giới window.
* Dễ giải thích cho security endpoint.

---

## Nhược điểm

* Tốn memory vì phải lưu timestamp từng request.
* Với key có traffic lớn, danh sách timestamp có thể dài.
* Cần cleanup timestamp cũ.

---

## Dùng khi nào?

Phù hợp cho:

* brute force login;
* forgot password/OTP;
* register account;
* endpoint nhạy cảm cần limit chính xác;
* webhook receiver cần tránh spam từ một source.

Không lý tưởng cho:

* endpoint traffic cực lớn;
* API gateway global nếu không có storage phù hợp.

---

## Redis pattern

Có thể dùng sorted set:

```text
ZREMRANGEBYSCORE key 0 now-window
ZCARD key
ZADD key now request_id
EXPIRE key window
```

Nên gói trong Lua script để các bước atomic.

---

## Câu trả lời phỏng vấn

> Sliding window log lưu timestamp từng request và đếm request trong khoảng thời gian trượt. Nó chính xác, hợp với login hoặc OTP, nhưng tốn memory hơn vì phải lưu log request. Với Redis có thể dùng sorted set và Lua script.
