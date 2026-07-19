package tetris

// Fits kiểm tra khối ở trạng thái hiện tại có nằm hợp lệ trên board không.
//
// Hàm nền tảng của toàn bộ package: mọi thao tác di chuyển, xoay, spawn
// đều quy về "thử tạo trạng thái mới rồi hỏi Fits".
//
// Mẫu code này (tạo bản thử -> kiểm tra -> chỉ commit khi hợp lệ) tránh
// được cả một lớp bug: không bao giờ có trạng thái trung gian không hợp lệ.
//
// Độ phức tạp: O(BlockSize) = O(4).
func Fits(board Board, block ActiveBlock) bool {
	for _, p := range block.Cells() {
		if board.Occupied(p.X, p.Y) {
			return false
		}
	}
	return true
}

// CanMove kiểm tra khối có dịch được (dx, dy) không.
func CanMove(board Board, block ActiveBlock, dx, dy int) bool {
	moved := block
	moved.Position = Point{X: block.Position.X + dx, Y: block.Position.Y + dy}
	return Fits(board, moved)
}

// CanMoveDown là câu hỏi được hỏi nhiều nhất trong game: khối còn rơi
// tiếp được không? Trả false nghĩa là đã tới lúc merge khối vào board.
func CanMoveDown(board Board, block ActiveBlock) bool {
	return CanMove(board, block, 0, 1)
}

// Move dịch khối đi (dx, dy). Trả về khối mới và false nếu bị chặn.
//
// Trả về giá trị mới thay vì mutate qua con trỏ: ActiveBlock là struct
// nhỏ, copy rẻ, và cách này khiến "nước đi bị chặn" không thể làm hỏng
// trạng thái cũ.
func Move(board Board, block ActiveBlock, dx, dy int) (ActiveBlock, bool) {
	moved := block
	moved.Position = Point{X: block.Position.X + dx, Y: block.Position.Y + dy}
	if !Fits(board, moved) {
		return block, false
	}
	return moved, true
}

// MoveLeft dịch trái 1 ô.
func MoveLeft(board Board, block ActiveBlock) (ActiveBlock, bool) {
	return Move(board, block, -1, 0)
}

// MoveRight dịch phải 1 ô.
func MoveRight(board Board, block ActiveBlock) (ActiveBlock, bool) {
	return Move(board, block, 1, 0)
}

// MoveDown rơi xuống 1 ô. Đây chính là hành động của mỗi tick.
func MoveDown(board Board, block ActiveBlock) (ActiveBlock, bool) {
	return Move(board, block, 0, 1)
}

// HardDrop thả khối rơi thẳng xuống vị trí thấp nhất có thể.
//
// Trả về khối ở vị trí cuối và số ô đã rơi (nhiều game cộng điểm theo
// khoảng cách này để thưởng người chơi thả nhanh).
//
// Cách làm: lặp MoveDown cho tới khi không đi được nữa.
// Độ phức tạp: O(Height * BlockSize). Board cao 20 nên hoàn toàn ổn;
// không cần tối ưu bằng cách tính trước độ cao từng cột.
//
// Lưu ý: HardDrop chỉ ĐỊNH VỊ, không merge. Tách hai việc ra để Ghost
// Piece tái sử dụng được chính hàm này.
func HardDrop(board Board, block ActiveBlock) (ActiveBlock, int) {
	distance := 0
	for {
		next, ok := MoveDown(board, block)
		if !ok {
			return block, distance
		}
		block = next
		distance++
	}
}

// GhostPosition trả về vị trí mà khối sẽ đáp xuống nếu thả ngay bây giờ.
//
// Đây chính là cái bóng mờ hiển thị dưới đáy trong game hiện đại, giúp
// người chơi ước lượng vị trí. Cài đặt chỉ là HardDrop mà không merge.
func GhostPosition(board Board, block ActiveBlock) ActiveBlock {
	ghost, _ := HardDrop(board, block)
	return ghost
}
