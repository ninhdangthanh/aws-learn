package blockpuzzle

import (
	"math/rand"
	"testing"
)

func TestIsGameOver(t *testing.T) {
	square := ParseBlock("Square", "XX\nXX")
	single := ParseBlock("Single", "X")

	tests := []struct {
		name   string
		board  string
		blocks []Block
		want   bool
	}{
		{
			name:   "board trống, còn block thì chưa thua",
			board:  "...\n...\n...",
			blocks: []Block{square},
			want:   false,
		},
		{
			name:   "board đầy, mọi block đều kẹt",
			board:  "XXX\nXXX\nXXX",
			blocks: []Block{square, single},
			want:   true,
		},
		{
			name:   "block to kẹt nhưng block nhỏ vẫn đặt được",
			board:  "XXX\nX.X\nXXX",
			blocks: []Block{square, single},
			want:   false,
		},
		{
			name:   "không còn block nào trên tay",
			board:  "...\n...\n...",
			blocks: nil,
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsGameOver(ParseBoard(tt.board), tt.blocks); got != tt.want {
				t.Errorf("IsGameOver() = %v, muốn %v", got, tt.want)
			}
		})
	}
}

func TestCalculateScore(t *testing.T) {
	square := ParseBlock("Square", "XX\nXX") // 4 ô

	tests := []struct {
		name    string
		cleared ClearResult
		want    int
	}{
		{"chỉ đặt block, không xoá gì", ClearResult{}, 4},
		{"xoá 1 hàng", ClearResult{Rows: 1}, 4 + 100},
		{"xoá 2 hàng, có thưởng combo", ClearResult{Rows: 2}, 4 + 200 + 50},
		{"xoá 1 hàng 1 cột", ClearResult{Rows: 1, Cols: 1}, 4 + 200 + 50},
		{"xoá 4 đường", ClearResult{Rows: 2, Cols: 2}, 4 + 400 + 150},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CalculateScore(square, tt.cleared); got != tt.want {
				t.Errorf("CalculateScore() = %d, muốn %d", got, tt.want)
			}
		})
	}
}

func TestGamePlay(t *testing.T) {
	g := NewGame(5, 3, rand.New(rand.NewSource(1)))

	t.Run("nước đi không hợp lệ bị từ chối", func(t *testing.T) {
		if g.Play(99, 0, 0) {
			t.Error("Play() với index ngoài phạm vi phải trả false")
		}
		if g.Play(0, -1, 0) {
			t.Error("Play() với toạ độ âm phải trả false")
		}
	})

	t.Run("đặt hợp lệ thì block rời khỏi tay và được cộng điểm", func(t *testing.T) {
		handBefore := len(g.Hand)
		if !g.Play(0, 0, 0) {
			t.Fatal("Play() = false, muốn true")
		}
		if len(g.Hand) != handBefore-1 {
			t.Errorf("số block trên tay = %d, muốn %d", len(g.Hand), handBefore-1)
		}
		if g.Score == 0 {
			t.Error("điểm vẫn bằng 0 sau khi đặt block")
		}
	})
}

func TestGame_HetTayThiPhatBoMoi(t *testing.T) {
	g := NewGame(10, 3, rand.New(rand.NewSource(42)))

	// Đặt hết 3 block trên tay bằng nước đi hợp lệ đầu tiên tìm được.
	for i := 0; i < 3; i++ {
		placements := FindPlacements(g.Board, g.Hand[0])
		if len(placements) == 0 {
			t.Fatal("không tìm được nước đi hợp lệ")
		}
		if !g.Play(0, placements[0].X, placements[0].Y) {
			t.Fatal("Play() = false, muốn true")
		}
	}

	if len(g.Hand) != 3 {
		t.Errorf("sau khi đặt hết, tay có %d block, muốn được phát lại 3", len(g.Hand))
	}
}
