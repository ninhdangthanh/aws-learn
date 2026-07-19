package tetris

// Merge đóng băng khối đang rơi vào board.
//
// Gọi khi CanMoveDown trả false. Sau bước này khối không còn là thực thể
// riêng nữa, nó trở thành một phần của board.
//
// Vì sao khối đang rơi KHÔNG được ghi vào board.Cells từ đầu?
// Nếu ghi sớm, ở lượt di chuyển kế tiếp khối sẽ phát hiện chính các ô
// của mình là "đã bị chiếm" và tự chặn mình. Muốn ghi sớm thì phải xoá
// khối khỏi board trước mỗi lần kiểm tra rồi vẽ lại — phức tạp và dễ sai
// hơn hẳn. Tách riêng ActiveBlock khỏi Board là cách sạch hơn.
//
// Độ phức tạp: O(BlockSize).
func Merge(board *Board, block ActiveBlock) {
	for _, p := range block.Cells() {
		if p.X < 0 || p.X >= board.Width || p.Y < 0 || p.Y >= board.Height {
			continue // an toàn: bỏ qua ô ngoài biên thay vì panic
		}
		board.Cells[p.Y][p.X] = block.Shape.Type
	}
}

// SpawnPosition tính vị trí xuất hiện của khối mới: căn giữa theo chiều
// ngang, sát mép trên.
//
// Chia đôi phần dư sang trái để khối nghiêng về bên trái khi board có
// chiều rộng chẵn — đúng như Tetris chuẩn.
func SpawnPosition(board Board, shape Shape) Point {
	return Point{
		X: (board.Width - shape.Width(0)) / 2,
		Y: 0,
	}
}

// Spawn tạo khối mới ở vị trí xuất hiện.
//
// Trả về ok = false nếu vị trí xuất hiện đã bị chiếm — đó chính là
// điều kiện GAME OVER: rác đã chất cao tới mép trên board.
//
// Điểm đáng nói khi phỏng vấn: game over trong Tetris KHÔNG phải là
// "board đầy", mà là "không spawn được khối mới". Board vẫn còn nhiều ô
// trống ở dưới cũng không cứu được nếu đỉnh đã bị chặn.
func Spawn(board Board, shape Shape) (ActiveBlock, bool) {
	block := ActiveBlock{
		Shape:    shape,
		Position: SpawnPosition(board, shape),
		Rotation: 0,
	}
	return block, Fits(board, block)
}

// IsGameOver kiểm tra khối cho trước có spawn được không.
func IsGameOver(board Board, shape Shape) bool {
	_, ok := Spawn(board, shape)
	return !ok
}
