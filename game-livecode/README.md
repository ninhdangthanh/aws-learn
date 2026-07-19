# game-livecode

Bộ ôn luyện live coding cho 2 dạng bài game board hay gặp khi phỏng vấn Backend:
**Block Puzzle** (Woodoku/Blockudoku) và **Tetris**.

Trọng tâm không phải là game, mà là **tư duy mô hình hoá và chia nhỏ bài toán** —
đó là thứ interviewer thật sự chấm.

## Bắt đầu từ đâu

Đọc [`notes/00-tong-quan.md`](notes/00-tong-quan.md) trước — nó phân biệt 2 game
và đưa lộ trình triển khai từng bước khi live coding.

| File | Nội dung |
|---|---|
| [notes/00-tong-quan.md](notes/00-tong-quan.md) | Phân biệt 2 game, lộ trình code, bản đồ repo |
| [notes/01-block-puzzle.md](notes/01-block-puzzle.md) | Bài toán tĩnh — 14 case, modeling, complexity |
| [notes/02-tetris.md](notes/02-tetris.md) | Bài toán động — 15 case, gravity, wall kick, 7-bag |
| [notes/03-cau-hoi-phong-van.md](notes/03-cau-hoi-phong-van.md) | Câu hỏi phụ, cách clarify, các bẫy, checklist |

## Chạy

```bash
go test ./...                  # 154 test, table-driven
go run ./cmd/blockpuzzle-demo  # AI tự chơi Block Puzzle, in board mỗi nước
go run ./cmd/tetris-demo       # AI tự chơi Tetris
go run ./cmd/tetris-demo -slow # chậm lại để xem cho rõ
```

## Cấu trúc

```
blockpuzzle/     Board tĩnh: đặt block vào ô bất kỳ, xoá hàng VÀ cột, không gravity
  model.go       Point, Block, Board, ParseBoard/ParseBlock
  placement.go   CanPlaceAt, CanPlace, PlaceBlock, FindPlacements
  clear.go       IsRowFull, IsColFull, ClearFullRowsAndCols
  rotate.go      Rotate90, Normalize, AllRotations
  game.go        Game, IsGameOver, CalculateScore
  ai.go          FindBestMove, CountHoles, flood fill

tetris/          Board động: block rơi theo thời gian, chỉ xoá hàng, CÓ gravity
  model.go       Point, Shape, ActiveBlock, Board
  shapes.go      7 tetromino, RotatedCells
  move.go        Fits, CanMoveDown, HardDrop, GhostPosition
  rotate.go      Rotate + wall kick
  lines.go       ClearLines (gravity), ScoreForLines, CountHoles
  merge.go       Merge, Spawn, IsGameOver
  bag.go         7-bag randomizer
  game.go        Game, Tick, Hold, TickInterval
  ai.go          FindBestPlacement (heuristic Dellacherie)

cmd/             Demo chạy được, in board ra terminal
notes/           Ghi chú đầy đủ bằng tiếng Việt
```

Hai package **cố tình độc lập hoàn toàn** — mỗi bên tự khai báo `Point` và
`Board`. Trùng lặp một chút nhưng đọc một package là hiểu trọn một bài, đúng
kiểu bạn sẽ gõ lại trong phòng phỏng vấn.

## Ghi chú về test

Test viết theo kiểu **table-driven**, board mô tả bằng ASCII cho dễ đọc:

```go
{
    name:  "gravity: hàng trên dồn xuống lấp chỗ trống",
    board: "..X.\nXXXX\nX..X",
    want:  1,
    after: "....\n..X.\nX..X\n",
}
```

Tên test đặt theo đúng tên bài toán để dùng làm flashcard ôn lại:

```bash
go test ./tetris/ -run TestClearLines -v
go test ./blockpuzzle/ -run TestCanPlace -v
```
