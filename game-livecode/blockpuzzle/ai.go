package blockpuzzle

// Move là một nước đi ứng viên mà AI cân nhắc.
type Move struct {
	Block Block // hình dạng đã xoay sẵn
	At    Point // origin trên board
	Score int   // điểm heuristic, càng cao càng tốt
}

// FindBestMove tìm nước đi tốt nhất cho một block bằng brute force.
//
// Thuật toán:
//
//	for mỗi góc xoay:
//	    for mỗi ô trên board:
//	        nếu đặt được:
//	            clone board -> đặt -> xoá đường -> chấm điểm
//	return nước có điểm cao nhất
//
// Vì sao brute force chấp nhận được? Board 10x10 = 100 vị trí, tối đa 4 góc
// xoay, mỗi lần simulate tốn O(Width*Height) cho bước xoá đường.
// => ~4 * 100 * 100 = 40.000 phép tính. Với máy tính hiện đại là tức thời.
// Không cần alpha-beta hay memo hoá gì cả.
//
// Trả về ok = false nếu block không đặt được ở đâu.
func FindBestMove(board Board, block Block, allowRotate bool) (Move, bool) {
	candidates := []Block{Normalize(block)}
	if allowRotate {
		candidates = AllRotations(block)
	}

	best := Move{Score: -1}
	found := false

	for _, shape := range candidates {
		for _, at := range FindPlacements(board, shape) {
			score := evaluateMove(board, shape, at)
			if score > best.Score {
				best = Move{Block: shape, At: at, Score: score}
				found = true
			}
		}
	}
	return best, found
}

// evaluateMove chấm điểm một nước đi trên bản clone của board.
//
// Heuristic gồm 3 thành phần, xếp theo mức độ quan trọng:
//  1. Điểm thực tế ăn được (xoá đường) — mục tiêu trực tiếp.
//  2. Số ô trống còn lại — thưởng cho việc giữ board thoáng.
//  3. Phạt lỗ thủng: ô trống bị kẹt giữa các ô đã chiếm rất khó lấp,
//     nên trừ điểm để AI ưu tiên đặt block sát nhau thay vì rải rác.
//
// Đây là chỗ để nói chuyện với interviewer: heuristic là lựa chọn thiết
// kế, không có đáp án duy nhất đúng. Điều quan trọng là giải thích được
// vì sao chọn từng thành phần và trọng số.
func evaluateMove(board Board, block Block, at Point) int {
	sim := board.Clone()
	if !PlaceBlock(&sim, block, at.X, at.Y) {
		return -1
	}

	cleared := ClearFullRowsAndCols(&sim)
	score := CalculateScore(block, cleared)
	score += CountEmpty(sim)
	score -= CountHoles(sim) * 3

	return score
}

// CountEmpty đếm số ô trống trên board.
func CountEmpty(board Board) int {
	count := 0
	for y := 0; y < board.Height; y++ {
		for x := 0; x < board.Width; x++ {
			if !board.Cells[y][x] {
				count++
			}
		}
	}
	return count
}

// CountHoles đếm số ô trống bị kẹt hoàn toàn — cả 4 hướng đều là ô đã
// chiếm hoặc là biên. Đây là những ô gần như không bao giờ lấp được nữa.
func CountHoles(board Board) int {
	count := 0
	for y := 0; y < board.Height; y++ {
		for x := 0; x < board.Width; x++ {
			if board.Cells[y][x] {
				continue
			}
			if board.Occupied(x-1, y) && board.Occupied(x+1, y) &&
				board.Occupied(x, y-1) && board.Occupied(x, y+1) {
				count++
			}
		}
	}
	return count
}

// CountEmptyRegions đếm số vùng trống liên thông bằng flood fill (BFS).
//
// Hai ô trống thuộc cùng một vùng nếu đi được từ ô này sang ô kia qua các
// ô trống theo 4 hướng. Ví dụ board dưới có 2 vùng:
//
//	XX...
//	XX...
//	XXXXX
//	..XX.
//	..XX.
//
// Ứng dụng: board bị vỡ thành nhiều vùng nhỏ là dấu hiệu sắp thua, vì
// block to sẽ không nhét vừa vùng nào. Có thể dùng làm heuristic bổ sung.
//
// Dùng BFS với queue thay vì DFS đệ quy để tránh tràn stack trên board lớn.
//
// Độ phức tạp: O(Width * Height) — mỗi ô được thăm đúng một lần.
func CountEmptyRegions(board Board) int {
	visited := make([][]bool, board.Height)
	for y := range visited {
		visited[y] = make([]bool, board.Width)
	}

	regions := 0
	for y := 0; y < board.Height; y++ {
		for x := 0; x < board.Width; x++ {
			if board.Cells[y][x] || visited[y][x] {
				continue
			}
			regions++
			floodFill(board, visited, x, y)
		}
	}
	return regions
}

func floodFill(board Board, visited [][]bool, startX, startY int) {
	queue := []Point{{X: startX, Y: startY}}
	visited[startY][startX] = true

	directions := []Point{{X: 1, Y: 0}, {X: -1, Y: 0}, {X: 0, Y: 1}, {X: 0, Y: -1}}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		for _, d := range directions {
			nx, ny := cur.X+d.X, cur.Y+d.Y
			if board.Occupied(nx, ny) || visited[ny][nx] {
				continue
			}
			visited[ny][nx] = true
			queue = append(queue, Point{X: nx, Y: ny})
		}
	}
}
