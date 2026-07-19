// Package tetris mô hình hoá game Tetris — block RƠI từ trên xuống.
//
// Ba khác biệt cốt lõi so với Block Puzzle (xem package blockpuzzle):
//
//  1. Có block đang hoạt động (ActiveBlock) di chuyển theo thời gian,
//     nên phải tách "hình dạng tĩnh" khỏi "trạng thái động".
//  2. Có GRAVITY: xoá hàng xong thì mọi hàng phía trên dồn xuống.
//  3. Chỉ xoá theo HÀNG, không xoá theo cột.
package tetris

import "strings"

// Point là toạ độ 2 chiều. Y hướng xuống, đúng quy ước màn hình.
type Point struct {
	X int
	Y int
}

// ShapeType là 7 loại khối chuẩn của Tetris (gọi là tetromino).
type ShapeType uint8

const (
	Empty ShapeType = iota
	I
	J
	L
	O
	S
	T
	Z
)

func (s ShapeType) String() string {
	return [...]string{".", "I", "J", "L", "O", "S", "T", "Z"}[s]
}

// Shape là dữ liệu TĨNH của một loại khối: hình dạng ở góc xoay 0.
//
// Điểm thiết kế quan trọng nhất của package này:
// Shape là immutable và dùng chung cho mọi khối cùng loại. Nó không biết
// mình đang ở đâu trên board. Vị trí và góc xoay nằm ở ActiveBlock.
//
// Vì sao tách? Nếu nhét Position vào Shape thì mỗi lần spawn phải copy
// toàn bộ hình dạng, và không thể chia sẻ định nghĩa khối giữa các ván.
// Tách ra thì Shape là hằng số, ActiveBlock nhẹ và copy thoải mái —
// rất tiện cho Ghost Piece và AI (cần clone trạng thái liên tục).
type Shape struct {
	Type  ShapeType
	Cells []Point // toạ độ tương đối tại rotation 0
}

// ActiveBlock là trạng thái ĐỘNG: khối nào, đang ở đâu, xoay bao nhiêu.
type ActiveBlock struct {
	Shape    Shape
	Position Point // origin trên board
	Rotation int   // 0..3, mỗi bậc là 90 độ theo chiều kim đồng hồ
}

// Cells trả về toạ độ TUYỆT ĐỐI của khối trên board.
//
// Đây là cầu nối giữa dữ liệu tĩnh và trạng thái động:
// hình dạng gốc -> xoay theo Rotation -> cộng Position.
func (a ActiveBlock) Cells() []Point {
	rotated := a.Shape.RotatedCells(a.Rotation)
	out := make([]Point, len(rotated))
	for i, p := range rotated {
		out[i] = Point{X: a.Position.X + p.X, Y: a.Position.Y + p.Y}
	}
	return out
}

// Board là bàn chơi. Cells[y][x] lưu loại khối đã đóng băng tại ô đó,
// Empty nghĩa là ô trống.
//
// Vì sao lưu ShapeType thay vì bool? Để render đúng màu từng khối và để
// debug dễ hơn — nhìn board là biết ô đó đến từ khối nào.
type Board struct {
	Width  int
	Height int
	Cells  [][]ShapeType
}

// Kích thước board Tetris chuẩn.
const (
	StandardWidth  = 10
	StandardHeight = 20
)

// NewBoard tạo board rỗng.
func NewBoard(width, height int) Board {
	cells := make([][]ShapeType, height)
	for y := range cells {
		cells[y] = make([]ShapeType, width)
	}
	return Board{Width: width, Height: height, Cells: cells}
}

// Occupied trả về true nếu ô nằm ngoài biên hoặc đã bị chiếm.
//
// Lưu ý về biên trên: ở bản này y < 0 cũng tính là "vướng", nghĩa là khối
// phải nằm trọn trong board. Tetris thật cho khối spawn ló lên trên biên
// (vùng vanish zone). Bỏ qua chi tiết đó giúp code gọn hơn khi live coding;
// nếu interviewer hỏi thì nói rõ đây là đơn giản hoá có chủ ý.
func (b Board) Occupied(x, y int) bool {
	if x < 0 || x >= b.Width || y < 0 || y >= b.Height {
		return true
	}
	return b.Cells[y][x] != Empty
}

// Clone trả về bản sao sâu. Cần cho Ghost Piece, AI và Undo.
func (b Board) Clone() Board {
	cells := make([][]ShapeType, b.Height)
	for y := range b.Cells {
		cells[y] = make([]ShapeType, b.Width)
		copy(cells[y], b.Cells[y])
	}
	return Board{Width: b.Width, Height: b.Height, Cells: cells}
}

// String render board dạng ASCII.
func (b Board) String() string {
	var sb strings.Builder
	for y := 0; y < b.Height; y++ {
		for x := 0; x < b.Width; x++ {
			sb.WriteString(b.Cells[y][x].String())
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

// Render vẽ board KÈM khối đang rơi và ghost của nó, dùng để hiển thị.
//
// Khối đang rơi cố tình KHÔNG được ghi vào Board.Cells cho tới lúc chạm
// đáy (xem Merge). Nếu ghi sớm, khối sẽ tự va chạm với chính nó ở bước
// di chuyển tiếp theo.
// Ghost được vẽ bằng ':' để phân biệt với khối thật.
func (b Board) Render(active ActiveBlock, showGhost bool) string {
	// Lớp overlay: vẽ đè lên board mà không đụng vào dữ liệu gốc.
	overlay := make(map[Point]string)

	if showGhost {
		for _, p := range GhostPosition(b, active).Cells() {
			overlay[p] = ":"
		}
	}
	// Khối thật vẽ sau để luôn đè lên ghost khi hai bên chồng nhau.
	for _, p := range active.Cells() {
		overlay[p] = active.Shape.Type.String()
	}

	var sb strings.Builder
	for y := 0; y < b.Height; y++ {
		for x := 0; x < b.Width; x++ {
			if ch, ok := overlay[Point{X: x, Y: y}]; ok {
				sb.WriteString(ch)
			} else {
				sb.WriteString(b.Cells[y][x].String())
			}
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

// ParseBoard dựng board từ ASCII, dùng cho test.
// '.' là ô trống, ký tự khác là ô đã chiếm (mặc định gán loại I).
func ParseBoard(s string) Board {
	lines := splitNonEmptyLines(s)
	if len(lines) == 0 {
		return NewBoard(0, 0)
	}

	board := NewBoard(len(lines[0]), len(lines))
	for y, line := range lines {
		for x, ch := range line {
			if ch != '.' {
				board.Cells[y][x] = I
			}
		}
	}
	return board
}

func splitNonEmptyLines(s string) []string {
	var out []string
	for _, line := range strings.Split(strings.TrimSpace(s), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}
