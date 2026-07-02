# Fixed Window Counter

Fixed Window Counter chia thời gian thành các cửa sổ cố định, ví dụ mỗi 1 phút. Mỗi key có một counter trong cửa sổ hiện tại. Nếu counter vượt limit thì reject.

Ví dụ: `100 requests/minute/user`.

```text
12:00:00 - 12:00:59 -> counter A
12:01:00 - 12:01:59 -> counter B
```

---

## Ưu điểm

* Rất dễ hiểu và dễ implement.
* Tốn ít memory: chỉ cần counter + window start.
* Hợp với quota đơn giản theo user/API key/tenant.
* Dễ làm bằng Redis `INCR` + `EXPIRE`.

---

## Nhược điểm

Có burst ở ranh giới cửa sổ.

Ví dụ limit `100 requests/minute`:

* user gửi 100 request ở `12:00:59`;
* rồi gửi 100 request ở `12:01:00`;
* trong 2 giây backend nhận 200 request nhưng vẫn không vi phạm từng window.

---

## Dùng khi nào?

Phù hợp cho:

* free tier quota đơn giản;
* API nội bộ ít nhạy cảm;
* giới hạn số lần gọi endpoint nhẹ;
* MVP/early stage cần limiter dễ triển khai.

Không lý tưởng cho:

* login brute force cần chính xác;
* endpoint đắt tiền;
* API public có traffic lớn và nhiều burst.

---

## Redis pattern

```text
key = rl:{endpoint}:{user_id}:{window_start}
INCR key
EXPIRE key window_size + buffer
```

Với production, nên dùng Lua script để `INCR` và `EXPIRE` atomic khi key mới tạo.

---

## Câu trả lời phỏng vấn

> Fixed window counter đếm request theo cửa sổ cố định, ví dụ mỗi phút. Nó đơn giản, rẻ và dễ làm bằng Redis, nhưng có nhược điểm là burst ở ranh giới window. Tôi dùng nó cho quota đơn giản hoặc free tier, còn login/security endpoint thì ưu tiên sliding window chính xác hơn.
