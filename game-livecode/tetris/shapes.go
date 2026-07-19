package tetris

import "sort"

// Bảy tetromino chuẩn. Toạ độ là hình dạng tại rotation 0.
//
//	I      J      L      O     S      T      Z
//	XXXX   X..    ..X    XX    .XX    XXX    XX.
//	       XXX    XXX    XX    XX.    .X.    .XX
var (
	ShapeI = Shape{Type: I, Cells: []Point{{0, 0}, {1, 0}, {2, 0}, {3, 0}}}
	ShapeJ = Shape{Type: J, Cells: []Point{{0, 0}, {0, 1}, {1, 1}, {2, 1}}}
	ShapeL = Shape{Type: L, Cells: []Point{{2, 0}, {0, 1}, {1, 1}, {2, 1}}}
	ShapeO = Shape{Type: O, Cells: []Point{{0, 0}, {1, 0}, {0, 1}, {1, 1}}}
	ShapeS = Shape{Type: S, Cells: []Point{{1, 0}, {2, 0}, {0, 1}, {1, 1}}}
	ShapeT = Shape{Type: T, Cells: []Point{{0, 0}, {1, 0}, {2, 0}, {1, 1}}}
	ShapeZ = Shape{Type: Z, Cells: []Point{{0, 0}, {1, 0}, {1, 1}, {2, 1}}}
)

// AllShapes là bộ 7 khối, dùng cho 7-bag randomizer.
var AllShapes = []Shape{ShapeI, ShapeJ, ShapeL, ShapeO, ShapeS, ShapeT, ShapeZ}

// RotatedCells trả về hình dạng tương đối sau khi xoay `rotation` lần 90 độ
// theo chiều kim đồng hồ.
//
// Công thức mỗi lần xoay: (x, y) -> (-y, x), sau đó normalize về góc
// trên bên trái.
//
// rotation được lấy modulo 4 và xử lý cả số âm, nên gọi RotatedCells(-1)
// vẫn đúng — tiện khi xoay ngược chiều kim đồng hồ.
//
// Độ phức tạp: O(4 * BlockSize), coi như hằng số.
func (s Shape) RotatedCells(rotation int) []Point {
	rotation = ((rotation % 4) + 4) % 4

	cells := make([]Point, len(s.Cells))
	copy(cells, s.Cells)

	for i := 0; i < rotation; i++ {
		for j, p := range cells {
			cells[j] = Point{X: -p.Y, Y: p.X}
		}
		cells = normalize(cells)
	}
	return normalize(cells)
}

// normalize dịch toạ độ sao cho minX = 0, minY = 0, rồi sort để hai hình
// dạng giống nhau luôn có cùng biểu diễn.
func normalize(cells []Point) []Point {
	if len(cells) == 0 {
		return cells
	}

	minX, minY := cells[0].X, cells[0].Y
	for _, p := range cells {
		if p.X < minX {
			minX = p.X
		}
		if p.Y < minY {
			minY = p.Y
		}
	}

	out := make([]Point, len(cells))
	for i, p := range cells {
		out[i] = Point{X: p.X - minX, Y: p.Y - minY}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Y != out[j].Y {
			return out[i].Y < out[j].Y
		}
		return out[i].X < out[j].X
	})
	return out
}

// Width trả về chiều rộng bounding box tại góc xoay cho trước.
func (s Shape) Width(rotation int) int {
	cells := s.RotatedCells(rotation)
	max := 0
	for _, p := range cells {
		if p.X+1 > max {
			max = p.X + 1
		}
	}
	return max
}
