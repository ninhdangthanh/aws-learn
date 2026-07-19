package blockpuzzle

import "testing"

func TestIsRowFull(t *testing.T) {
	board := ParseBoard(`
		XXXXX
		XXXX.
		.....`)

	tests := []struct {
		row  int
		want bool
	}{
		{0, true},
		{1, false},
		{2, false},
	}

	for _, tt := range tests {
		if got := IsRowFull(board, tt.row); got != tt.want {
			t.Errorf("IsRowFull(%d) = %v, muốn %v", tt.row, got, tt.want)
		}
	}
}

func TestIsColFull(t *testing.T) {
	board := ParseBoard(`
		X.X
		X.X
		X..`)

	tests := []struct {
		col  int
		want bool
	}{
		{0, true},
		{1, false},
		{2, false},
	}

	for _, tt := range tests {
		if got := IsColFull(board, tt.col); got != tt.want {
			t.Errorf("IsColFull(%d) = %v, muốn %v", tt.col, got, tt.want)
		}
	}
}

func TestClearFullRowsAndCols(t *testing.T) {
	tests := []struct {
		name  string
		board string
		want  ClearResult
		after string
	}{
		{
			name:  "không có gì để xoá",
			board: "X..\n...\n...",
			want:  ClearResult{Rows: 0, Cols: 0},
			after: "X..\n...\n...\n",
		},
		{
			name:  "xoá 1 hàng, các ô khác đứng yên (không có gravity)",
			board: "..X\nXXX\n...",
			want:  ClearResult{Rows: 1, Cols: 0},
			after: "..X\n...\n...\n",
		},
		{
			name:  "xoá 1 cột",
			board: "X..\nX..\nX..",
			want:  ClearResult{Rows: 0, Cols: 1},
			after: "...\n...\n...\n",
		},
		{
			name:  "xoá nhiều hàng cùng lúc",
			board: "XXX\nXXX\n...",
			want:  ClearResult{Rows: 2, Cols: 0},
			after: "...\n...\n...\n",
		},
		{
			// Đây là case bẫy: nếu xoá hàng 0 trước rồi mới kiểm tra cột,
			// cột 0 sẽ không còn đầy và bị bỏ sót.
			name:  "hàng và cột giao nhau phải cùng bị xoá",
			board: "XXX\nX..\nX.X",
			want:  ClearResult{Rows: 1, Cols: 1},
			after: "...\n...\n..X\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			board := ParseBoard(tt.board)
			got := ClearFullRowsAndCols(&board)

			if got != tt.want {
				t.Errorf("ClearFullRowsAndCols() = %+v, muốn %+v", got, tt.want)
			}
			if board.String() != tt.after {
				t.Errorf("board sau khi xoá:\n%s\nmuốn:\n%s", board.String(), tt.after)
			}
		})
	}
}
