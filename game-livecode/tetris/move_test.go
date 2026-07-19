package tetris

import "testing"

func TestCanMoveDown(t *testing.T) {
	tests := []struct {
		name  string
		board string
		block ActiveBlock
		want  bool
	}{
		{
			name:  "giữa board trống thì rơi được",
			board: "....\n....\n....\n....",
			block: ActiveBlock{Shape: ShapeO, Position: Point{X: 0, Y: 0}},
			want:  true,
		},
		{
			name:  "đã chạm đáy board",
			board: "....\n....\n....\n....",
			block: ActiveBlock{Shape: ShapeO, Position: Point{X: 0, Y: 2}},
			want:  false,
		},
		{
			name:  "chạm khối đã đóng băng bên dưới",
			board: "....\n....\nXX..\n....",
			block: ActiveBlock{Shape: ShapeO, Position: Point{X: 0, Y: 0}},
			want:  false,
		},
		{
			name:  "khối đã đóng băng lệch sang bên, vẫn rơi được",
			board: "....\n....\n..XX\n....",
			block: ActiveBlock{Shape: ShapeO, Position: Point{X: 0, Y: 0}},
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CanMoveDown(ParseBoard(tt.board), tt.block); got != tt.want {
				t.Errorf("CanMoveDown() = %v, muốn %v", got, tt.want)
			}
		})
	}
}

func TestMoveLeftRight(t *testing.T) {
	board := ParseBoard(`
		.....
		.....
		.....`)
	block := ActiveBlock{Shape: ShapeO, Position: Point{X: 2, Y: 0}}

	t.Run("dịch trái thành công", func(t *testing.T) {
		moved, ok := MoveLeft(board, block)
		if !ok || moved.Position.X != 1 {
			t.Errorf("MoveLeft() = %v, %v; muốn X=1, true", moved.Position, ok)
		}
	})

	t.Run("bị tường trái chặn", func(t *testing.T) {
		atWall := ActiveBlock{Shape: ShapeO, Position: Point{X: 0, Y: 0}}
		moved, ok := MoveLeft(board, atWall)
		if ok {
			t.Error("MoveLeft() = true, muốn false khi sát tường")
		}
		if moved.Position != atWall.Position {
			t.Error("nước đi bị chặn nhưng vị trí vẫn thay đổi")
		}
	})

	t.Run("bị tường phải chặn", func(t *testing.T) {
		atWall := ActiveBlock{Shape: ShapeO, Position: Point{X: 3, Y: 0}}
		if _, ok := MoveRight(board, atWall); ok {
			t.Error("MoveRight() = true, muốn false khi sát tường phải")
		}
	})
}

func TestHardDrop(t *testing.T) {
	tests := []struct {
		name         string
		board        string
		block        ActiveBlock
		wantY        int
		wantDistance int
	}{
		{
			name:         "rơi thẳng xuống đáy board trống",
			board:        "....\n....\n....\n....",
			block:        ActiveBlock{Shape: ShapeO, Position: Point{X: 0, Y: 0}},
			wantY:        2,
			wantDistance: 2,
		},
		{
			name:         "dừng trên đống đã có sẵn",
			board:        "....\n....\n....\nXX..",
			block:        ActiveBlock{Shape: ShapeO, Position: Point{X: 0, Y: 0}},
			wantY:        1,
			wantDistance: 1,
		},
		{
			name:         "đã nằm sát đáy thì không rơi thêm",
			board:        "....\n....\n....\n....",
			block:        ActiveBlock{Shape: ShapeO, Position: Point{X: 0, Y: 2}},
			wantY:        2,
			wantDistance: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dropped, distance := HardDrop(ParseBoard(tt.board), tt.block)
			if dropped.Position.Y != tt.wantY {
				t.Errorf("Y sau khi thả = %d, muốn %d", dropped.Position.Y, tt.wantY)
			}
			if distance != tt.wantDistance {
				t.Errorf("khoảng cách rơi = %d, muốn %d", distance, tt.wantDistance)
			}
		})
	}
}

func TestGhostPosition_KhongDungVaoBoard(t *testing.T) {
	board := ParseBoard("....\n....\n....\n....")
	before := board.String()

	block := ActiveBlock{Shape: ShapeO, Position: Point{X: 0, Y: 0}}
	ghost := GhostPosition(board, block)

	if ghost.Position.Y != 2 {
		t.Errorf("ghost ở Y=%d, muốn 2", ghost.Position.Y)
	}
	if board.String() != before {
		t.Error("GhostPosition() đã làm thay đổi board — phải thuần đọc")
	}
}
