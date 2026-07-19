package blockpuzzle

import (
	"reflect"
	"testing"
)

func TestCanPlaceAt(t *testing.T) {
	board := ParseBoard(`
		.....
		.....
		..X..
		.....
		.....`)

	square := ParseBlock("Square", "XX\nXX")

	tests := []struct {
		name string
		x, y int
		want bool
	}{
		{"góc trên trái, vùng trống", 0, 0, true},
		{"đè lên đúng ô đã chiếm", 2, 2, false},
		{"chạm ô đã chiếm ở góc dưới phải", 1, 1, false},
		{"tràn biên phải", 4, 0, false},
		{"tràn biên dưới", 0, 4, false},
		{"origin âm", -1, 0, false},
		{"lách sát bên phải ô đã chiếm", 3, 1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CanPlaceAt(board, square, tt.x, tt.y); got != tt.want {
				t.Errorf("CanPlaceAt(%d,%d) = %v, muốn %v", tt.x, tt.y, got, tt.want)
			}
		})
	}
}

func TestCanPlace(t *testing.T) {
	tests := []struct {
		name  string
		board string
		block Block
		want  bool
	}{
		{
			name:  "board trống luôn đặt được",
			board: ".....\n.....\n.....\n.....\n.....",
			block: ParseBlock("Square", "XX\nXX"),
			want:  true,
		},
		{
			name:  "có vật cản nhưng vẫn còn chỗ",
			board: ".....\n.....\n..X..\n.....\n.....",
			block: ParseBlock("Square", "XX\nXX"),
			want:  true,
		},
		{
			name:  "board đầy hoàn toàn",
			board: "XXX\nXXX\nXXX",
			block: ParseBlock("Single", "X"),
			want:  false,
		},
		{
			name:  "còn ô trống nhưng block quá to",
			board: "X.X\n...\nX.X",
			block: ParseBlock("Square", "XX\nXX"),
			want:  false,
		},
		{
			name:  "chỉ còn đúng 1 ô trống, block 1 ô thì vừa",
			board: "XXX\nX.X\nXXX",
			block: ParseBlock("Single", "X"),
			want:  true,
		},
		{
			name:  "block dài hơn board",
			board: "...\n...\n...",
			block: ParseBlock("Line5", "XXXXX"),
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CanPlace(ParseBoard(tt.board), tt.block); got != tt.want {
				t.Errorf("CanPlace() = %v, muốn %v", got, tt.want)
			}
		})
	}
}

func TestFindPlacements(t *testing.T) {
	board := ParseBoard(`
		...
		.X.
		...`)
	domino := ParseBlock("Domino", "XX")

	got := FindPlacements(board, domino)
	want := []Point{{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 0, Y: 2}, {X: 1, Y: 2}}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("FindPlacements() = %v, muốn %v", got, want)
	}
}

func TestPlaceBlock(t *testing.T) {
	t.Run("đặt hợp lệ thì board đổi", func(t *testing.T) {
		board := ParseBoard("...\n...\n...")
		block := ParseBlock("Corner", "XX\n.X")

		if !PlaceBlock(&board, block, 0, 0) {
			t.Fatal("PlaceBlock() = false, muốn true")
		}

		want := "XX.\n.X.\n...\n"
		if got := board.String(); got != want {
			t.Errorf("board sau khi đặt:\n%s\nmuốn:\n%s", got, want)
		}
	})

	t.Run("đặt không hợp lệ thì board không đổi", func(t *testing.T) {
		board := ParseBoard("...\n.X.\n...")
		before := board.String()

		if PlaceBlock(&board, ParseBlock("Square", "XX\nXX"), 0, 0) {
			t.Fatal("PlaceBlock() = true, muốn false")
		}
		if board.String() != before {
			t.Error("board bị thay đổi dù nước đi không hợp lệ")
		}
	})
}

func TestBoardClone(t *testing.T) {
	original := ParseBoard("...\n...\n...")
	clone := original.Clone()

	clone.Cells[0][0] = true

	if original.Cells[0][0] {
		t.Error("sửa clone làm ảnh hưởng board gốc — Clone đang copy nông")
	}
}
