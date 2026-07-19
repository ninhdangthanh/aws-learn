package tetris

import "math/rand"

// Game gói toàn bộ trạng thái một ván Tetris.
type Game struct {
	Board   Board
	Current ActiveBlock
	Hold    *Shape // khối đang cất, nil nếu chưa cất gì
	Score   int
	Lines   int // tổng số hàng đã xoá
	Level   int
	Over    bool

	bag      *Bag
	holdUsed bool // mỗi khối chỉ được Hold một lần, chống swap vô hạn
}

// NewGame tạo ván mới.
func NewGame(width, height int, r *rand.Rand) *Game {
	g := &Game{
		Board: NewBoard(width, height),
		bag:   NewBag(r),
	}
	g.spawnNext()
	return g
}

// NewStandardGame tạo ván với board 10x20 chuẩn.
func NewStandardGame(r *rand.Rand) *Game {
	return NewGame(StandardWidth, StandardHeight, r)
}

// Next trả về n khối sắp tới, để hiển thị ô "Next".
func (g *Game) Next(n int) []Shape { return g.bag.Peek(n) }

// Tick là một nhịp trọng lực — hàm được gọi mỗi khoảng thời gian nhất
// định (ví dụ 500ms, càng lên level càng nhanh).
//
// Luồng xử lý, thứ tự rất quan trọng:
//
//	rơi được  -> rơi 1 ô, kết thúc lượt
//	không rơi -> lock: merge vào board
//	          -> xoá hàng đầy + cộng điểm
//	          -> spawn khối mới
//	          -> spawn thất bại thì game over
//
// Trả về số hàng đã xoá trong nhịp này.
func (g *Game) Tick() int {
	if g.Over {
		return 0
	}

	if next, ok := MoveDown(g.Board, g.Current); ok {
		g.Current = next
		return 0
	}
	return g.lockAndRefill()
}

// lockAndRefill thực hiện chuỗi lock -> clear -> spawn.
//
// Tách riêng vì cả Tick và HardDrop đều cần đúng chuỗi này. Nếu để lặp
// code ở hai chỗ, chỉ cần sửa luật tính điểm ở một chỗ là hai đường đi
// lệch nhau — loại bug rất khó phát hiện.
func (g *Game) lockAndRefill() int {
	Merge(&g.Board, g.Current)

	cleared := ClearLines(&g.Board)
	if cleared > 0 {
		g.Score += ScoreForLines(cleared, g.Level)
		g.Lines += cleared
		g.Level = g.Lines / 10 // cứ 10 hàng lên 1 level
	}

	g.holdUsed = false // khối mới thì được quyền Hold lại
	g.spawnNext()
	return cleared
}

func (g *Game) spawnNext() {
	block, ok := Spawn(g.Board, g.bag.Next())
	g.Current = block
	if !ok {
		g.Over = true
	}
}

// MoveLeft dịch khối hiện tại sang trái. Trả về false nếu bị chặn.
func (g *Game) MoveLeft() bool {
	next, ok := MoveLeft(g.Board, g.Current)
	g.Current = next
	return ok
}

// MoveRight dịch khối hiện tại sang phải.
func (g *Game) MoveRight() bool {
	next, ok := MoveRight(g.Board, g.Current)
	g.Current = next
	return ok
}

// SoftDrop rơi nhanh 1 ô do người chơi chủ động, được cộng 1 điểm mỗi ô.
//
// Nếu không rơi được nữa thì khoá khối luôn, không đợi tick kế tiếp.
func (g *Game) SoftDrop() bool {
	if g.Over {
		return false
	}
	if next, ok := MoveDown(g.Board, g.Current); ok {
		g.Current = next
		g.Score++
		return true
	}
	g.lockAndRefill()
	return false
}

// HardDrop thả thẳng xuống đáy rồi khoá khối ngay lập tức.
//
// Cộng 2 điểm mỗi ô rơi — gấp đôi SoftDrop, vì người chơi đã cam kết
// vị trí và chấp nhận mất quyền chỉnh sửa.
func (g *Game) HardDrop() int {
	if g.Over {
		return 0
	}
	dropped, distance := HardDrop(g.Board, g.Current)
	g.Current = dropped
	g.Score += distance * 2
	return g.lockAndRefill()
}

// RotateCW xoay khối hiện tại theo chiều kim đồng hồ (có wall kick).
func (g *Game) RotateCW() bool {
	next, ok := RotateCW(g.Board, g.Current)
	g.Current = next
	return ok
}

// RotateCCW xoay ngược chiều kim đồng hồ.
func (g *Game) RotateCCW() bool {
	next, ok := RotateCCW(g.Board, g.Current)
	g.Current = next
	return ok
}

// Ghost trả về vị trí đáp của khối hiện tại, để vẽ bóng mờ.
func (g *Game) Ghost() ActiveBlock {
	return GhostPosition(g.Board, g.Current)
}

// HoldCurrent cất khối hiện tại vào ô Hold và lấy khối đã cất ra dùng.
//
// Luật: mỗi khối chỉ được Hold MỘT lần. Không có luật này, người chơi có
// thể swap qua lại vô hạn giữa hai khối để câu giờ — vừa phá cân bằng
// game vừa khiến trọng lực mất tác dụng.
//
// Trả về false nếu đã dùng quyền Hold cho khối này rồi, hoặc khối lấy ra
// không spawn được.
func (g *Game) HoldCurrent() bool {
	if g.Over || g.holdUsed {
		return false
	}

	currentShape := g.Current.Shape

	if g.Hold == nil {
		// Lần đầu cất: không có gì để đổi lại, lấy luôn khối kế tiếp.
		g.Hold = &currentShape
		g.spawnNext()
	} else {
		swapped := *g.Hold
		g.Hold = &currentShape

		block, ok := Spawn(g.Board, swapped)
		g.Current = block
		if !ok {
			g.Over = true
			return false
		}
	}

	g.holdUsed = true
	return true
}

// TickInterval trả về khoảng thời gian giữa hai nhịp rơi, tính bằng mili
// giây, theo level hiện tại.
//
// Giảm 50ms mỗi level, chặn dưới ở 100ms để game không nhanh tới mức
// không thể phản ứng.
func (g *Game) TickInterval() int {
	interval := 500 - g.Level*50
	if interval < 100 {
		return 100
	}
	return interval
}
