package tetris

// Placement là một nước đi ứng viên: xoay bao nhiêu, đặt ở cột nào.
type Placement struct {
	Rotation int
	Block    ActiveBlock // trạng thái sau khi đã thả xuống đáy
	Score    float64
}

// FindBestPlacement tìm nước đi tốt nhất cho khối hiện tại.
//
// Thuật toán vét cạn:
//
//	for mỗi góc xoay (tối đa 4):
//	    for mỗi cột (tối đa 10):
//	        thả khối xuống -> clone board -> merge -> xoá hàng -> chấm điểm
//
// Không gian tìm kiếm chỉ 4 x 10 = 40 nước, mỗi nước tốn O(W*H) để chấm
// điểm => khoảng 8.000 phép tính. Vét cạn thoải mái, không cần cắt tỉa.
//
// Nếu interviewer hỏi mở rộng "nhìn trước 2 khối": nhân thêm 40 lần nữa
// = 1.600 nước, vẫn chạy được. Nhìn trước 3 khối thì 64.000 — bắt đầu
// cần cắt tỉa hoặc chỉ giữ top-N nước tốt nhất ở mỗi tầng.
//
// Trả về ok = false nếu không đặt được ở đâu.
func FindBestPlacement(board Board, shape Shape) (Placement, bool) {
	best := Placement{Score: -1e18}
	found := false

	for rotation := 0; rotation < 4; rotation++ {
		width := shape.Width(rotation)

		for x := 0; x <= board.Width-width; x++ {
			block := ActiveBlock{Shape: shape, Position: Point{X: x, Y: 0}, Rotation: rotation}
			if !Fits(board, block) {
				continue // cột đã bị chất kín tới nóc
			}

			dropped, _ := HardDrop(board, block)
			score := evaluate(board, dropped)

			if score > best.Score {
				best = Placement{Rotation: rotation, Block: dropped, Score: score}
				found = true
			}
		}
	}
	return best, found
}

// Trọng số heuristic, lấy theo bộ nổi tiếng của Pierre Dellacherie —
// bộ này chơi Tetris rất tốt dù chỉ nhìn 1 khối.
//
// Dấu âm nghĩa là "càng nhiều càng tệ".
const (
	weightLines     = 0.76  // thưởng cho xoá hàng
	weightHeight    = -0.51 // tổng chiều cao: xếp càng cao càng nguy hiểm
	weightHoles     = -0.36 // lỗ chôn: tệ nhất, gần như không sửa được
	weightBumpiness = -0.18 // độ gồ ghề: bề mặt lởm chởm khó đặt khối
)

// evaluate chấm điểm board sau khi đặt khối.
//
// Bốn thành phần và lý do:
//
//  1. Số hàng xoá được — mục tiêu trực tiếp của game.
//  2. Tổng chiều cao — xếp thấp thì còn nhiều chỗ xoay xở.
//  3. Số lỗ chôn — mỗi lỗ khoá cứng một hàng cho tới khi đào xong.
//  4. Độ gồ ghề (tổng chênh lệch chiều cao giữa các cột liền kề) —
//     bề mặt phẳng nhận được nhiều loại khối hơn. Ngoại lệ có chủ ý là
//     người chơi giỏi cố tình chừa một khe sâu để chờ khối I ăn 4 hàng.
//
// Trọng số là lựa chọn thiết kế, không có đáp án duy nhất. Đây là chỗ
// đáng để thảo luận với interviewer thay vì khẳng định con số nào đúng.
func evaluate(board Board, block ActiveBlock) float64 {
	sim := board.Clone()
	Merge(&sim, block)
	cleared := ClearLines(&sim)

	totalHeight := 0
	bumpiness := 0
	prev := Height(sim, 0)

	for x := 0; x < sim.Width; x++ {
		h := Height(sim, x)
		totalHeight += h
		if x > 0 {
			bumpiness += abs(h - prev)
		}
		prev = h
	}

	return weightLines*float64(cleared) +
		weightHeight*float64(totalHeight) +
		weightHoles*float64(CountHoles(sim)) +
		weightBumpiness*float64(bumpiness)
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
