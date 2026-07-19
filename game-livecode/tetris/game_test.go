package tetris

import (
	"math/rand"
	"testing"
)

func TestMergeVaSpawn(t *testing.T) {
	t.Run("Merge ghi khối vào board", func(t *testing.T) {
		board := ParseBoard("....\n....\n....")
		block := ActiveBlock{Shape: ShapeO, Position: Point{X: 1, Y: 1}}

		Merge(&board, block)

		want := "....\n.OO.\n.OO.\n"
		if got := board.String(); got != want {
			t.Errorf("board sau Merge:\n%s\nmuốn:\n%s", got, want)
		}
	})

	t.Run("Spawn căn giữa khối theo chiều ngang", func(t *testing.T) {
		board := NewBoard(10, 20)
		block, ok := Spawn(board, ShapeI) // I rộng 4 ô

		if !ok {
			t.Fatal("Spawn() = false trên board trống")
		}
		if block.Position.X != 3 {
			t.Errorf("X = %d, muốn 3 ((10-4)/2)", block.Position.X)
		}
		if block.Position.Y != 0 {
			t.Errorf("Y = %d, muốn 0", block.Position.Y)
		}
	})
}

func TestIsGameOver(t *testing.T) {
	tests := []struct {
		name  string
		board string
		want  bool
	}{
		{
			name:  "board trống thì spawn được",
			board: "....\n....\n....\n....",
			want:  false,
		},
		{
			// Điểm cần nhớ: game over là "không spawn được", KHÔNG phải
			// "board đầy". Ở đây phía dưới vẫn trống nhưng đỉnh đã bị chặn.
			name:  "đỉnh bị chặn dù dưới còn trống",
			board: "XXXX\n....\n....\n....",
			want:  true,
		},
		{
			name:  "đỉnh chỉ bị chặn ở mép, khối vẫn spawn được ở giữa",
			board: "X..X\n....\n....\n....",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// ShapeO rộng 2, spawn ở x = (4-2)/2 = 1.
			if got := IsGameOver(ParseBoard(tt.board), ShapeO); got != tt.want {
				t.Errorf("IsGameOver() = %v, muốn %v", got, tt.want)
			}
		})
	}
}

func TestTick_RoiMotOMoiNhip(t *testing.T) {
	g := NewGame(10, 20, rand.New(rand.NewSource(1)))
	startY := g.Current.Position.Y

	g.Tick()

	if g.Current.Position.Y != startY+1 {
		t.Errorf("Y sau 1 tick = %d, muốn %d", g.Current.Position.Y, startY+1)
	}
}

func TestTick_ChamDayThiKhoaVaSpawnMoi(t *testing.T) {
	g := NewGame(10, 5, rand.New(rand.NewSource(1)))
	shapeBefore := g.Current.Shape.Type

	// Board trống thì chưa có ô nào bị chiếm.
	if countOccupied(g.Board) != 0 {
		t.Fatal("board mới tạo phải trống")
	}

	// Tick cho tới khi khối đầu tiên chạm đáy và bị khoá.
	ticks := 0
	for countOccupied(g.Board) == 0 && ticks < 20 {
		g.Tick()
		ticks++
	}

	if countOccupied(g.Board) == 0 {
		t.Fatalf("sau %d tick, khối %s vẫn không được merge vào board", ticks, shapeBefore)
	}

	// Khoá xong phải có khối mới xuất hiện ở đỉnh.
	if g.Current.Position.Y != 0 {
		t.Errorf("khối mới phải spawn ở Y=0, đang ở %d", g.Current.Position.Y)
	}
}

func TestHardDrop_CongDiemVaKhoaNgay(t *testing.T) {
	g := NewGame(10, 20, rand.New(rand.NewSource(1)))

	g.HardDrop()

	if g.Score == 0 {
		t.Error("HardDrop() không cộng điểm theo khoảng cách rơi")
	}
	if g.Current.Position.Y != 0 {
		t.Error("HardDrop() phải khoá khối cũ và spawn khối mới ở đỉnh")
	}
}

func TestHold(t *testing.T) {
	g := NewGame(10, 20, rand.New(rand.NewSource(1)))
	first := g.Current.Shape.Type

	t.Run("lần đầu cất thì lấy khối kế tiếp ra chơi", func(t *testing.T) {
		if !g.HoldCurrent() {
			t.Fatal("HoldCurrent() = false ở lần đầu")
		}
		if g.Hold == nil || g.Hold.Type != first {
			t.Errorf("Hold = %v, muốn %v", g.Hold, first)
		}
		if g.Current.Shape.Type == first {
			t.Error("khối hiện tại phải được thay bằng khối mới")
		}
	})

	t.Run("không được Hold hai lần cho cùng một khối", func(t *testing.T) {
		if g.HoldCurrent() {
			t.Error("HoldCurrent() = true lần thứ hai, muốn false — nếu không " +
				"người chơi có thể swap vô hạn để câu giờ")
		}
	})

	t.Run("khoá khối xong thì được Hold lại", func(t *testing.T) {
		held := g.Hold.Type
		current := g.Current.Shape.Type

		g.HardDrop() // khoá khối, reset quyền Hold

		if !g.HoldCurrent() {
			t.Fatal("HoldCurrent() = false sau khi đã khoá khối")
		}
		if g.Current.Shape.Type != held {
			t.Errorf("khối lấy ra = %v, muốn khối đã cất %v", g.Current.Shape.Type, held)
		}
		_ = current
	})
}

func TestTickInterval_GiamTheoLevel(t *testing.T) {
	tests := []struct {
		level int
		want  int
	}{
		{0, 500},
		{1, 450},
		{5, 250},
		{8, 100},
		{20, 100}, // chặn dưới
	}

	for _, tt := range tests {
		g := &Game{Level: tt.level}
		if got := g.TickInterval(); got != tt.want {
			t.Errorf("level %d: TickInterval() = %d, muốn %d", tt.level, got, tt.want)
		}
	}
}
