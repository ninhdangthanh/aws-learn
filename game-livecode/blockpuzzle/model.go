// Package blockpuzzle mô hình hoá game Block Puzzle (Woodoku, Blockudoku).
//
// Đặc điểm của thể loại này: block KHÔNG rơi. Người chơi kéo thả block vào
// một ô bất kỳ trên board. Hàng hoặc cột nào đầy thì bị xoá, các ô còn lại
// đứng yên (không có gravity — đây là khác biệt lớn nhất so với Tetris).
package blockpuzzle

import "strings"

// Point là toạ độ 2 chiều.
//
// Tuỳ ngữ cảnh mà Point mang nghĩa khác nhau:
//   - Trong Block.Cells: toạ độ TƯƠNG ĐỐI so với origin của block.
//   - Trong kết quả FindPlacements: toạ độ TUYỆT ĐỐI trên board.
type Point struct {
	X int // cột
	Y int // hàng
}

// Block là một khối, lưu dưới dạng danh sách các ô tương đối so với góc trên
// bên trái (origin) của bounding box.
//
// Vì sao lưu bằng []Point thay vì ma trận [][]bool?
//   - Không tốn bộ nhớ cho các ô trống trong bounding box.
//   - Kiểm tra va chạm chỉ cần cộng offset, không phải duyệt cả ma trận.
//   - Xoay block = biến đổi toạ độ từng điểm, rất gọn (xem rotate.go).
//   - Số ô của block nhỏ (thường 1..5) nên duyệt cực nhanh.
type Block struct {
	Name  string
	Cells []Point
}

// Board là bàn chơi. Cells[y][x] == true nghĩa là ô đó đã bị chiếm.
//
// Quy ước toạ độ dùng thống nhất toàn package: hàng trước, cột sau —
// Cells[y][x], với y là hàng (0 ở trên cùng) và x là cột (0 ở bên trái).
type Board struct {
	Width  int
	Height int
	Cells  [][]bool
}

// NewBoard tạo board rỗng kích thước width x height.
func NewBoard(width, height int) Board {
	cells := make([][]bool, height)
	for y := range cells {
		cells[y] = make([]bool, width)
	}
	return Board{Width: width, Height: height, Cells: cells}
}

// Occupied trả về true nếu ô (x, y) nằm ngoài biên hoặc đã bị chiếm.
//
// Gộp check biên vào đây giúp các hàm gọi không phải lặp lại điều kiện biên.
func (b Board) Occupied(x, y int) bool {
	if x < 0 || x >= b.Width || y < 0 || y >= b.Height {
		return true
	}
	return b.Cells[y][x]
}

// Clone trả về bản sao sâu của board.
//
// Cần cho AI (thử đặt rồi tính điểm mà không phá board gốc) và cho Undo.
// Lưu ý: copy từng hàng, vì [][]bool là slice của slice — copy nông sẽ
// khiến hai board dùng chung mảng dữ liệu.
func (b Board) Clone() Board {
	cells := make([][]bool, b.Height)
	for y := range b.Cells {
		cells[y] = make([]bool, b.Width)
		copy(cells[y], b.Cells[y])
	}
	return Board{Width: b.Width, Height: b.Height, Cells: cells}
}

// String render board dạng ASCII: '.' là ô trống, 'X' là ô đã chiếm.
func (b Board) String() string {
	var sb strings.Builder
	for y := 0; y < b.Height; y++ {
		for x := 0; x < b.Width; x++ {
			if b.Cells[y][x] {
				sb.WriteByte('X')
			} else {
				sb.WriteByte('.')
			}
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

// ParseBoard dựng board từ chuỗi ASCII, tiện để viết test cho dễ đọc.
//
//	ParseBoard(`
//	.....
//	..X..
//	.....`)
//
// Ký tự '.' là ô trống, mọi ký tự khác là ô đã chiếm.
func ParseBoard(s string) Board {
	lines := splitNonEmptyLines(s)
	if len(lines) == 0 {
		return NewBoard(0, 0)
	}

	board := NewBoard(len(lines[0]), len(lines))
	for y, line := range lines {
		for x, ch := range line {
			board.Cells[y][x] = ch != '.'
		}
	}
	return board
}

// ParseBlock dựng block từ chuỗi ASCII, đã Normalize sẵn.
//
//	ParseBlock("L", `
//	X.
//	X.
//	XX`)
func ParseBlock(name, s string) Block {
	var cells []Point
	for y, line := range splitNonEmptyLines(s) {
		for x, ch := range line {
			if ch != '.' {
				cells = append(cells, Point{X: x, Y: y})
			}
		}
	}
	return Normalize(Block{Name: name, Cells: cells})
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
