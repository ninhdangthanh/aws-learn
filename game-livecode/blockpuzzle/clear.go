package blockpuzzle

// IsRowFull kiểm tra hàng row đã đầy chưa.
//
// Độ phức tạp: O(Width).
func IsRowFull(board Board, row int) bool {
	for x := 0; x < board.Width; x++ {
		if !board.Cells[row][x] {
			return false
		}
	}
	return true
}

// IsColFull kiểm tra cột col đã đầy chưa.
//
// Độ phức tạp: O(Height).
func IsColFull(board Board, col int) bool {
	for y := 0; y < board.Height; y++ {
		if !board.Cells[y][col] {
			return false
		}
	}
	return true
}

// ClearRow xoá sạch một hàng.
//
// Khác Tetris: ở Block Puzzle KHÔNG có gravity. Các ô phía trên đứng yên,
// hàng bị xoá chỉ đơn giản trở thành hàng trống tại chỗ.
func ClearRow(board *Board, row int) {
	for x := 0; x < board.Width; x++ {
		board.Cells[row][x] = false
	}
}

// ClearCol xoá sạch một cột.
func ClearCol(board *Board, col int) {
	for y := 0; y < board.Height; y++ {
		board.Cells[y][col] = false
	}
}

// ClearResult là số hàng và số cột đã bị xoá trong một lượt.
type ClearResult struct {
	Rows int
	Cols int
}

// Total trả về tổng số đường đã xoá, dùng để tính combo.
func (r ClearResult) Total() int { return r.Rows + r.Cols }

// ClearFullRowsAndCols xoá mọi hàng đầy và mọi cột đầy, trả về số lượng.
//
// BẪY QUAN TRỌNG — đây là chỗ interviewer hay soi:
// phải QUÉT XONG toàn bộ hàng và cột đầy TRƯỚC, rồi mới xoá.
//
// Nếu vừa quét vừa xoá, ví dụ board:
//
//	XXXXX
//	X....
//	X....
//	X....
//	X....
//
// Hàng 0 đầy và cột 0 đầy. Xoá hàng 0 trước sẽ làm cột 0 hết đầy,
// và cột 0 sẽ không bao giờ được xoá — sai luật game.
//
// Nói cách khác: điều kiện "đầy" phải được đánh giá trên CÙNG một
// snapshot của board.
//
// Độ phức tạp: O(Width * Height).
func ClearFullRowsAndCols(board *Board) ClearResult {
	var fullRows, fullCols []int

	// Giai đoạn 1: quét trên board chưa bị thay đổi.
	for y := 0; y < board.Height; y++ {
		if IsRowFull(*board, y) {
			fullRows = append(fullRows, y)
		}
	}
	for x := 0; x < board.Width; x++ {
		if IsColFull(*board, x) {
			fullCols = append(fullCols, x)
		}
	}

	// Giai đoạn 2: xoá.
	for _, y := range fullRows {
		ClearRow(board, y)
	}
	for _, x := range fullCols {
		ClearCol(board, x)
	}

	return ClearResult{Rows: len(fullRows), Cols: len(fullCols)}
}
