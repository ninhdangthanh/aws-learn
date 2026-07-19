package blockpuzzle

import "testing"

func TestFindBestMove_UuTienNuocXoaDuoc(t *testing.T) {
	// Hàng cuối chỉ còn thiếu đúng 1 ô ở cột 4.
	board := ParseBoard(`
		.....
		.....
		.....
		.....
		XXXX.`)

	single := ParseBlock("Single", "X")

	move, ok := FindBestMove(board, single, false)
	if !ok {
		t.Fatal("FindBestMove() = false, muốn tìm được nước đi")
	}

	want := Point{X: 4, Y: 4}
	if move.At != want {
		t.Errorf("AI chọn %v, muốn %v (ô lấp đầy hàng cuối)", move.At, want)
	}
}

func TestFindBestMove_BoardDayThiKhongCoNuoc(t *testing.T) {
	board := ParseBoard("XXX\nXXX\nXXX")

	if _, ok := FindBestMove(board, ParseBlock("Single", "X"), true); ok {
		t.Error("FindBestMove() = true, muốn false khi board đầy")
	}
}

func TestFindBestMove_XoayDeVuaChoTrong(t *testing.T) {
	// Chỉ còn khe dọc 3 ô ở cột 1. Thanh ngang phải xoay mới đặt được.
	board := ParseBoard(`
		X.X
		X.X
		X.X`)

	line := ParseBlock("Line3", "XXX")

	if _, ok := FindBestMove(board, line, false); ok {
		t.Error("không xoay thì không được đặt vừa, muốn ok = false")
	}

	move, ok := FindBestMove(board, line, true)
	if !ok {
		t.Fatal("cho phép xoay thì phải đặt được")
	}
	if move.At != (Point{X: 1, Y: 0}) {
		t.Errorf("AI chọn %v, muốn {1 0}", move.At)
	}
}

func TestCountHoles(t *testing.T) {
	tests := []struct {
		name  string
		board string
		want  int
	}{
		{"board trống không có lỗ", "...\n...\n...", 0},
		{"1 ô trống bị vây kín", "XXX\nX.X\nXXX", 1},
		{"ô trống ở góc, biên tính là tường", "X.\nXX", 1},
		{"khe thoáng không phải lỗ", "X.X\n...\nX.X", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CountHoles(ParseBoard(tt.board)); got != tt.want {
				t.Errorf("CountHoles() = %d, muốn %d", got, tt.want)
			}
		})
	}
}

func TestCountEmptyRegions(t *testing.T) {
	tests := []struct {
		name  string
		board string
		want  int
	}{
		{"board trống là 1 vùng", "...\n...\n...", 1},
		{"board đầy có 0 vùng", "XXX\nXXX\nXXX", 0},
		{
			name:  "tường ngang chia board thành 2 vùng",
			board: "...\nXXX\n...",
			want:  2,
		},
		{
			name:  "4 góc bị tách rời thành 4 vùng",
			board: ".X.\nXXX\n.X.",
			want:  4,
		},
		{
			name:  "vùng nối chéo không tính là liên thông",
			board: ".X\nX.",
			want:  2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CountEmptyRegions(ParseBoard(tt.board)); got != tt.want {
				t.Errorf("CountEmptyRegions() = %d, muốn %d", got, tt.want)
			}
		})
	}
}
