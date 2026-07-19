package tetris

import "testing"

func TestClearLines(t *testing.T) {
	tests := []struct {
		name  string
		board string
		want  int
		after string
	}{
		{
			name:  "không có hàng nào đầy",
			board: "X...\n.X..\n..X.",
			want:  0,
			after: "X...\n.X..\n..X.\n",
		},
		{
			name:  "xoá 1 hàng ở đáy",
			board: "X...\n.X..\nXXXX",
			want:  1,
			after: "....\nX...\n.X..\n",
		},
		{
			// Đây là điểm khác biệt cốt lõi so với Block Puzzle:
			// hàng ..X. nằm trên hàng bị xoá phải TỤT XUỐNG.
			name:  "gravity: hàng trên dồn xuống lấp chỗ trống",
			board: "..X.\nXXXX\nX..X",
			want:  1,
			after: "....\n..X.\nX..X\n",
		},
		{
			name:  "xoá nhiều hàng liền nhau",
			board: "..X.\nXXXX\nXXXX\nX..X",
			want:  2,
			after: "....\n....\n..X.\nX..X\n",
		},
		{
			name:  "xoá nhiều hàng KHÔNG liền nhau",
			board: "XXXX\n..X.\nXXXX\n.X..",
			want:  2,
			after: "....\n....\n..X.\n.X..\n",
		},
		{
			name:  "xoá sạch board",
			board: "XXXX\nXXXX",
			want:  2,
			after: "....\n....\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			board := ParseBoard(tt.board)
			got := ClearLines(&board)

			if got != tt.want {
				t.Errorf("ClearLines() = %d, muốn %d", got, tt.want)
			}
			if got := ascii(board); got != tt.after {
				t.Errorf("board sau khi xoá:\n%s\nmuốn:\n%s", got, tt.after)
			}
		})
	}
}

func TestScoreForLines(t *testing.T) {
	tests := []struct {
		name  string
		lines int
		level int
		want  int
	}{
		{"không xoá hàng nào", 0, 0, 0},
		{"1 hàng ở level 0", 1, 0, 100},
		{"4 hàng (Tetris) ở level 0", 4, 0, 800},
		{"1 hàng ở level 3 được nhân 4", 1, 3, 400},
		{"4 hàng ở level 1 được nhân 2", 4, 1, 1600},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ScoreForLines(tt.lines, tt.level); got != tt.want {
				t.Errorf("ScoreForLines(%d, %d) = %d, muốn %d", tt.lines, tt.level, got, tt.want)
			}
		})
	}
}

func TestScore_XoaGopLoiHonXoaLe(t *testing.T) {
	tetris := ScoreForLines(4, 0)
	single := ScoreForLines(1, 0) * 4

	if tetris <= single {
		t.Errorf("xoá 4 hàng cùng lúc = %d, xoá lẻ 4 lần = %d; "+
			"bảng điểm phải thưởng cho việc xoá gộp", tetris, single)
	}
}

func TestCountHoles(t *testing.T) {
	tests := []struct {
		name  string
		board string
		want  int
	}{
		{"board trống", "....\n....\n....", 0},
		{"đống xếp phẳng không có lỗ", "....\n....\nXXXX", 0},
		{"1 lỗ bị chôn dưới khối", "....\nX...\n....", 1},
		{"2 lỗ chồng nhau trong cùng cột", "X...\n....\n....", 2},
		{"lỗ ở nhiều cột khác nhau", "X.X.\n....\nXXXX", 2},
		{"ô trống nằm TRÊN đống thì không tính là lỗ", "....\n.XX.\nXXXX", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CountHoles(ParseBoard(tt.board)); got != tt.want {
				t.Errorf("CountHoles() = %d, muốn %d", got, tt.want)
			}
		})
	}
}

func TestHeight(t *testing.T) {
	board := ParseBoard(`
		..X.
		.XX.
		XXX.`)

	tests := []struct {
		col  int
		want int
	}{
		{0, 1},
		{1, 2},
		{2, 3},
		{3, 0},
	}

	for _, tt := range tests {
		if got := Height(board, tt.col); got != tt.want {
			t.Errorf("Height(cột %d) = %d, muốn %d", tt.col, got, tt.want)
		}
	}
}
