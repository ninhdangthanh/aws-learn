package blockpuzzle

import "math/rand"

// Catalog là bộ block chuẩn hay dùng khi demo/test.
var Catalog = []Block{
	ParseBlock("Single", "X"),
	ParseBlock("Domino", "XX"),
	ParseBlock("Line3", "XXX"),
	ParseBlock("Line5", "XXXXX"),
	ParseBlock("Square", "XX\nXX"),
	ParseBlock("L", "X.\nX.\nXX"),
	ParseBlock("T", "XXX\n.X."),
	ParseBlock("S", ".XX\nXX."),
	ParseBlock("Corner", "X.\nXX"),
}

// RandomBlock lấy ngẫu nhiên một block trong catalog.
//
// Nhận *rand.Rand thay vì dùng global rand để test có thể truyền seed cố
// định và tái lập được kết quả.
func RandomBlock(r *rand.Rand) Block {
	return Catalog[r.Intn(len(Catalog))]
}

// IsGameOver trả về true khi KHÔNG block nào trong tay đặt được ở bất kỳ đâu.
//
// Ở Block Puzzle người chơi thường được phát 3 block cùng lúc và đặt theo
// thứ tự tuỳ ý, nên chỉ cần MỘT block còn chỗ là chưa thua.
//
// Độ phức tạp: O(len(blocks) * Width * Height * BlockSize).
func IsGameOver(board Board, blocks []Block) bool {
	for _, b := range blocks {
		if CanPlace(board, b) {
			return false
		}
	}
	return true
}

// Hằng số tính điểm. Con số cụ thể tuỳ luật game, ở đây chọn cho dễ hiểu.
const (
	PointsPerCell = 1   // mỗi ô của block đặt xuống
	PointsPerLine = 100 // mỗi hàng hoặc cột xoá được
	ComboBonus    = 50  // thưởng thêm cho mỗi đường kể từ đường thứ 2
)

// CalculateScore tính điểm cho một lượt đặt block.
//
// Công thức: điểm đặt block + điểm xoá đường + thưởng combo.
// Combo thưởng theo cấp số cộng để xoá nhiều đường cùng lúc luôn lợi hơn
// xoá lẻ tẻ từng đường — đây là thứ tạo chiều sâu chiến thuật cho game.
func CalculateScore(block Block, cleared ClearResult) int {
	score := len(block.Cells) * PointsPerCell
	score += cleared.Total() * PointsPerLine
	if n := cleared.Total(); n > 1 {
		score += (n - 1) * ComboBonus
	}
	return score
}

// Game gói toàn bộ trạng thái một ván chơi.
type Game struct {
	Board  Board
	Hand   []Block // các block đang cầm trên tay
	Score  int
	Over   bool
	rng    *rand.Rand
	handSz int
}

// NewGame tạo ván mới với board size x size và handSize block trên tay.
func NewGame(size, handSize int, r *rand.Rand) *Game {
	g := &Game{
		Board:  NewBoard(size, size),
		Score:  0,
		rng:    r,
		handSz: handSize,
	}
	g.refillHand()
	return g
}

// Play đặt block thứ index trong tay xuống (x, y).
//
// Một lượt chơi đầy đủ gồm 4 bước, thứ tự quan trọng:
//  1. Đặt block (thất bại thì không đổi gì cả)
//  2. Xoá hàng/cột đầy
//  3. Cộng điểm
//  4. Bỏ block khỏi tay, hết tay thì phát mới, rồi check game over
//
// Trả về false nếu nước đi không hợp lệ.
func (g *Game) Play(index, x, y int) bool {
	if g.Over || index < 0 || index >= len(g.Hand) {
		return false
	}

	block := g.Hand[index]
	if !PlaceBlock(&g.Board, block, x, y) {
		return false
	}

	cleared := ClearFullRowsAndCols(&g.Board)
	g.Score += CalculateScore(block, cleared)

	g.Hand = append(g.Hand[:index], g.Hand[index+1:]...)
	if len(g.Hand) == 0 {
		g.refillHand()
	}

	// Check game over SAU khi đã phát block mới — vì bộ block mới quyết
	// định người chơi còn nước đi hay không.
	g.Over = IsGameOver(g.Board, g.Hand)
	return true
}

func (g *Game) refillHand() {
	g.Hand = make([]Block, 0, g.handSz)
	for i := 0; i < g.handSz; i++ {
		g.Hand = append(g.Hand, RandomBlock(g.rng))
	}
}
