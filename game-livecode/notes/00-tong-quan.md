# Tổng quan — Live coding game board

Đây là dạng bài live coding rất được ưa chuộng cho vị trí Backend Middle, vì nó
kiểm tra **tư duy mô hình hoá (modeling)** và **thuật toán** thay vì kiến thức
framework. Không ai quan tâm bạn có chơi game đó không — họ quan tâm cách bạn
biến một mô tả bằng lời thành struct và hàm.

---

## Phân biệt 2 game — ĐỌC KỸ PHẦN NÀY TRƯỚC

Đây là chỗ dễ nhầm nhất. Hai game nhìn giống nhau (đều là khối vuông trên lưới)
nhưng luật khác hẳn, kéo theo cách model khác hẳn.

| | **Block Puzzle** (Woodoku, Blockudoku) | **Tetris** |
|---|---|---|
| Block di chuyển | Không. Người chơi kéo thả vào ô bất kỳ | Rơi từ trên xuống theo thời gian |
| Có block "đang hoạt động"? | Không | Có — `ActiveBlock` |
| Xoá theo | **Hàng VÀ cột** | Chỉ **hàng** |
| Sau khi xoá | Ô còn lại **đứng yên** | **Gravity** — hàng trên dồn xuống |
| Game over khi | Không block nào trên tay đặt được | Không spawn được khối mới |
| Số block trong tay | Thường 3 | 1 (+ next queue, + hold) |
| Xoay block | Thường KHÔNG cho xoay | Có, kèm wall kick |
| Yếu tố thời gian | Không | Có — tick, level, tốc độ rơi |

**Câu một dòng để nhớ:** Block Puzzle là bài toán *tĩnh* (chỉ hỏi "đặt được ở
đâu?"), Tetris là bài toán *động* (có trạng thái thay đổi theo thời gian).

Nếu interviewer mô tả đề mà bạn chưa rõ là game nào, hỏi ngay:
> "Cho em hỏi block có tự rơi xuống theo thời gian không, hay người chơi đặt
> tuỳ ý vào ô bất kỳ ạ?"

Câu trả lời quyết định toàn bộ thiết kế phía sau.

---

## Bản đồ code trong repo này

```
blockpuzzle/                    tetris/
├── model.go     Point, Block   ├── model.go   Point, Shape, ActiveBlock
├── placement.go CanPlace       ├── shapes.go  7 tetromino + RotatedCells
├── clear.go     xoá hàng+cột   ├── move.go    CanMoveDown, HardDrop, Ghost
├── rotate.go    Rotate90       ├── rotate.go  Rotate + wall kick
├── game.go      Game, Score    ├── lines.go   ClearLines + GRAVITY
└── ai.go        FindBestMove   ├── merge.go   Merge, Spawn, GameOver
                                ├── bag.go     7-bag randomizer
                                ├── game.go    Game, Tick, Hold
                                └── ai.go      FindBestPlacement
```

Chạy thử:

```bash
go test ./...                  # 154 test
go run ./cmd/blockpuzzle-demo  # AI tự chơi Block Puzzle
go run ./cmd/tetris-demo       # AI tự chơi Tetris
```

---

## Lộ trình triển khai khi live coding

Interviewer hiếm khi bắt viết cả game trong 45–60 phút. Họ cho một bài nhỏ rồi
mở rộng dần. Cứ đi theo thứ tự này, mỗi bước đều chạy được và test được:

### Nếu là Block Puzzle

1. `Point`, `Block`, `Board` — dừng lại giải thích vì sao chọn `[]Point`
2. `CanPlaceAt(board, block, x, y)` — hàm nền tảng
3. `CanPlace(board, block)` — đề gốc, chỉ là 2 vòng lặp bọc ngoài bước 2
4. `PlaceBlock(board, block, x, y)`
5. `IsRowFull` / `IsColFull`
6. `ClearFullRowsAndCols` — **nhớ cái bẫy quét-xong-rồi-mới-xoá**
7. `IsGameOver(board, blocks)`
8. Còn thời gian: `Rotate90`, `FindPlacements`, tính điểm, AI

### Nếu là Tetris

1. `Point`, `Shape`, `ActiveBlock`, `Board` — **nhấn mạnh việc tách
   Shape (tĩnh) khỏi ActiveBlock (động)**, đây là điểm cộng lớn nhất
2. `Fits(board, block)` — hàm nền tảng
3. `CanMoveDown`, `MoveLeft`, `MoveRight`
4. `Merge(board, block)` — giải thích vì sao không ghi khối vào board sớm
5. `ClearLines` — **phần gravity là chỗ dễ sai nhất, làm cẩn thận**
6. `Spawn` + `IsGameOver`
7. `Tick` — ráp mọi thứ lại thành vòng game
8. Còn thời gian: `Rotate` + wall kick, `HardDrop`, `Ghost`, 7-bag, `Hold`

---

## 3 điều tạo khác biệt giữa ứng viên tốt và ứng viên trung bình

**1. Clarify trước khi code.** Xem [03-cau-hoi-phong-van.md](03-cau-hoi-phong-van.md).
Bỏ 30 giây hỏi lại đề tốt hơn code 15 phút sai hướng.

**2. Nói được độ phức tạp.** Không chỉ "O(n)" chung chung mà nói rõ n là gì:
"O(W × H × BlockSize), với board 10×10 và block tối đa 5 ô thì khoảng 500 phép
so sánh — hoàn toàn không cần tối ưu."

**3. Biết chỗ nào có bẫy và nói ra trước khi bị hỏi.**
- Block Puzzle: quét xong hàng+cột rồi mới xoá
- Tetris: gravity phải quét từ dưới lên; không ghi khối đang rơi vào board

---

## Đọc tiếp

- [01-block-puzzle.md](01-block-puzzle.md) — bài toán tĩnh, 14 case
- [02-tetris.md](02-tetris.md) — bài toán động, 15 case
- [03-cau-hoi-phong-van.md](03-cau-hoi-phong-van.md) — câu hỏi phụ và cách trả lời
