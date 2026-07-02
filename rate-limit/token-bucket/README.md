# Token Bucket

Token Bucket có một bucket chứa token. Token được refill đều theo thời gian. Mỗi request cần tiêu thụ một hoặc nhiều token. Nếu bucket còn token thì allow, hết token thì reject hoặc wait.

Ví dụ:

```text
refill rate = 10 tokens/second
capacity    = 50 tokens
```

User có thể burst tối đa 50 request nhanh, nhưng về lâu dài chỉ đi được trung bình 10 request/second.

---

## Ưu điểm

* Cho phép burst có kiểm soát.
* Giữ average rate ổn định.
* Rất phổ biến ở API gateway.
* Dễ áp dụng cho user/API key/tenant.

---

## Nhược điểm

* Cần chọn capacity hợp lý; quá lớn thì burst làm backend đau, quá nhỏ thì user thấy bị chặn cứng.
* Nếu implement phân tán, cần state shared và atomic.
* Không tự làm mượt request như leaky bucket, vì burst vẫn có thể đi qua.

---

## Dùng khi nào?

Phù hợp cho:

* API gateway;
* public API theo API key;
* SaaS free/premium tier;
* endpoint đọc nhiều có thể chịu burst;
* kiểm soát chi phí OpenAI/AWS theo user/tenant.

Ví dụ:

```text
free:    1 token/second, capacity 10
premium: 20 tokens/second, capacity 200
```

Với API tính tiền theo call, có thể trừ nhiều token cho request đắt:

```text
GET /products           -> 1 token
POST /ai/generate       -> 20 tokens
POST /reports/export    -> 50 tokens
```

---

## Câu trả lời phỏng vấn

> Token bucket refill token theo tốc độ cố định và request tiêu thụ token. Nó cho phép burst trong giới hạn capacity nhưng vẫn giữ average rate. Tôi thường dùng cho API gateway, tenant quota hoặc API tính tiền theo call như OpenAI/AWS.
