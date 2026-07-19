package tetris

import "testing"

// renderShape vẽ hình dạng tương đối ra ASCII để so sánh trong test.
func renderShape(cells []Point) string {
	w, h := 0, 0
	for _, p := range cells {
		if p.X+1 > w {
			w = p.X + 1
		}
		if p.Y+1 > h {
			h = p.Y + 1
		}
	}

	grid := make([][]byte, h)
	for y := range grid {
		grid[y] = make([]byte, w)
		for x := range grid[y] {
			grid[y][x] = '.'
		}
	}
	for _, p := range cells {
		grid[p.Y][p.X] = 'X'
	}

	out := ""
	for _, row := range grid {
		out += string(row) + "\n"
	}
	return out
}

func TestRotatedCells(t *testing.T) {
	tests := []struct {
		name     string
		shape    Shape
		rotation int
		want     string
	}{
		{"I ở góc 0 nằm ngang", ShapeI, 0, "XXXX\n"},
		{"I xoay 1 lần thành dọc", ShapeI, 1, "X\nX\nX\nX\n"},
		{"I xoay 2 lần lại nằm ngang", ShapeI, 2, "XXXX\n"},
		{"O xoay không đổi", ShapeO, 1, "XX\nXX\n"},
		{"T ở góc 0", ShapeT, 0, "XXX\n.X.\n"},
		{"T xoay 1 lần", ShapeT, 1, ".X\nXX\n.X\n"},
		{"T xoay 2 lần lộn ngược", ShapeT, 2, ".X.\nXXX\n"},
		{"T xoay 3 lần", ShapeT, 3, "X.\nXX\nX.\n"},
		{"rotation âm bằng xoay ngược chiều", ShapeT, -1, "X.\nXX\nX.\n"},
		{"rotation vượt quá 4 được lấy modulo", ShapeT, 5, ".X\nXX\n.X\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderShape(tt.shape.RotatedCells(tt.rotation))
			if got != tt.want {
				t.Errorf("RotatedCells(%d):\n%s\nmuốn:\n%s", tt.rotation, got, tt.want)
			}
		})
	}
}

func TestRotatedCells_BonVongVeChoCu(t *testing.T) {
	for _, shape := range AllShapes {
		t.Run(shape.Type.String(), func(t *testing.T) {
			if renderShape(shape.RotatedCells(4)) != renderShape(shape.RotatedCells(0)) {
				t.Errorf("khối %s xoay 4 lần không về hình cũ", shape.Type)
			}
		})
	}
}

func TestRotate_WallKick(t *testing.T) {
	t.Run("khối I dọc sát tường phải xoay được nhờ wall kick", func(t *testing.T) {
		// Board rộng 5, khối I dọc ở cột 3. Xoay tại chỗ sẽ chiếm cột
		// 3,4,5,6 -> tràn biên. Phải kick sang trái 2 ô mới vừa.
		board := NewBoard(5, 6)
		block := ActiveBlock{Shape: ShapeI, Position: Point{X: 3, Y: 0}, Rotation: 1}

		rotated, ok := RotateCW(board, block)
		if !ok {
			t.Fatal("Rotate() = false, muốn wall kick giúp xoay thành công")
		}
		if rotated.Position.X != 1 {
			t.Errorf("X sau wall kick = %d, muốn 1 (bị đẩy trái 2 ô)", rotated.Position.X)
		}
		if !Fits(board, rotated) {
			t.Error("khối sau khi xoay nằm ở vị trí không hợp lệ")
		}
	})

	t.Run("khối bị kẹt hoàn toàn thì không xoay được", func(t *testing.T) {
		// Board chỉ rộng đúng 1 cột: khối I dọc không thể thành ngang.
		board := NewBoard(1, 6)
		block := ActiveBlock{Shape: ShapeI, Position: Point{X: 0, Y: 0}, Rotation: 1}

		rotated, ok := RotateCW(board, block)
		if ok {
			t.Error("Rotate() = true, muốn false khi không có chỗ")
		}
		if rotated.Position != block.Position || rotated.Rotation != block.Rotation {
			t.Error("xoay thất bại nhưng khối vẫn bị thay đổi")
		}
	})

	t.Run("khối O luôn xoay thành công và không đổi gì", func(t *testing.T) {
		board := NewBoard(2, 2)
		block := ActiveBlock{Shape: ShapeO, Position: Point{X: 0, Y: 0}}

		rotated, ok := RotateCW(board, block)
		if !ok || rotated.Position != block.Position || rotated.Rotation != block.Rotation {
			t.Error("khối O phải xoay thành công mà không thay đổi trạng thái")
		}
	})
}

func TestRotate_KhongXuyenQuaKhoiDaDongBang(t *testing.T) {
	// Khối I nằm dọc ở cột 0, hai bên đều bị chặn kín.
	board := ParseBoard(`
		.XXX
		.XXX
		.XXX
		.XXX`)
	block := ActiveBlock{Shape: ShapeI, Position: Point{X: 0, Y: 0}, Rotation: 1}

	if _, ok := RotateCW(board, block); ok {
		t.Error("Rotate() = true, muốn false vì mọi wall kick đều bị chặn")
	}
}
