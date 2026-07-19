package blockpuzzle

import "testing"

// render vẽ block ra ASCII để so sánh trong test cho trực quan.
func render(block Block) string {
	if len(block.Cells) == 0 {
		return ""
	}

	w, h := 0, 0
	for _, p := range block.Cells {
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
	for _, p := range block.Cells {
		grid[p.Y][p.X] = 'X'
	}

	out := ""
	for _, row := range grid {
		out += string(row) + "\n"
	}
	return out
}

func TestRotate90(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "thanh ngang thành thanh dọc",
			input: "XXX",
			want:  "X\nX\nX\n",
		},
		{
			name:  "ô vuông xoay vẫn là ô vuông",
			input: "XX\nXX",
			want:  "XX\nXX\n",
		},
		{
			name:  "chữ L",
			input: "X.\nX.\nXX",
			want:  "XXX\nX..\n",
		},
		{
			name:  "chữ T",
			input: "XXX\n.X.",
			want:  ".X\nXX\n.X\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := render(Rotate90(ParseBlock("test", tt.input)))
			if got != tt.want {
				t.Errorf("Rotate90():\n%s\nmuốn:\n%s", got, tt.want)
			}
		})
	}
}

func TestRotate90_BonVongVeChoCu(t *testing.T) {
	original := ParseBlock("L", "X.\nX.\nXX")

	rotated := original
	for i := 0; i < 4; i++ {
		rotated = Rotate90(rotated)
	}

	if render(rotated) != render(original) {
		t.Errorf("xoay 4 lần phải về hình cũ, được:\n%s\nmuốn:\n%s",
			render(rotated), render(original))
	}
}

func TestAllRotations(t *testing.T) {
	tests := []struct {
		name  string
		block string
		want  int
	}{
		{"ô đơn có 1 biến thể", "X", 1},
		{"ô vuông có 1 biến thể", "XX\nXX", 1},
		{"thanh dài có 2 biến thể", "XXX", 2},
		{"chữ S có 2 biến thể", ".XX\nXX.", 2},
		{"chữ L có 4 biến thể", "X.\nX.\nXX", 4},
		{"chữ T có 4 biến thể", "XXX\n.X.", 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := len(AllRotations(ParseBlock("test", tt.block))); got != tt.want {
				t.Errorf("AllRotations() có %d biến thể, muốn %d", got, tt.want)
			}
		})
	}
}

func TestNormalize_DichVeGocTrenTrai(t *testing.T) {
	block := Block{Cells: []Point{{X: 5, Y: 3}, {X: 6, Y: 3}, {X: 6, Y: 4}}}
	got := Normalize(block)

	if render(got) != "XX\n.X\n" {
		t.Errorf("Normalize():\n%s\nmuốn:\nXX\n.X\n", render(got))
	}
}
