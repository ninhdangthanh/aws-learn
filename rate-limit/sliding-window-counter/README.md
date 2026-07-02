# Sliding Window Counter

Sliding Window Counter là phiên bản nhẹ hơn của sliding window log. Thay vì lưu từng timestamp request, nó lưu counter của current window và previous window, rồi tính weighted count theo mức độ trôi của thời gian.

Ví dụ:

```text
estimated_count = current_count + previous_count * overlap_ratio
```

Nếu đang ở giữa cửa sổ hiện tại, một phần request ở cửa sổ trước vẫn được tính vào giới hạn.

---

## Ưu điểm

* Mượt hơn fixed window.
* Ít memory hơn sliding window log.
* Hợp với API traffic lớn.
* Có thể implement bằng Redis counter.

---

## Nhược điểm

* Không chính xác tuyệt đối như sliding window log.
* Logic phức tạp hơn fixed window.
* Cần cẩn thận khi tính window boundary.

---

## Dùng khi nào?

Phù hợp cho:

* public API;
* tenant quota;
* API gateway;
* search/listing endpoint;
* fair usage giữa nhiều user.

Không lý tưởng khi:

* cần audit chính xác từng request;
* cần security decision rất nhạy như login của hệ thống tài chính.

---

## Câu trả lời phỏng vấn

> Sliding window counter là trade-off giữa fixed window và sliding window log. Nó dùng counter của current và previous window để ước lượng số request trong cửa sổ trượt. Độ chính xác không tuyệt đối, nhưng memory thấp và đủ tốt cho public API hoặc tenant quota.
