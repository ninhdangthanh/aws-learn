# Block Puzzle — bài toán tĩnh

Game kiểu Woodoku / Blockudoku. Người chơi được phát 3 block, kéo thả vào ô bất
kỳ trên board 9x9 hoặc 10x10. Hàng hoặc cột nào đầy thì bị xoá. **Block không
rơi, và không có gravity.**

Code: [`blockpuzzle/`](../blockpuzzle/)

---

## Đề bài gốc

> Cho board hiện tại (ô trống và ô đã chiếm) và một block mới.
> Viết hàm `CanPlace(board, block) bool` kiểm tra còn ít nhất một vị trí nào
> đặt được block hay không.

---

## Phần 1 — Mô hình hoá

### Point

```go
type Point struct {
    X int  // cột
    Y int  // hàng
}
```

Cùng một struct nhưng mang 2 nghĩa tuỳ ngữ cảnh:
- Trong `Block.Cells`: toạ độ **tương đối** so với origin của block
- Trong kết quả `FindPlacements`: toạ độ **tuyệt đối** trên board

Nói rõ điều này ra khi phỏng vấn — nó cho thấy bạn ý thức được sự mơ hồ và chấp
nhận nó một cách có chủ ý, thay vì vô tình.

### Board

```go
type Board struct {
    Width  int
    Height int
    Cells  [][]bool  // true = đã chiếm
}
```

**Quy ước toạ độ phải thống nhất tuyệt đối:** `Cells[y][x]` — hàng trước, cột
sau. Đây là nguồn bug số 1 trong mọi bài lưới. Cứ viết comment ngay dòng khai
báo và nói to lên khi code: "em dùng Cells hàng-cột nhé."

Thêm một helper nhỏ nhưng cực kỳ đáng giá:

```go
func (b Board) Occupied(x, y int) bool {
    if x < 0 || x >= b.Width || y < 0 || y >= b.Height {
        return true   // ngoài biên coi như đã chiếm
    }
    return b.Cells[y][x]
}
```

Gộp check biên vào đây khiến mọi hàm gọi sau này không phải lặp lại điều kiện
biên. Ít code lặp = ít chỗ để sai.

### Block — vì sao lưu bằng `[]Point`?

```go
type Block struct {
    Name  string
    Cells []Point   // toạ độ tương đối so với góc trên bên trái
}
```

Block chữ L:

```
X.
X.
XX
```

lưu thành `{0,0}, {0,1}, {0,2}, {1,2}`.

Block vuông:

```
XX
XX
```

lưu thành `{0,0}, {1,0}, {0,1}, {1,1}`.

**Câu trả lời khi interviewer hỏi "tại sao không dùng ma trận `[][]bool`?"**

> Em lưu block dưới dạng danh sách các ô tương đối so với góc trên bên trái.
> Cách này có mấy ưu điểm:
>
> 1. **Không lãng phí bộ nhớ** cho các ô trống trong bounding box. Block chữ L
>    có bounding box 2x3 = 6 ô nhưng chỉ 4 ô thật.
> 2. **Kiểm tra va chạm chỉ là cộng offset**, duyệt đúng số ô của block chứ
>    không phải cả bounding box.
> 3. **Xoay rất gọn** — chỉ cần biến đổi toạ độ từng điểm, không phải xoay ma trận.
> 4. **Số ô nhỏ** (thường 1–5) nên vòng lặp cực nhanh, coi như O(1).

Nếu interviewer phản biện "nhưng ma trận dễ hình dung hơn": đồng ý là dễ đọc
hơn khi debug, và đề xuất giữ cả hai — `[]Point` để tính toán, thêm hàm
`ParseBlock` dựng từ ASCII để viết test cho dễ đọc. Repo này làm đúng vậy.

---

## Phần 2 — Các case interviewer hay hỏi

### 1. `CanPlaceAt` — đặt được ở đúng vị trí này không?

```go
func CanPlaceAt(board Board, block Block, x, y int) bool {
    for _, p := range block.Cells {
        if board.Occupied(x+p.X, y+p.Y) {
            return false
        }
    }
    return true
}
```

Hàm nền tảng. Mọi thứ phía sau đều gọi lại nó.
**Độ phức tạp: O(BlockSize)**, thực tế là O(1).

### 2. `CanPlace` — còn chỗ nào đặt được không?

```go
func CanPlace(board Board, block Block) bool {
    for y := 0; y < board.Height; y++ {
        for x := 0; x < board.Width; x++ {
            if CanPlaceAt(board, block, x, y) {
                return true
            }
        }
    }
    return false
}
```

**Độ phức tạp: O(W × H × BlockSize).** Board 10x10, block ≤ 5 ô → ~500 phép so
sánh. Cực nhanh, không cần tối ưu.

> **Lưu ý về biên:** vòng lặp vẫn quét tới sát mép phải và mép dưới, không trừ
> đi kích thước block. Vì `Occupied` đã tự loại các origin làm block tràn ra
> ngoài rồi. Viết vậy tránh hẳn bug off-by-one — một lỗi rất hay gặp khi cố
> "tối ưu" bằng `x <= board.Width - blockWidth`.

### 3. `PlaceBlock` — đặt block xuống

```go
func PlaceBlock(board *Board, block Block, x, y int) bool {
    if !CanPlaceAt(*board, block, x, y) {
        return false
    }
    for _, p := range block.Cells {
        board.Cells[y+p.Y][x+p.X] = true
    }
    return true
}
```

**Vì sao trả `bool` chứ không phải void?** Hàm mutate state mà không validate là
nguồn bug kinh điển — caller quên gọi `CanPlaceAt` trước sẽ ghi đè ô đã chiếm
hoặc panic vì index out of range. Check ngay trong hàm giúp nó an toàn khi đứng
một mình. Đây là điểm cộng nếu bạn chủ động nói ra.

### 4. `IsRowFull` / `IsColFull`

```go
func IsRowFull(board Board, row int) bool {
    for x := 0; x < board.Width; x++ {
        if !board.Cells[row][x] {
            return false
        }
    }
    return true
}
```

**O(Width)** và **O(Height)** tương ứng.

### 5. `ClearRow` / `ClearCol` — xoá tại chỗ

```go
func ClearRow(board *Board, row int) {
    for x := 0; x < board.Width; x++ {
        board.Cells[row][x] = false
    }
}
```

**Khác Tetris ở đây:** không có gravity. Hàng bị xoá chỉ đơn giản trở thành hàng
trống tại chỗ, các ô phía trên đứng yên.

### 6. `ClearFullRowsAndCols` — ⚠️ CÁI BẪY QUAN TRỌNG NHẤT

Interviewer rất hay soi chỗ này.

```go
func ClearFullRowsAndCols(board *Board) ClearResult {
    var fullRows, fullCols []int

    // Giai đoạn 1: QUÉT trên board chưa bị thay đổi
    for y := 0; y < board.Height; y++ {
        if IsRowFull(*board, y) {
            fullRows = append(fullRows, y)
        }
    }
    for x := 0; x < board.Width; x++ {
        if IsColFull(*board, x) {
            fullCols = append(fullCols, x)
        }
    }

    // Giai đoạn 2: XOÁ
    for _, y := range fullRows { ClearRow(board, y) }
    for _, x := range fullCols { ClearCol(board, x) }

    return ClearResult{Rows: len(fullRows), Cols: len(fullCols)}
}
```

**Vì sao phải tách 2 giai đoạn?** Xét board này:

```
XXX     <- hàng 0 đầy
X..
X.X     ^ cột 0 cũng đầy
```

Nếu vừa quét vừa xoá: xoá hàng 0 trước → cột 0 mất ô trên cùng → không còn đầy
→ **cột 0 không bao giờ được xoá**. Sai luật game.

Nói cách khác: điều kiện "đầy" phải được đánh giá trên **cùng một snapshot** của
board. Đây là một dạng của lỗi kinh điển "sửa collection trong khi đang duyệt nó".

**O(W × H).**

### 7. `FindPlacements` — trả về tất cả vị trí đặt được

Biến thể của `CanPlace`: thay vì dừng ở kết quả đầu tiên thì gom hết lại. Dùng
cho tính năng gợi ý nước đi (hint) và cho AI.

```go
func FindPlacements(board Board, block Block) []Point {
    var result []Point
    for y := 0; y < board.Height; y++ {
        for x := 0; x < board.Width; x++ {
            if CanPlaceAt(board, block, x, y) {
                result = append(result, Point{X: x, Y: y})
            }
        }
    }
    return result
}
```

### 8. `IsGameOver`

```go
func IsGameOver(board Board, blocks []Block) bool {
    for _, b := range blocks {
        if CanPlace(board, b) {
            return false
        }
    }
    return true
}
```

Người chơi được phát 3 block và đặt theo thứ tự tuỳ ý, nên **chỉ cần một block
còn chỗ là chưa thua**.

**O(len(blocks) × W × H × BlockSize).**

### 9. Tính điểm

```go
func CalculateScore(block Block, cleared ClearResult) int {
    score := len(block.Cells) * PointsPerCell
    score += cleared.Total() * PointsPerLine
    if n := cleared.Total(); n > 1 {
        score += (n - 1) * ComboBonus   // thưởng combo
    }
    return score
}
```

Điểm đáng nói: **thưởng combo theo cấp số cộng** để xoá nhiều đường cùng lúc
luôn lợi hơn xoá lẻ tẻ. Đây chính là thứ tạo chiều sâu chiến thuật cho game —
nếu không có nó, người chơi cứ xoá được đường nào là xoá, không cần suy nghĩ.

### 10. `Rotate90` — xoay block

Game Block Puzzle thật thường KHÔNG cho xoay, nhưng interviewer rất hay hỏi
"nếu support rotate thì sao?" để xem bạn xử lý toạ độ thế nào.

```go
func Rotate90(block Block) Block {
    rotated := Block{Name: block.Name, Cells: make([]Point, len(block.Cells))}
    for i, p := range block.Cells {
        rotated.Cells[i] = Point{X: -p.Y, Y: p.X}
    }
    return Normalize(rotated)
}
```

**Cách suy ra công thức `(x,y) -> (-y,x)` ngay tại chỗ** (đừng học thuộc, sẽ
quên): trục Y hướng **xuống** theo quy ước màn hình. Lấy một điểm dễ hình dung —
điểm `(1,0)` nằm bên phải origin. Xoay theo chiều kim đồng hồ, nó phải nhảy
xuống **dưới** origin, tức thành `(0,1)`. Thay vào công thức: `(1,0) -> (-0,1)
= (0,1)`. Khớp.

Xoay ngược chiều kim đồng hồ thì là `(x,y) -> (y,-x)`.

### 11. `Normalize` — bắt buộc phải có sau khi xoay

```go
func Normalize(block Block) Block {
    // dịch toàn bộ toạ độ sao cho minX = 0 và minY = 0
    // rồi sort Cells để hai block cùng hình dạng có cùng biểu diễn
}
```

Sau khi xoay, toạ độ có thể **âm**. Normalize kéo block về góc trên bên trái để
origin luôn có nghĩa thống nhất — nếu không, `CanPlaceAt` sẽ tính sai vị trí.

Việc **sort** thêm vào là để hai block cùng hình dạng luôn có cùng biểu diễn,
phục vụ so sánh trong test và loại trùng ở `AllRotations`.

### 12. `AllRotations` — các biến thể thật sự khác nhau

Không phải block nào cũng có 4 biến thể:

| Block | Số biến thể |
|---|---|
| Ô đơn, ô vuông 2x2 | 1 |
| Thanh dài, chữ S, chữ Z | 2 |
| Chữ L, chữ J, chữ T | 4 |

Loại trùng quan trọng cho AI — đỡ phải simulate lại cùng một hình dạng.

### 13. `Clone` — ⚠️ nhớ copy sâu

```go
func (b Board) Clone() Board {
    cells := make([][]bool, b.Height)
    for y := range b.Cells {
        cells[y] = make([]bool, b.Width)
        copy(cells[y], b.Cells[y])   // phải copy TỪNG hàng
    }
    return Board{Width: b.Width, Height: b.Height, Cells: cells}
}
```

`[][]bool` là slice của slice. `copy(dst, src)` ở mức ngoài chỉ copy các **con
trỏ hàng** — hai board sẽ dùng chung mảng dữ liệu và sửa cái này làm hỏng cái kia.

Cần cho AI (thử nước đi mà không phá board gốc) và cho Undo.

### 14. Flood Fill — đếm vùng trống liên thông

Ít gặp hơn nhưng có thể bị hỏi. Hai ô trống thuộc cùng một vùng nếu đi được từ ô
này sang ô kia qua các ô trống theo **4 hướng** (không tính chéo).

```
XX...
XX...
XXXXX      -> 2 vùng
..XX.
..XX.
```

**Dùng BFS với queue thay vì DFS đệ quy** để tránh tràn stack trên board lớn.
Nói ra lý do này là điểm cộng.

**Ứng dụng:** board vỡ thành nhiều vùng nhỏ là dấu hiệu sắp thua, vì block to sẽ
không nhét vừa vùng nào. Có thể dùng làm heuristic cho AI.

**O(W × H)** — mỗi ô được thăm đúng một lần.

---

## Phần 3 — AI: tìm nước đi tốt nhất

Đây là câu hỏi khó nhất, thường chỉ hỏi khi còn dư thời gian.

```
bestScore = -1
for mỗi góc xoay:
    for mỗi vị trí đặt được:
        clone board
        đặt block
        xoá hàng + cột
        chấm điểm
return nước tốt nhất
```

**Vì sao brute force chấp nhận được?** Board 10x10 = 100 vị trí × tối đa 4 góc
xoay × O(W×H) để chấm điểm ≈ 40.000 phép tính. Với máy tính hiện đại là tức
thời. Không cần alpha-beta hay memo hoá.

Heuristic trong repo này gồm 3 thành phần:

1. **Điểm thực tế ăn được** — mục tiêu trực tiếp
2. **Số ô trống còn lại** — thưởng cho việc giữ board thoáng
3. **Phạt lỗ thủng** (ô trống bị vây kín 4 phía) — những ô này gần như không bao
   giờ lấp được nữa, trừ 3 điểm mỗi lỗ

> Khi trình bày phần này, nhấn mạnh: **heuristic là lựa chọn thiết kế, không có
> đáp án duy nhất đúng.** Điều quan trọng là giải thích được vì sao chọn từng
> thành phần và tại sao trọng số như vậy. Interviewer muốn nghe lập luận, không
> phải con số.

---

## Bảng tra nhanh độ phức tạp

| Hàm | Độ phức tạp | Ghi chú |
|---|---|---|
| `CanPlaceAt` | O(BlockSize) | thực tế O(1) |
| `CanPlace` | O(W × H × BlockSize) | ~500 phép với board 10x10 |
| `PlaceBlock` | O(BlockSize) | |
| `IsRowFull` | O(W) | |
| `IsColFull` | O(H) | |
| `ClearFullRowsAndCols` | O(W × H) | |
| `FindPlacements` | O(W × H × BlockSize) | |
| `IsGameOver` | O(k × W × H × BlockSize) | k = số block trên tay |
| `Rotate90` | O(BlockSize log BlockSize) | log là do sort trong Normalize |
| `Clone` | O(W × H) | |
| `CountEmptyRegions` | O(W × H) | flood fill |
| `FindBestMove` | O(4 × W × H × W × H) | brute force, vẫn nhanh |
