package tetris

import "testing"

func TestFindBestPlacement_LapKheDeXoaHang(t *testing.T) {
	// Bốn hàng cuối chỉ thiếu đúng cột 0. Khối I xoay dọc lấp vào đó sẽ
	// xoá được cả 4 hàng — nước đi tốt nhất tuyệt đối.
	board := ParseBoard(`
		..........
		..........
		..........
		..........
		..........
		..........
		.XXXXXXXXX
		.XXXXXXXXX
		.XXXXXXXXX
		.XXXXXXXXX`)

	best, ok := FindBestPlacement(board, ShapeI)
	if !ok {
		t.Fatal("FindBestPlacement() = false, muốn tìm được nước đi")
	}

	if best.Block.Position.X != 0 {
		t.Errorf("AI đặt ở cột %d, muốn cột 0 (khe duy nhất)", best.Block.Position.X)
	}

	// Xác nhận nước đi này thật sự xoá được 4 hàng.
	sim := board.Clone()
	Merge(&sim, best.Block)
	if cleared := ClearLines(&sim); cleared != 4 {
		t.Errorf("nước AI chọn xoá được %d hàng, muốn 4", cleared)
	}
}

func TestFindBestPlacement_TranhTaoLoChon(t *testing.T) {
	// Cột 0 sâu hơn hẳn. Đặt khối O nằm đè lên miệng khe sẽ chôn ô trống
	// bên dưới — AI phải tránh nước đó.
	board := ParseBoard(`
		..........
		..........
		..........
		.XXXXXXXXX
		.XXXXXXXXX`)

	best, ok := FindBestPlacement(board, ShapeO)
	if !ok {
		t.Fatal("FindBestPlacement() = false")
	}

	sim := board.Clone()
	Merge(&sim, best.Block)
	ClearLines(&sim)

	if holes := CountHoles(sim); holes > 0 {
		t.Errorf("nước AI chọn tạo ra %d lỗ chôn, muốn 0", holes)
	}
}

func TestFindBestPlacement_BoardDayThiKhongCoNuoc(t *testing.T) {
	board := ParseBoard(`
		XXXX
		XXXX
		XXXX`)

	if _, ok := FindBestPlacement(board, ShapeO); ok {
		t.Error("FindBestPlacement() = true, muốn false khi board đầy")
	}
}
