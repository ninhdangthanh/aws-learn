package tetris

import "math/rand"

// Bag là bộ sinh khối theo thuật toán 7-bag của Tetris hiện đại.
//
// VÌ SAO KHÔNG DÙNG RANDOM THUẦN?
//
// Nếu mỗi lần chỉ rand.Intn(7), người chơi có thể xui liên tiếp: xác suất
// không thấy khối I trong 12 lượt là (6/7)^12 ≈ 16%. Không hiếm chút nào,
// và người chơi sẽ thấy game "ăn gian" chứ không phải mình chơi dở.
//
// 7-bag giải quyết bằng cách: bỏ đủ 7 loại khối vào túi, xáo trộn, rút
// hết túi rồi mới đổ túi mới. Hệ quả:
//   - Mỗi 7 lượt chắc chắn có đủ cả 7 loại, không bao giờ đói khối I.
//   - Khoảng cách xa nhất giữa hai khối cùng loại là 12 lượt (cuối túi
//     này tới đầu túi sau), có giới hạn rõ ràng.
//   - Vẫn đủ ngẫu nhiên để không đoán trước được thứ tự.
//
// Đây là ví dụ rất hay khi phỏng vấn về việc "ngẫu nhiên đúng luật phân
// phối" quan trọng hơn "ngẫu nhiên tuyệt đối" trong thiết kế game.
type Bag struct {
	rng     *rand.Rand
	current []Shape
}

// NewBag tạo bộ sinh. Truyền rng có seed cố định để test tái lập được.
func NewBag(r *rand.Rand) *Bag {
	b := &Bag{rng: r}
	b.refill()
	return b
}

// Next lấy khối tiếp theo, tự đổ túi mới khi hết.
func (b *Bag) Next() Shape {
	if len(b.current) == 0 {
		b.refill()
	}
	shape := b.current[0]
	b.current = b.current[1:]
	return shape
}

// Peek xem trước n khối sắp tới mà không lấy ra khỏi túi.
//
// Game hiện đại hiển thị 5 khối kế tiếp để người chơi lập kế hoạch.
// Cài đặt: nếu túi hiện tại không đủ, mô phỏng thêm túi mới trên bản sao
// của rng... nhưng rand.Rand không clone được, nên cách thực dụng là đổ
// sẵn túi tiếp theo vào current khi cần.
func (b *Bag) Peek(n int) []Shape {
	for len(b.current) < n {
		b.appendShuffledBag()
	}
	out := make([]Shape, n)
	copy(out, b.current[:n])
	return out
}

func (b *Bag) refill() {
	b.current = nil
	b.appendShuffledBag()
}

// appendShuffledBag nối thêm một túi 7 khối đã xáo trộn vào cuối hàng đợi.
//
// Xáo trộn bằng Fisher-Yates (rand.Shuffle của Go dùng đúng thuật toán
// này): duyệt từ cuối về đầu, mỗi bước đổi chỗ phần tử hiện tại với một
// phần tử ngẫu nhiên trong phần chưa duyệt. O(n) và phân phối đều.
func (b *Bag) appendShuffledBag() {
	bag := make([]Shape, len(AllShapes))
	copy(bag, AllShapes)

	b.rng.Shuffle(len(bag), func(i, j int) {
		bag[i], bag[j] = bag[j], bag[i]
	})

	b.current = append(b.current, bag...)
}
