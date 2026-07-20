# Tetris — bài toán động

Block **rơi từ trên xuống** theo thời gian. Người chơi dịch trái/phải, xoay, thả
nhanh. Khối chạm đáy thì đóng băng vào board. Hàng nào đầy thì bị xoá và **mọi
hàng phía trên dồn xuống (gravity)**.

Code: [`tetris/`](../tetris/)

---

## Phần 1 — Mô hình hoá (phần quan trọng nhất)

### Nguyên tắc cốt lõi: tách dữ liệu TĨNH khỏi trạng thái ĐỘNG

Đây là thứ interviewer đánh giá cao nhất ở bài này.

**Cách làm ngây thơ** — nhét tất cả vào một struct:

```go
type Block struct {
    ID       int
    Cells    []Point
    Position Point     // ⚠️ trộn hình dạng với vị trí
}
```

**Cách làm tốt** — tách đôi:

```go
// TĨNH: hình dạng, không bao giờ thay đổi, dùng chung cho mọi khối cùng loại
type Shape struct {
    Type  ShapeType
    Cells []Point      // toạ độ tương đối tại rotation 0
}

// ĐỘNG: khối nào, đang ở đâu, xoay bao nhiêu
type ActiveBlock struct {
    Shape    Shape
    Position Point
    Rotation int       // 0..3
}
```

**Vì sao tách?** Đây là câu trả lời khi được hỏi:

> Em tách hình dạng khỏi trạng thái vì hai thứ này có vòng đời khác nhau.
> `Shape` là hằng số — khối chữ T lúc nào cũng là khối chữ T, định nghĩa một
> lần dùng cả ván. `ActiveBlock` thì thay đổi liên tục mỗi tick.
>
> Nếu gộp chung, mỗi lần spawn phải copy toàn bộ hình dạng, và không chia sẻ
> được định nghĩa khối giữa các ván. Tách ra thì `ActiveBlock` rất nhẹ, copy
> thoải mái — điều này quan trọng cho Ghost Piece và AI, vì cả hai đều cần
> clone trạng thái liên tục.
>
> Ngoài ra nó dễ test và dễ mở rộng hơn: muốn thêm loại khối mới chỉ cần thêm
> một `Shape`, không đụng gì tới logic di chuyển.

Đây là một ứng dụng cụ thể của nguyên tắc **tách immutable data khỏi runtime
state** — nói được tên nguyên tắc ra là điểm cộng.

### Cầu nối giữa hai thế giới

```go
// Cells trả về toạ độ TUYỆT ĐỐI trên board
func (a ActiveBlock) Cells() []Point {
    rotated := a.Shape.RotatedCells(a.Rotation)   // hình dạng gốc -> xoay
    out := make([]Point, len(rotated))
    for i, p := range rotated {
        out[i] = Point{X: a.Position.X + p.X, Y: a.Position.Y + p.Y}  // -> cộng vị trí
    }
    return out
}
```

Toàn bộ package chỉ cần đúng một chỗ chuyển đổi này.

### Board

```go
type Board struct {
    Width  int
    Height int
    Cells  [][]ShapeType   // Empty = ô trống
}
```

**Vì sao lưu `ShapeType` thay vì `bool`?** Để render đúng màu từng khối (game
thật mỗi loại một màu) và để debug dễ hơn — nhìn board là biết ô đó đến từ khối
nào. Chi phí gần như bằng 0 vì `ShapeType` là `uint8`.

### Game

```go
type Game struct {
    Board   Board
    Current ActiveBlock
    Hold    *Shape        // nil = chưa cất gì
    Score   int
    Lines   int
    Level   int
    Over    bool

    bag      *Bag
    holdUsed bool          // mỗi khối chỉ được Hold một lần
}
```

`Hold` là con trỏ vì nó thật sự có trạng thái "chưa có gì" — dùng `*Shape` diễn
đạt điều đó chính xác hơn là một zero value giả.

### Bảy khối chuẩn (tetromino)

```
I      J      L      O     S      T      Z
XXXX   X..    ..X    XX    .XX    XXX    XX.
       XXX    XXX    XX    XX.    .X.    .XX
```

---

## Phần 2 — Các case interviewer hay hỏi

Xếp theo mức độ phổ biến khi phỏng vấn Backend:

| Độ phổ biến | Bài toán |
|---|---|
| ⭐⭐⭐⭐⭐ | `CanMoveDown()` |
| ⭐⭐⭐⭐⭐ | `Rotate()` |
| ⭐⭐⭐⭐ | `ClearLines()` (kèm gravity) |
| ⭐⭐⭐⭐ | `Merge()` |
| ⭐⭐⭐⭐ | `Spawn()` / `IsGameOver()` |
| ⭐⭐⭐ | `HardDrop()` |
| ⭐⭐⭐ | `Tick()` |
| ⭐⭐ | `GhostPiece()` |
| ⭐⭐ | `Hold()` |
| ⭐ | Wall Kick |
| ⭐ | 7-Bag Randomizer |
| ⭐ | AI / Undo / Replay |

---

### 1. `Fits` — hàm nền tảng

```go
func Fits(board Board, block ActiveBlock) bool {
    for _, p := range block.Cells() {
        if board.Occupied(p.X, p.Y) {
            return false
        }
    }
    return true
}
```

**Mẫu code đáng nhớ:** mọi thao tác (di chuyển, xoay, spawn) đều quy về
**"tạo bản thử → hỏi `Fits` → chỉ commit khi hợp lệ"**. Mẫu này tránh được cả
một lớp bug: không bao giờ tồn tại trạng thái trung gian không hợp lệ.

### 2. `CanMoveDown` — câu hỏi được hỏi nhiều nhất

```go
func CanMove(board Board, block ActiveBlock, dx, dy int) bool {
    moved := block
    moved.Position = Point{X: block.Position.X + dx, Y: block.Position.Y + dy}
    return Fits(board, moved)
}

func CanMoveDown(board Board, block ActiveBlock) bool {
    return CanMove(board, block, 0, 1)
}
```

Trả `false` nghĩa là **đã tới lúc merge khối vào board**.

**O(BlockSize) = O(4).**

### 3. `Move` — trả về giá trị mới thay vì mutate

```go
func Move(board Board, block ActiveBlock, dx, dy int) (ActiveBlock, bool) {
    moved := block
    moved.Position = Point{X: block.Position.X + dx, Y: block.Position.Y + dy}
    if !Fits(board, moved) {
        return block, false      // trả về khối CŨ nguyên vẹn
    }
    return moved, true
}
```

`ActiveBlock` là struct nhỏ, copy rẻ. Trả về giá trị mới khiến "nước đi bị chặn"
không thể làm hỏng trạng thái cũ — an toàn hơn hẳn so với mutate qua con trỏ.

### 4. `Merge` — đóng băng khối vào board

```go
func Merge(board *Board, block ActiveBlock) {
    for _, p := range block.Cells() {
        board.Cells[p.Y][p.X] = block.Shape.Type
    }
}
```

**⚠️ Câu hỏi bẫy: "vì sao không ghi khối đang rơi vào board ngay từ đầu?"**

> Vì ở lượt di chuyển kế tiếp, khối sẽ phát hiện chính các ô của mình là "đã bị
> chiếm" và **tự chặn mình**. Muốn ghi sớm thì phải xoá khối khỏi board trước
> mỗi lần kiểm tra rồi vẽ lại — phức tạp và dễ sai hơn hẳn.
>
> Giữ `ActiveBlock` tách khỏi `Board` cho tới lúc lock là cách sạch hơn. Lúc
> render thì vẽ đè lên một lớp overlay, không đụng vào dữ liệu gốc.

### 5. `ClearLines` + GRAVITY — ⚠️ phần dễ sai nhất

Đây là khác biệt lớn nhất so với Block Puzzle:

```
.....        .....
XXXXX   ->   .....
.....        .....
AAAAA        AAAAA
```

Hàng `XXXXX` bị xoá, **mọi hàng phía trên tụt xuống 1 ô**.

**Cách cài đặt: con trỏ hai đầu, quét từ DƯỚI LÊN.**

```go
func ClearLines(board *Board) int {
    write := board.Height - 1

    for read := board.Height - 1; read >= 0; read-- {
        if IsRowFull(*board, read) {
            continue              // nuốt hàng này, write đứng yên
        }
        if write != read {
            copy(board.Cells[write], board.Cells[read])
        }
        write--
    }

    cleared := write + 1

    // Phần trên cùng còn sót dữ liệu cũ, phải xoá trắng
    for y := write; y >= 0; y-- {
        for x := 0; x < board.Width; x++ {
            board.Cells[y][x] = Empty
        }
    }
    return cleared
}
```

**Vì sao quét từ dưới lên chứ không phải trên xuống?** Vì các hàng dồn **xuống**.
Đi từ đáy thì nguồn (`read`) luôn ở phía trên đích (`write`), nên không bao giờ
ghi đè lên dữ liệu chưa kịp đọc. Quét từ trên xuống sẽ phá dữ liệu.

**Vì sao không dùng cách "cắt slice rồi chèn hàng mới lên đầu"?** Cách đó cũng
đúng và ngắn hơn:

```go
board.Cells = append(board.Cells[:y], board.Cells[y+1:]...)
board.Cells = append([][]ShapeType{newRow}, board.Cells...)
```

nhưng cấp phát bộ nhớ mới mỗi lần xoá. Cách hai con trỏ chạy **tại chỗ, một
lượt duy nhất**. Nói được cả hai cách và lý do chọn là điểm cộng.

**O(W × H)**, không phụ thuộc số hàng bị xoá.

### 6. `Spawn` và `IsGameOver`

```go
func SpawnPosition(board Board, shape Shape) Point {
    return Point{X: (board.Width - shape.Width(0)) / 2, Y: 0}   // căn giữa
}

func Spawn(board Board, shape Shape) (ActiveBlock, bool) {
    block := ActiveBlock{Shape: shape, Position: SpawnPosition(board, shape)}
    return block, Fits(board, block)
}
```

**Điểm đáng nói:** game over trong Tetris **KHÔNG phải là "board đầy"**, mà là
**"không spawn được khối mới"**. Board vẫn còn nhiều ô trống ở dưới cũng không
cứu được nếu đỉnh đã bị chặn:

```
XXXX     <- đỉnh bị chặn -> GAME OVER
....
....     <- dưới vẫn trống, vô nghĩa
....
```

### 7. `Tick` — nhịp trọng lực, ráp mọi thứ lại

Hàm được gọi mỗi khoảng thời gian nhất định (500ms, càng lên level càng nhanh).

```go
func (g *Game) Tick() int {
    if next, ok := MoveDown(g.Board, g.Current); ok {
        g.Current = next
        return 0                      // rơi được thì kết thúc lượt
    }
    return g.lockAndRefill()
}

func (g *Game) lockAndRefill() int {
    Merge(&g.Board, g.Current)        // 1. đóng băng
    cleared := ClearLines(&g.Board)   // 2. xoá hàng
    if cleared > 0 {                  // 3. cộng điểm
        g.Score += ScoreForLines(cleared, g.Level)
        g.Lines += cleared
        g.Level = g.Lines / 10
    }
    g.holdUsed = false
    g.spawnNext()                     // 4. spawn khối mới (thất bại -> game over)
    return cleared
}
```

**Thứ tự 4 bước này rất quan trọng.** Tách `lockAndRefill` ra riêng vì cả `Tick`
và `HardDrop` đều cần đúng chuỗi này — nếu để lặp code ở hai chỗ, chỉ cần sửa
luật tính điểm ở một chỗ là hai đường đi lệch nhau. Loại bug rất khó phát hiện.

**Mỗi tick không "rơi liên tục" mà là thử ứng viên rồi mới quyết định:**
`MoveDown` gọi `Move(board, block, 0, 1)` — hàm này tạo một **bản copy** của
block với `Position.Y + 1`, gọi `Fits` để kiểm tra bản copy đó hợp lệ hay
không, rồi **mới** gán vào `g.Current` nếu hợp lệ (xem mục 3, "trả về giá trị
mới thay vì mutate"). Nếu bản ứng viên không hợp lệ → không rơi được nữa →
lock + clear + spawn.

Nói cách khác: `Tick` không tính toán quỹ đạo rơi, nó chỉ hỏi "y+1 có được
không?" mỗi nhịp, và pattern "tạo ứng viên → kiểm tra Fits → commit hoặc bỏ"
lặp lại giống hệt ở `MoveLeft`/`MoveRight`/`RotateCW`/`HardDrop` — tất cả đều
quy về `Fits`.

**Lưu ý:** `Game.Tick()` hiện chưa được gọi bởi timer thời gian thực nào
trong code (chỉ `game_test.go` gọi trực tiếp để test logic, và
`cmd/tetris-demo` không dùng `Tick` — AI tính thẳng vị trí đích rồi
`HardDrop` luôn). Muốn có gravity thật theo `TickInterval()`, cần vòng lặp
ngoài kiểu `time.NewTicker` gọi `Tick()` định kỳ — chưa tồn tại.

### 8. `HardDrop` — thả thẳng xuống đáy

```go
func HardDrop(board Board, block ActiveBlock) (ActiveBlock, int) {
    distance := 0
    for {
        next, ok := MoveDown(board, block)
        if !ok {
            return block, distance
        }
        block = next
        distance++
    }
}
```

**O(H × BlockSize).** Board cao 20 nên hoàn toàn ổn, không cần tối ưu bằng cách
tính trước độ cao từng cột.

**Điểm thiết kế:** `HardDrop` chỉ **định vị**, không merge. Tách hai việc ra để
Ghost Piece tái sử dụng được chính hàm này.

### 9. `GhostPiece` — bóng mờ dưới đáy

```go
func GhostPosition(board Board, block ActiveBlock) ActiveBlock {
    ghost, _ := HardDrop(board, block)
    return ghost
}
```

Chỉ là `HardDrop` mà không merge. Đây là ví dụ đẹp cho việc **tách hàm đúng chỗ
thì tính năng mới gần như miễn phí**.

### 10. `Rotate` + Wall Kick

```go
var wallKickOffsets = []Point{
    {0, 0}, {-1, 0}, {1, 0}, {-2, 0}, {2, 0}, {0, -1},
}

func Rotate(board Board, block ActiveBlock, dir int) (ActiveBlock, bool) {
    if block.Shape.Type == O {
        return block, true      // khối vuông xoay kiểu gì cũng như cũ
    }
    for _, kick := range wallKickOffsets {
        candidate := block
        candidate.Rotation = ((block.Rotation+dir)%4 + 4) % 4
        candidate.Position = Point{block.Position.X + kick.X, block.Position.Y + kick.Y}
        if Fits(board, candidate) {
            return candidate, true
        }
    }
    return block, false
}
```

**Vì sao cần wall kick?** Khối I nằm dọc sát tường, xoay thành ngang sẽ tràn
biên:

```
|X          |XXXX     <- vướng tường, không xoay được
|X
|X    ->    dịch vào trong rồi mới xoay:
|X          |.XXXX    <- hợp lệ
```

Không có wall kick, người chơi sẽ thấy game "không cho xoay" một cách vô lý.

**Thứ tự offset có chủ ý:** thử tại chỗ trước → dịch ngang 1 ô → 2 ô (cần cho
khối I dài 4 ô) → cuối cùng mới đẩy lên. Đẩy lên xếp cuối vì nó thay đổi độ cao
của khối, cảm giác chơi sẽ lạ nếu ưu tiên nó hơn dịch ngang.

> Đây là bản đơn giản hoá. Tetris chuẩn dùng bảng **SRS (Super Rotation System)**
> với offset riêng cho từng cặp góc xoay và từng loại khối. Nếu interviewer hỏi,
> nói rõ: cách này giữ được tinh thần "xoay vướng thì thử dịch" mà không phải
> thuộc lòng bảng tra.

### 11. `Hold` — cất khối để dùng sau

```go
func (g *Game) HoldCurrent() bool {
    if g.Over || g.holdUsed {
        return false
    }
    // ... swap Current với Hold ...
    g.holdUsed = true
    return true
}
```

**Luật quan trọng: mỗi khối chỉ được Hold MỘT lần.** Không có luật này, người
chơi có thể swap qua lại vô hạn giữa hai khối để câu giờ — vừa phá cân bằng game
vừa khiến trọng lực mất tác dụng. Cờ `holdUsed` được reset khi khối bị lock.

Nói được lý do tồn tại của cờ này là điểm cộng — nó cho thấy bạn nghĩ về **luật
game**, không chỉ về code.

### 12. 7-Bag Randomizer

**Vì sao không dùng `rand.Intn(7)`?**

> Với random thuần, người chơi có thể xui liên tiếp. Xác suất không thấy khối I
> trong 12 lượt là (6/7)^12 ≈ **16%** — không hiếm chút nào. Người chơi sẽ thấy
> game "ăn gian" chứ không nghĩ là mình chơi dở.

**7-bag:** bỏ đủ 7 loại khối vào túi, xáo trộn, rút hết túi rồi mới đổ túi mới.

```go
func (b *Bag) appendShuffledBag() {
    bag := make([]Shape, len(AllShapes))
    copy(bag, AllShapes)
    b.rng.Shuffle(len(bag), func(i, j int) { bag[i], bag[j] = bag[j], bag[i] })
    b.current = append(b.current, bag...)
}
```

Hệ quả:
- Mỗi 7 lượt chắc chắn có đủ cả 7 loại, không bao giờ đói khối I
- Khoảng cách xa nhất giữa hai khối cùng loại là **13** (đầu túi này tới cuối
  túi sau), có giới hạn rõ ràng
- Vẫn đủ ngẫu nhiên để không đoán trước được thứ tự

> Đây là ví dụ rất hay khi phỏng vấn về việc **"ngẫu nhiên đúng luật phân phối"
> quan trọng hơn "ngẫu nhiên tuyệt đối"** trong thiết kế game. Nếu bạn kể được
> câu chuyện này, interviewer sẽ nhớ.

`rand.Shuffle` của Go dùng **Fisher-Yates**: duyệt từ cuối về đầu, mỗi bước đổi
chỗ phần tử hiện tại với một phần tử ngẫu nhiên trong phần chưa duyệt. O(n) và
phân phối đều.

### 13. Tính điểm

```go
var lineScores = map[int]int{0: 0, 1: 100, 2: 300, 3: 500, 4: 800}

func ScoreForLines(lines, level int) int {
    return lineScores[lines] * (level + 1)
}
```

Bảng điểm **cố tình phi tuyến**: xoá 4 hàng một lúc ("Tetris") được 800 điểm,
gấp đôi so với xoá 4 lần lẻ từng hàng (4 × 100 = 400). Đây là thứ khiến người
chơi giỏi chấp nhận rủi ro xếp cao chờ khối I, thay vì xoá lắt nhắt cho an toàn.

Nhân theo level để game càng về sau càng nhiều điểm, giữ động lực chơi tiếp.

### 14. Tốc độ theo level

```go
func (g *Game) TickInterval() int {
    interval := 500 - g.Level*50
    if interval < 100 {
        return 100          // chặn dưới
    }
    return interval
}
```

Chặn dưới quan trọng: không có nó, game sẽ nhanh tới mức không thể phản ứng
(hoặc tệ hơn, interval âm).

### 15. Undo / Replay

- **Undo:** clone board + `ActiveBlock` vào một stack trạng thái. Board 10x20 =
  200 byte mỗi snapshot, giữ 100 bước cũng chỉ 20KB — thoải mái.
- **Replay:** thay vì lưu trạng thái, lưu **danh sách thao tác** (`Left`,
  `Left`, `RotateCW`, `HardDrop`) cùng seed của randomizer. Phát lại bằng cách
  chạy lại từ đầu. Nhẹ hơn nhiều và là lý do vì sao `NewBag` nhận `*rand.Rand`
  chứ không dùng global rand.

Điểm cộng nếu bạn tự nói ra: **replay chỉ tái lập được nếu randomizer tất định
theo seed** — đó là một ràng buộc thiết kế phải quyết từ đầu, không thể thêm sau.

---

## Phần 3 — AI

```
for mỗi góc xoay (tối đa 4):
    for mỗi cột (tối đa 10):
        thả khối xuống -> clone board -> merge -> xoá hàng -> chấm điểm
```

Không gian tìm kiếm chỉ **4 × 10 = 40 nước**. Vét cạn thoải mái.

Heuristic dùng bộ trọng số nổi tiếng của **Pierre Dellacherie** — chơi rất tốt
dù chỉ nhìn 1 khối:

| Thành phần | Trọng số | Ý nghĩa |
|---|---|---|
| Số hàng xoá được | +0.76 | mục tiêu trực tiếp |
| Tổng chiều cao | −0.51 | xếp thấp thì còn chỗ xoay xở |
| Số lỗ chôn | −0.36 | mỗi lỗ khoá cứng một hàng |
| Độ gồ ghề | −0.18 | bề mặt phẳng nhận được nhiều loại khối hơn |

**Mở rộng "nhìn trước 2 khối":** nhân thêm 40 lần = 1.600 nước, vẫn chạy được.
Nhìn trước 3 khối thì 64.000 — bắt đầu cần cắt tỉa hoặc chỉ giữ top-N nước tốt
nhất ở mỗi tầng. Nói được ngưỡng này ra là điểm cộng.

**Ngoại lệ thú vị:** heuristic phạt độ gồ ghề, nhưng người chơi giỏi lại **cố ý**
chừa một khe sâu để chờ khối I ăn 4 hàng. Tức là heuristic "đúng" cho AI đơn
giản lại mâu thuẫn với chiến thuật tối ưu của con người. Đây là chỗ rất đáng để
thảo luận nếu interviewer quan tâm.

---

## Bảng tra nhanh độ phức tạp

| Hàm | Độ phức tạp | Ghi chú |
|---|---|---|
| `Fits` | O(BlockSize) = O(4) | |
| `CanMoveDown` / `Move` | O(4) | |
| `RotatedCells` | O(4 × BlockSize) | coi như hằng số |
| `Rotate` (có wall kick) | O(6 × BlockSize) | 6 offset |
| `Merge` | O(BlockSize) | |
| `IsRowFull` | O(W) | |
| `ClearLines` | O(W × H) | không phụ thuộc số hàng xoá |
| `HardDrop` / `GhostPosition` | O(H × BlockSize) | |
| `Spawn` / `IsGameOver` | O(BlockSize) | |
| `Bag.Next` | O(1) amortized | O(7) mỗi 7 lần đổ túi |
| `CountHoles` / `Height` | O(W × H) | |
| `FindBestPlacement` | O(4 × W × W × H) | ~8.000 phép, vét cạn thoải mái |
