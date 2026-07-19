# Câu hỏi phỏng vấn và cách trả lời

---

## Phần 1 — Clarify TRƯỚC khi code

Đây là 30 giây đáng giá nhất trong cả buổi. Interviewer cố tình mô tả đề hơi mơ
hồ để xem bạn có hỏi lại không. Ứng viên lao vào code ngay thường bị đánh giá
thấp hơn ứng viên hỏi một câu đúng chỗ.

### Câu hỏi vàng cho đề "kiểm tra block còn đặt được không"

> "Em muốn xác nhận một chút: ý bài là kiểm tra xem block có thể đặt ở **ít
> nhất một vị trí bất kỳ** trên board mà không đè lên ô đã chiếm và không vượt
> biên, đúng không ạ? Hay cần kiểm tra nó có vừa với **một vùng trống cụ thể**
> đã được chỉ định?"

### Các câu clarify khác nên hỏi

| Hỏi gì | Vì sao quan trọng |
|---|---|
| "Block có tự rơi theo thời gian không?" | Quyết định là Block Puzzle hay Tetris — thay đổi toàn bộ thiết kế |
| "Có cho xoay block không?" | Có xoay thì cần `Rotate` + `Normalize`, tăng đáng kể khối lượng code |
| "Xoá theo hàng, hay cả hàng và cột?" | Block Puzzle xoá cả hai, Tetris chỉ hàng |
| "Sau khi xoá, các ô còn lại có rơi xuống không?" | Đây chính là câu hỏi về gravity |
| "Board có luôn vuông không, hay NxM?" | Ảnh hưởng cách viết vòng lặp và tên biến |
| "Người chơi cầm 1 block hay nhiều block?" | Ảnh hưởng logic game over |

**Cách hỏi khéo:** đừng hỏi liên tiếp 6 câu như tra khảo. Hỏi 1–2 câu quan trọng
nhất, rồi vừa code vừa nói giả định của mình ra: *"Em giả sử board là NxM cho
tổng quát nhé, nếu chỉ cần vuông thì cũng chạy đúng."*

---

## Phần 2 — Câu hỏi thiết kế

### "Tại sao lưu block bằng `[]Point` mà không phải ma trận?"

> Em lưu block dưới dạng danh sách các ô tương đối so với góc trên bên trái.
> Mấy ưu điểm:
>
> 1. Không lãng phí bộ nhớ cho ô trống trong bounding box — block chữ L có
>    bounding box 6 ô nhưng chỉ 4 ô thật.
> 2. Kiểm tra va chạm chỉ là cộng offset, duyệt đúng số ô của block.
> 3. Xoay chỉ cần biến đổi toạ độ từng điểm, không phải xoay ma trận.
> 4. Số ô nhỏ (1–5) nên coi như O(1).
>
> Đánh đổi là khó hình dung hơn khi debug, nên em thêm hàm `ParseBlock` dựng
> block từ ASCII để viết test cho dễ đọc.

Câu cuối quan trọng: **nêu được nhược điểm và cách bù đắp** cho thấy bạn đánh
giá cân bằng chứ không phải học thuộc ưu điểm.

### "Tại sao tách `Shape` và `ActiveBlock`?" (Tetris)

Xem [02-tetris.md](02-tetris.md#nguyên-tắc-cốt-lõi-tách-dữ-liệu-tĩnh-khỏi-trạng-thái-động).
Ý chính: tách **immutable data** khỏi **runtime state**. Shape là hằng số dùng
chung, ActiveBlock nhẹ và copy thoải mái — quan trọng cho Ghost Piece và AI.

### "Tại sao không ghi khối đang rơi vào board luôn?"

> Vì lượt di chuyển kế tiếp, khối sẽ thấy chính các ô của mình là "đã bị chiếm"
> và tự chặn mình. Muốn ghi sớm thì phải xoá khối khỏi board trước mỗi lần kiểm
> tra rồi vẽ lại — phức tạp và dễ sai hơn hẳn.

### "Nếu không muốn sửa board gốc thì làm sao?"

`Clone()` với **copy sâu**. Nhấn mạnh: `[][]bool` là slice của slice, copy nông
chỉ copy con trỏ hàng — hai board sẽ dùng chung dữ liệu. Cần cho AI, Undo, và
Ghost Piece.

### "Làm sao test được logic này?"

Đây là câu hỏi rất hay gặp với vị trí Backend. Trả lời:

> Em dùng **table-driven test**, mỗi case là một board viết bằng ASCII cho dễ
> đọc:
>
> ```go
> {
>     name:  "gravity: hàng trên dồn xuống lấp chỗ trống",
>     board: "..X.\nXXXX\nX..X",
>     want:  1,
>     after: "....\n..X.\nX..X\n",
> }
> ```
>
> Board dạng chuỗi vừa là input vừa là tài liệu — nhìn là hiểu ngay case đang
> test cái gì. Em viết `ParseBoard` để dựng board từ chuỗi và `String()` để so
> sánh kết quả.

Repo này làm đúng vậy — xem [`blockpuzzle/clear_test.go`](../blockpuzzle/clear_test.go)
và [`tetris/lines_test.go`](../tetris/lines_test.go).

---

## Phần 3 — Những cái bẫy phải biết trước

### Bẫy 1: xoá hàng và cột cùng lúc (Block Puzzle)

Phải **quét xong** danh sách hàng đầy và cột đầy **rồi mới xoá**. Vừa quét vừa
xoá sẽ bỏ sót. Xem [01-block-puzzle.md](01-block-puzzle.md#6-clearfullrowsandcols--️-cái-bẫy-quan-trọng-nhất).

### Bẫy 2: gravity phải quét từ dưới lên (Tetris)

Các hàng dồn **xuống**, nên nguồn phải luôn ở phía trên đích. Quét từ trên xuống
sẽ ghi đè dữ liệu chưa kịp đọc. Xem [02-tetris.md](02-tetris.md#5-clearlines--gravity--️-phần-dễ-sai-nhất).

### Bẫy 3: copy nông khi Clone

`[][]bool` phải copy **từng hàng**.

### Bẫy 4: off-by-one ở biên

Đừng "tối ưu" vòng lặp thành `x <= board.Width - blockWidth`. Cứ quét hết board
và để hàm `Occupied` lo phần biên. Ít code hơn, ít chỗ sai hơn.

### Bẫy 5: quy ước `[y][x]` không nhất quán

Chọn một quy ước, viết comment ngay dòng khai báo, và **nói to lên** khi code.
Đây là nguồn bug số 1 trong mọi bài lưới.

### Bẫy 6: game over ≠ board đầy (Tetris)

Game over là **không spawn được khối mới**. Board còn nhiều ô trống ở dưới cũng
vô nghĩa nếu đỉnh đã bị chặn.

---

## Phần 4 — Cách trình bày độ phức tạp

Đừng nói "O(n)" chung chung. Nói rõ **n là gì** và **con số thực tế**:

❌ "Hàm này là O(n²)."

✅ "`CanPlace` là O(W × H × BlockSize). Với board 10x10 và block tối đa 5 ô thì
khoảng 500 phép so sánh — cực nhanh, không cần tối ưu gì thêm."

Câu thứ hai cho thấy bạn hiểu **khi nào KHÔNG cần tối ưu**, đó cũng là một kỹ
năng được đánh giá cao. Ứng viên tối ưu sớm những chỗ không cần thường bị trừ
điểm.

Bảng tra nhanh: [01-block-puzzle.md](01-block-puzzle.md#bảng-tra-nhanh-độ-phức-tạp) ·
[02-tetris.md](02-tetris.md#bảng-tra-nhanh-độ-phức-tạp)

---

## Phần 5 — Checklist tự chấm sau buổi phỏng vấn

- [ ] Có clarify đề bài trước khi code không?
- [ ] Có giải thích được lựa chọn cấu trúc dữ liệu không?
- [ ] Tên hàm và tên biến có rõ nghĩa không? (`CanPlaceAt` chứ không phải `check2`)
- [ ] Có tách hàm nhỏ, mỗi hàm một việc không?
- [ ] Có nói được độ phức tạp kèm con số thực tế không?
- [ ] Có chủ động nêu ra cái bẫy trước khi bị hỏi không?
- [ ] Có nói được cách test không?
- [ ] Khi bí, có nói ra suy nghĩ của mình thay vì im lặng không?

Điểm cuối quan trọng: **im lặng là thứ tệ nhất trong live coding.** Interviewer
không đọc được suy nghĩ của bạn. Nói ra kể cả khi chưa chắc: *"Em đang nghĩ chỗ
này có thể xoá nhầm cột, để em thử một ví dụ cụ thể..."* — như vậy họ vẫn thấy
được cách bạn suy luận, và thường sẽ gợi ý cho bạn.

---

## Phần 6 — Nếu chỉ có 15 phút để ôn

Đọc đúng 3 thứ:

1. **Bảng phân biệt 2 game** trong [00-tong-quan.md](00-tong-quan.md#phân-biệt-2-game--đọc-kỹ-phần-này-trước)
2. **`CanPlaceAt` + `CanPlace`** — gõ lại từ đầu không nhìn code
3. **`ClearLines` với gravity** — gõ lại từ đầu, đây là phần dễ sai nhất

Nếu còn thời gian: công thức xoay `(x,y) -> (-y,x)` và **cách suy ra nó** thay
vì học thuộc.
