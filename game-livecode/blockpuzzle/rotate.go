package blockpuzzle

import "sort"

// Normalize dịch toàn bộ toạ độ sao cho minX = 0 và minY = 0.
//
// Vì sao cần? Sau khi xoay, toạ độ có thể âm. Normalize kéo block về góc
// trên bên trái để origin luôn có nghĩa thống nhất, nhờ đó CanPlaceAt và
// việc so sánh hai block bằng nhau vẫn đúng.
//
// Cells cũng được sort để hai block cùng hình dạng luôn có cùng biểu diễn —
// tiện cho việc so sánh trong test và loại trùng ở AllRotations.
func Normalize(block Block) Block {
	if len(block.Cells) == 0 {
		return block
	}

	minX, minY := block.Cells[0].X, block.Cells[0].Y
	for _, p := range block.Cells {
		if p.X < minX {
			minX = p.X
		}
		if p.Y < minY {
			minY = p.Y
		}
	}

	cells := make([]Point, len(block.Cells))
	for i, p := range block.Cells {
		cells[i] = Point{X: p.X - minX, Y: p.Y - minY}
	}

	sort.Slice(cells, func(i, j int) bool {
		if cells[i].Y != cells[j].Y {
			return cells[i].Y < cells[j].Y
		}
		return cells[i].X < cells[j].X
	})

	return Block{Name: block.Name, Cells: cells}
}

// Rotate90 xoay block 90 độ theo chiều kim đồng hồ.
//
// Công thức: (x, y) -> (-y, x).
//
// Suy ra nhanh khi phỏng vấn bằng cách thử một điểm: với trục Y hướng
// XUỐNG (quy ước của màn hình), điểm (1, 0) — bên phải origin — sau khi
// xoay theo chiều kim đồng hồ phải nằm BÊN DƯỚI origin, tức (0, 1).
// Thay vào công thức: (1, 0) -> (-0, 1) = (0, 1). Khớp.
//
// Nếu xoay ngược chiều kim đồng hồ thì dùng (x, y) -> (y, -x).
//
// Độ phức tạp: O(BlockSize log BlockSize) vì Normalize có sort.
func Rotate90(block Block) Block {
	rotated := Block{Name: block.Name, Cells: make([]Point, len(block.Cells))}
	for i, p := range block.Cells {
		rotated.Cells[i] = Point{X: -p.Y, Y: p.X}
	}
	return Normalize(rotated)
}

// AllRotations trả về các hình dạng khác nhau của block qua 4 góc xoay,
// đã loại bỏ trùng lặp.
//
// Ví dụ số lượng biến thể thực tế:
//   - Ô vuông 2x2: 1 (xoay kiểu gì cũng như cũ)
//   - Thanh dài: 2 (ngang, dọc)
//   - Chữ L: 4
//
// Loại trùng quan trọng cho AI: đỡ phải simulate lại cùng một hình dạng.
func AllRotations(block Block) []Block {
	var result []Block
	seen := make(map[string]bool)

	current := Normalize(block)
	for i := 0; i < 4; i++ {
		if key := shapeKey(current); !seen[key] {
			seen[key] = true
			result = append(result, current)
		}
		current = Rotate90(current)
	}
	return result
}

// shapeKey sinh khoá so sánh hình dạng. Dựa vào việc Normalize đã sort Cells
// nên cùng hình dạng luôn cho cùng chuỗi.
func shapeKey(block Block) string {
	buf := make([]byte, 0, len(block.Cells)*2)
	for _, p := range block.Cells {
		buf = append(buf, byte(p.X), byte(p.Y))
	}
	return string(buf)
}
