package blockpuzzle

// CanPlaceAt kiểm tra có đặt được block với origin tại (x, y) hay không.
//
// Đây là hàm nền tảng — gần như mọi bài mở rộng đều gọi lại nó.
// Điều kiện: mọi ô của block sau khi cộng offset phải nằm trong biên
// và chưa bị chiếm.
//
// Độ phức tạp: O(len(block.Cells)), thực tế là O(1) vì block chỉ 1..5 ô.
func CanPlaceAt(board Board, block Block, x, y int) bool {
	for _, p := range block.Cells {
		if board.Occupied(x+p.X, y+p.Y) {
			return false
		}
	}
	return true
}

// CanPlace kiểm tra còn ÍT NHẤT MỘT vị trí nào đặt được block hay không.
//
// Đây là đề bài gốc. Cách làm: brute force mọi origin trên board,
// trả về ngay khi tìm thấy vị trí hợp lệ đầu tiên.
//
// Độ phức tạp: O(Width * Height * BlockSize).
// Board 10x10, block tối đa 5 ô => ~500 phép so sánh, cực nhanh.
//
// Lưu ý về biên: vòng lặp vẫn quét tới mép phải/mép dưới của board.
// Không cần trừ đi kích thước block, vì Board.Occupied đã tự loại các
// origin làm block tràn ra ngoài. Viết vậy tránh bug off-by-one.
func CanPlace(board Board, block Block) bool {
	for y := 0; y < board.Height; y++ {
		for x := 0; x < board.Width; x++ {
			if CanPlaceAt(board, block, x, y) {
				return true
			}
		}
	}
	return false
}

// FindPlacements trả về TẤT CẢ origin đặt được block.
//
// Biến thể của CanPlace: thay vì dừng ở kết quả đầu tiên thì gom hết lại.
// Dùng cho gợi ý nước đi (hint) và cho AI.
//
// Độ phức tạp: O(Width * Height * BlockSize).
func FindPlacements(board Board, block Block) []Point {
	var result []Point
	for y := 0; y < board.Height; y++ {
		for x := 0; x < board.Width; x++ {
			if CanPlaceAt(board, block, x, y) {
				result = append(result, Point{X: x, Y: y})
			}
		}
	}
	return result
}

// PlaceBlock đặt block vào board tại (x, y) và trả về false nếu không hợp lệ.
//
// Vì sao trả bool thay vì void? Hàm mutate state mà không validate là nguồn
// bug kinh điển: caller quên gọi CanPlaceAt trước sẽ ghi đè ô đã chiếm hoặc
// panic vì index out of range. Check ngay trong hàm giúp nó an toàn khi
// đứng một mình.
//
// Độ phức tạp: O(BlockSize).
func PlaceBlock(board *Board, block Block, x, y int) bool {
	if !CanPlaceAt(*board, block, x, y) {
		return false
	}
	for _, p := range block.Cells {
		board.Cells[y+p.Y][x+p.X] = true
	}
	return true
}
