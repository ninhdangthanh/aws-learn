package tetris

// wallKickOffsets là danh sách dịch chuyển thử khi xoay bị vướng.
//
// Thứ tự có chủ ý: thử tại chỗ trước, rồi đẩy sang ngang 1 ô, rồi 2 ô
// (cần cho khối I dài 4 ô sát tường), cuối cùng mới thử đẩy lên.
//
// Đẩy lên xếp cuối vì nó thay đổi độ cao của khối — cảm giác chơi sẽ lạ
// nếu ưu tiên nó hơn dịch ngang.
//
// Đây là phiên bản đơn giản hoá. Tetris chuẩn dùng bảng SRS (Super
// Rotation System) với offset riêng cho từng cặp góc xoay và từng loại
// khối. Nếu interviewer hỏi, nói rõ: cách này giữ được tinh thần
// "xoay vướng thì thử dịch" mà không phải thuộc lòng bảng tra.
var wallKickOffsets = []Point{
	{X: 0, Y: 0},
	{X: -1, Y: 0},
	{X: 1, Y: 0},
	{X: -2, Y: 0},
	{X: 2, Y: 0},
	{X: 0, Y: -1},
}

// Rotate xoay khối theo chiều kim đồng hồ (dir = 1) hoặc ngược lại
// (dir = -1), có áp dụng wall kick.
//
// Vì sao cần wall kick? Khối I nằm dọc sát tường trái, xoay thành ngang
// sẽ tràn ra ngoài biên:
//
//	|X          |XXXX      -> vướng tường
//	|X    xoay
//	|X          dịch phải 1 ô rồi xoay:
//	|X          |.XXXX     -> hợp lệ
//
// Không có wall kick, người chơi sẽ thấy game "không cho xoay" một cách
// vô lý ở sát tường.
//
// Trả về false khi cả 6 offset đều không hợp lệ (khối bị kẹt hoàn toàn).
func Rotate(board Board, block ActiveBlock, dir int) (ActiveBlock, bool) {
	// Khối O xoay kiểu gì cũng như cũ, bỏ qua cho khỏi tốn công.
	if block.Shape.Type == O {
		return block, true
	}

	for _, kick := range wallKickOffsets {
		candidate := block
		candidate.Rotation = ((block.Rotation+dir)%4 + 4) % 4
		candidate.Position = Point{
			X: block.Position.X + kick.X,
			Y: block.Position.Y + kick.Y,
		}

		if Fits(board, candidate) {
			return candidate, true
		}
	}
	return block, false
}

// RotateCW xoay theo chiều kim đồng hồ.
func RotateCW(board Board, block ActiveBlock) (ActiveBlock, bool) {
	return Rotate(board, block, 1)
}

// RotateCCW xoay ngược chiều kim đồng hồ.
func RotateCCW(board Board, block ActiveBlock) (ActiveBlock, bool) {
	return Rotate(board, block, -1)
}
