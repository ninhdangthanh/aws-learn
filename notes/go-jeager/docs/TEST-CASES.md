# Bảng test case

Toàn bộ case đã chạy trên lab này, kèm lệnh tái lập và kết quả **quan sát thật**
(không phải kỳ vọng suy đoán). Dùng làm checklist hồi quy sau khi sửa code.

Ký hiệu: ✅ đã chạy và đạt · ⬜ chưa test · 🐞 từng phát hiện lỗi

Chuẩn bị: `make up && make check`. Biến dùng chung: `G=http://localhost:8080`

---

## A. Kịch bản chính (6 script trong `scripts/`)

| ID | Case | Lệnh | Kết quả quan sát | |
|---|---|---|---|---|
| A1 | Hệ thống sẵn sàng | `make check` | 7 container up, seed 4 SKU: 50/100/30/2 | ✅ |
| A2 | Luồng thành công | `make happy` | HTTP 201 · **23 span** · **4 service** trong 1 trace | ✅ |
| A3 | Hết hàng thật (SKU-RARE qty 99) | `make oos` | HTTP 409 | ✅ |
| A4 | Hết hàng ép bằng baggage | `make oos` | HTTP 409 · tag `lab.out_of_stock_forced=true` | ✅ |
| A5 | DB chậm | `make slow` | 39ms → **3054ms** | ✅ |
| A6 | Service panic | `make panic` | HTTP 503 · trace cụt còn **8 span / 2 service** | ✅ |
| A7 | Panic tự hồi phục | `make panic` | inventory-svc sống lại sau **~2s**, request kế tiếp 201 | ✅ |
| A8 | Lỗi async | `make async` | **HTTP 201** nhưng callback không tăng | ✅ |
| A9 | Async đối chứng | `make async` | request thường → callback tăng | ✅ |
| A10 | Lưu lượng hỗn hợp | `make load` | 40 request, phân bố đúng ~70/15/10/5 | ✅ |
| A11 | Chạy trọn bộ | `make all` | cả 6 kịch bản pass liên tiếp | ✅ |

---

## B. Cấu trúc trace — kiểm chứng qua Jaeger API

Không tin màu sắc trên UI; truy vấn thẳng API để có số liệu cứng.

```bash
TID=<trace_id lấy từ header X-Trace-Id>
curl -sS "http://localhost:16686/api/traces/$TID" | jq -r '.data[0] as $t
  | "SPANS: \($t.spans|length)", "SERVICES: \([$t.processes[].serviceName]|sort|join(", "))"'
```

| ID | Case | Kết quả quan sát | |
|---|---|---|---|
| B1 | Trace thành công đủ 4 service | `gateway, inventory-svc, notification-svc, order-svc` · 23 span | ✅ |
| B2 | Nhánh async cùng trace với request gốc | `order.created process` → `HTTP POST` → `POST /internal/callback` | ✅ |
| B3 | traceparent xuyên qua AMQP | `lab.injected_traceparent` == `lab.received_traceparent` | ✅ |
| B4 | Baggage xuyên qua AMQP | span consumer có `lab.fail_mode = async_fail` | ✅ |
| B5 | Lỗi lan ngược qua 3 service | **4 span** `error=true` ở gateway/order-svc/inventory-svc | ✅ |
| B6 | 🐞 Span gốc **không** đỏ khi 409 | `POST /orders` `status=409` `error=false` — otelhttp chỉ đánh dấu ≥500 | ✅ |
| B7 | Thời gian dồn ở span lá | `pg_sleep` **3013ms** vs gateway **3037ms** → overhead mạng thật 24ms | ✅ |
| B8 | Panic làm mất span | chỉ `gateway, order-svc`; có span client Reserve, **không có** span server | ✅ |
| B9 | Mã lỗi ghi đúng trên span | `lab.grpc_code=Unavailable` `lab.http_status=503` | ✅ |
| B10 | Trace async: gốc xanh, con đỏ | root `error=false`; đỏ ở `render_email` + `order.created process` | ✅ |

Lệnh liệt kê span lỗi:
```bash
curl -sS "http://localhost:16686/api/traces/$TID" | jq -r '.data[0] as $t
  | $t.spans[] | select([.tags[]|select(.key=="error" and .value==true)]|length>0)
  | "\($t.processes[.processID].serviceName) -> \(.operationName)"'
```

---

## C. API gateway

| ID | Case | Lệnh | Kết quả | |
|---|---|---|---|---|
| C1 | Tạo đơn hợp lệ | `POST /orders {"sku":"SKU-MOUSE","qty":1}` | 201 + `X-Trace-Id` | ✅ |
| C2 | Đọc đơn theo id | `GET /orders/{id}` | 200, đủ 5 trường | ✅ |
| C3 | Đơn không tồn tại | `GET /orders/0000...0000` | 404 | ✅ |
| C4 | 🐞 **id sai định dạng uuid** | `GET /orders/khong-phai-uuid` | **400** *(xem D1)* | ✅ |
| C5 | sku rỗng | `{"sku":"","qty":1}` | 400 | ✅ |
| C6 | qty = 0 | `{"sku":"SKU-MOUSE","qty":0}` | 400 | ✅ |
| C7 | qty âm | `{"sku":"SKU-MOUSE","qty":-5}` | 400 | ✅ |
| C8 | Body không phải JSON | `khong-phai-json` | 400 | ✅ |
| C9 | SKU không tồn tại | `{"sku":"SKU-KHONG-CO","qty":1}` | 404 | ✅ |
| C10 | Health check | `GET /healthz` | 200 | ✅ |
| C11 | Đếm callback | `GET /stats` | `callbacks_received` tăng đúng | ✅ |

```bash
for b in '{"sku":"","qty":1}' '{"sku":"SKU-MOUSE","qty":0}' \
         '{"sku":"SKU-MOUSE","qty":-5}' 'khong-phai-json'; do
  printf '%-32s -> %s\n' "$b" "$(curl -sS -o /dev/null -w '%{http_code}' \
    -X POST $G/orders -H 'Content-Type: application/json' -d "$b")"
done
```

---

## D. Hồi quy — lỗi đã từng xảy ra

Hai case này **phải luôn chạy lại** sau khi sửa code liên quan.

### D1 🐞 uuid sai định dạng trả 500 và lộ nội bộ Postgres

Triệu chứng cũ:
```
HTTP 500
{"error":"lỗi nội bộ","detail":"... invalid input syntax for type uuid ... (SQLSTATE 22P02)"}
```

Vì sao nghiêm trọng ngoài chuyện mã lỗi: `otelhttp` đánh dấu span lỗi từ 500 trở
lên. Một input sai của client bị trả 500 sẽ làm bẩn đúng bộ lọc `error=true` mà
bạn dùng để săn sự cố thật.

Sửa: `uuid.Parse` trong `GetOrder` — `cmd/order/main.go`.

```bash
curl -sS -o /dev/null -w '%{http_code}\n' "$G/orders/khong-phai-uuid"   # phải là 400
```

### D2 🐞 Viền khung script lệch khi có dấu tiếng Việt

`printf '%-60s'` đếm theo **byte**, không theo bề rộng hiển thị. Ký tự UTF-8
nhiều byte làm viền phải thụt vào. Sửa: bỏ viền phải trong `header()` —
`scripts/_lib.sh`.

```bash
./scripts/00-check.sh | head -4    # ba dòng tiêu đề phải thẳng hàng
```

### D3 🐞 `resource.Merge` xung đột schema URL

Triệu chứng: cả 4 service crash-loop ngay khi khởi động.
```
ERROR khởi tạo OTel thất bại lỗi="tạo resource: conflicting Schema URL"
```
`resource.Default()` mang schema URL của semconv mà SDK dùng nội bộ, không trùng
phiên bản semconv ta import. Sửa: `resource.NewSchemaless(...)` —
`internal/otelx/otelx.go`.

```bash
docker compose ps    # cả 4 service phải Up, không Restarting
```

---

## E. Dữ liệu và tính đúng đắn

| ID | Case | Kết quả quan sát | |
|---|---|---|---|
| E1 | Đơn hỏng được đánh dấu `FAILED` | 16 CONFIRMED / 7 FAILED sau bộ test | ✅ |
| E2 | **20 request đồng thời cùng SKU** | cả 20 trả 201, tồn kho trừ **đúng 20** → không lost update | ✅ |
| E3 | Vét cạn tồn kho về 0 | đặt vừa đủ → 201, qty = 0 | ✅ |
| E4 | Đặt thêm khi đã hết | 409, qty **không âm** (ràng buộc `CHECK qty >= 0` không bị chạm) | ✅ |
| E5 | Message nack không tồn đọng | queue `lab.order.created` = 0 message | ✅ |
| E6 | Log không có lỗi ngoài dự kiến | mọi dòng ERROR đều thuộc fault mode của lab | ✅ |

Test đồng thời (E2) — đường code `SELECT ... FOR UPDATE`:
```bash
BEFORE=$(docker exec lab-postgres psql -U lab -d lab -tAc \
  "SELECT qty FROM inventory WHERE sku='SKU-PHONE';")
for i in $(seq 1 20); do
  curl -sS -o /dev/null -X POST $G/orders -H 'Content-Type: application/json' \
    -d '{"sku":"SKU-PHONE","qty":1}' &
done; wait
AFTER=$(docker exec lab-postgres psql -U lab -d lab -tAc \
  "SELECT qty FROM inventory WHERE sku='SKU-PHONE';")
echo "đã trừ $((BEFORE - AFTER)) — phải bằng 20"
```

Quét log (E6):
```bash
docker compose logs --since 30m | grep -a ERROR | grep -avE \
'giữ hàng thất bại|không đủ hàng|cố tình panic|dựng email thất bại|xử lý message thất bại|order id không phải UUID|không có SKU này|gọi service phía dưới thất bại|đọc đơn thất bại'
# rỗng = mọi lỗi đều là lỗi cố ý của lab
```

---

## F. Hạ tầng và tooling

| ID | Case | Lệnh | Kết quả | |
|---|---|---|---|---|
| F1 | **Khởi động sạch từ số 0** | `docker compose down -v && make up` | seed đúng, gateway sẵn sàng ~3s | ✅ |
| F2 | Sinh lại code gRPC | `make proto` | ổn định, build vẫn pass | ✅ |
| F3 | Dọn module | `make tidy` | không đổi go.mod/go.sum | ✅ |
| F4 | Build tại máy | `make build` | pass | ✅ |
| F5 | Vet | `make vet` | sạch | ✅ |
| F6 | Cú pháp bash | `for f in scripts/*.sh; do bash -n "$f"; done` | sạch | ✅ |
| F7 | Liệt kê lệnh | `make help` | đủ target | ✅ |
| F8 | gRPC gọi trực tiếp `GetStock` | `grpcurl -plaintext -d '{"sku":"SKU-MOUSE"}' localhost:9092 inventory.v1.InventoryService/GetStock` | trả đúng tồn kho | ✅ |
| F9 | Cổng không đụng stack khác | `make up` | Postgres 5442, RabbitMQ 5673/15673 | ✅ |

---

## G. Chưa test

| ID | Case | Lý do | |
|---|---|---|---|
| G1 | `make psql` / `rabbit` / `jaeger` / `logs` | tương tác hoặc mở trình duyệt | ⬜ |
| G2 | Unit test Go | **repo chưa có `go test` nào** — toàn bộ kiểm chứng là black-box qua script | ⬜ |
| G3 | `make load N=500` | sẽ vét cạn SKU-MOUSE/SKU-PHONE và trả 409 thật, lệch tỉ lệ thiết kế. Không phải bug — `make reset` là xong | ⬜ |
| G4 | Jaeger mất kết nối giữa chừng | chưa thử tắt Jaeger khi service đang chạy | ⬜ |
| G5 | Postgres/RabbitMQ chết giữa chừng | chỉ test retry lúc khởi động, chưa test đứt giữa luồng | ⬜ |

**G2 là khoảng trống đáng kể nhất.** Hai chỗ đáng viết unit test trước tiên:

- `internal/amqpx/carrier.go` — `Get` phải xử lý được cả `string` và `[]byte`
  (client AMQP khác nhau gửi khác kiểu). Sai chỗ này thì trace đứt im lặng.
- `internal/faultx` — `Inject`/`Mode` round-trip, và trường hợp giá trị chứa ký
  tự không hợp lệ với đặc tả baggage phải bỏ qua chứ không làm hỏng request.

---

## Chạy lại nhanh toàn bộ

```bash
make reset && make up          # từ trạng thái trắng
make check                     # A1
yes '' | ./scripts/run-all.sh  # A2–A11
```

Rồi chạy tay các khối C, D, E ở trên.
