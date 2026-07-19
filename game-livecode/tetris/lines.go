package tetris

// IsRowFull kiểm tra hàng row đã đầy chưa.
//
// Độ phức tạp: O(Width).
func IsRowFull(board Board, row int) bool {
	for x := 0; x < board.Width; x++ {
		if board.Cells[row][x] == Empty {
			return false
		}
	}
	return true
}

// FullRows trả về danh sách chỉ số các hàng đầy, từ trên xuống dưới.
func FullRows(board Board) []int {
	var rows []int
	for y := 0; y < board.Height; y++ {
		if IsRowFull(board, y) {
			rows = append(rows, y)
		}
	}
	return rows
}

// ClearLines xoá mọi hàng đầy và DỒN các hàng phía trên xuống (gravity).
// Trả về số hàng đã xoá.
//
// Đây là khác biệt lớn nhất so với Block Puzzle, nơi ô đứng yên tại chỗ:
//
//	.....        .....
//	XXXXX   ->   .....
//	.....        .....
//	AAAAA        AAAAA
//
// Hàng XXXXX bị xoá, mọi hàng phía trên nó tụt xuống 1 ô.
//
// CÁCH CÀI ĐẶT — con trỏ hai đầu, quét từ DƯỚI LÊN:
//
//	write đi từ đáy lên, đánh dấu "hàng tiếp theo sẽ được ghi vào đâu"
//	read  đi từ đáy lên, đánh dấu "đang xét hàng nào"
//	hàng nào đầy thì read bỏ qua (write đứng yên -> hàng đó bị nuốt)
//	hàng nào không đầy thì copy read -> write, cả hai cùng đi lên
//	cuối cùng, các hàng còn dư ở trên cùng được xoá trắng
//
// Vì sao quét từ dưới lên chứ không phải trên xuống? Vì các hàng dồn
// XUỐNG. Đi từ đáy thì nguồn (read) luôn ở phía trên đích (write),
// nên không bao giờ ghi đè lên dữ liệu chưa kịp đọc.
//
// Vì sao không dùng cách "gặp hàng đầy thì cắt slice rồi chèn hàng mới
// lên đầu"? Cách đó cũng đúng và ngắn hơn, nhưng cấp phát bộ nhớ mới mỗi
// lần xoá. Cách hai con trỏ chạy tại chỗ, một lượt duy nhất.
//
// Độ phức tạp: O(Width * Height), không phụ thuộc số hàng bị xoá.
func ClearLines(board *Board) int {
	write := board.Height - 1

	for read := board.Height - 1; read >= 0; read-- {
		if IsRowFull(*board, read) {
			continue // nuốt hàng này
		}
		if write != read {
			copy(board.Cells[write], board.Cells[read])
		}
		write--
	}

	cleared := write + 1

	// Phần trên cùng còn sót dữ liệu cũ, phải xoá trắng.
	for y := write; y >= 0; y-- {
		for x := 0; x < board.Width; x++ {
			board.Cells[y][x] = Empty
		}
	}

	return cleared
}

// Điểm thưởng theo số hàng xoá cùng lúc.
//
// Bảng điểm cố tình phi tuyến: xoá 4 hàng một lúc (gọi là "Tetris") được
// 800 điểm, gấp đôi so với xoá 4 lần lẻ từng hàng (4 x 100 = 400).
// Đây là thứ khiến người chơi giỏi chấp nhận rủi ro xếp cao chờ khối I,
// thay vì xoá lắt nhắt cho an toàn.
var lineScores = map[int]int{
	0: 0,
	1: 100,
	2: 300,
	3: 500,
	4: 800,
}

// ScoreForLines tính điểm cho số hàng xoá được, có nhân theo level.
//
// Nhân level để game càng về sau càng nhiều điểm, giữ động lực chơi tiếp.
func ScoreForLines(lines, level int) int {
	base, ok := lineScores[lines]
	if !ok {
		base = lineScores[4] // phòng khi có biến thể xoá được hơn 4 hàng
	}
	return base * (level + 1)
}

// Height trả về chiều cao của cột x — khoảng cách từ đỉnh board tới ô đã
// chiếm cao nhất của cột đó. Cột trống trả về 0.
//
// Dùng cho AI và cho việc đánh giá độ nguy hiểm của board.
func Height(board Board, x int) int {
	for y := 0; y < board.Height; y++ {
		if board.Cells[y][x] != Empty {
			return board.Height - y
		}
	}
	return 0
}

// CountHoles đếm số ô trống bị chôn bên dưới một ô đã chiếm.
//
// Đây là chỉ số quan trọng nhất để đánh giá board Tetris: mỗi lỗ chôn là
// một hàng không thể xoá được cho tới khi đào hết phần bên trên nó.
//
// Độ phức tạp: O(Width * Height).
func CountHoles(board Board) int {
	holes := 0
	for x := 0; x < board.Width; x++ {
		blocked := false
		for y := 0; y < board.Height; y++ {
			if board.Cells[y][x] != Empty {
				blocked = true
			} else if blocked {
				holes++
			}
		}
	}
	return holes
}
