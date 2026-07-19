package tetris

import (
	"math/rand"
	"testing"
)

func TestBag_MoiTuiCoDuBayKhoi(t *testing.T) {
	bag := NewBag(rand.New(rand.NewSource(1)))

	// Kiểm tra 3 túi liên tiếp.
	for round := 0; round < 3; round++ {
		seen := make(map[ShapeType]int)
		for i := 0; i < 7; i++ {
			seen[bag.Next().Type]++
		}

		if len(seen) != 7 {
			t.Errorf("túi thứ %d chỉ có %d loại khối, muốn đủ 7", round+1, len(seen))
		}
		for shape, count := range seen {
			if count != 1 {
				t.Errorf("túi thứ %d: khối %v xuất hiện %d lần, muốn đúng 1", round+1, shape, count)
			}
		}
	}
}

func TestBag_KhoangCachToiDaGiuaHaiKhoiCungLoai(t *testing.T) {
	bag := NewBag(rand.New(rand.NewSource(7)))

	// Lấy 700 khối và đo khoảng cách xa nhất giữa hai lần cùng loại.
	//
	// 7-bag đảm bảo khoảng cách này không vượt quá 13: trường hợp tệ nhất
	// là khối nằm ĐẦU túi này và CUỐI túi sau, tức index 0 và index 13,
	// với 12 khối khác xen giữa.
	//
	// Đây chính là thứ random thuần không làm được — với rand.Intn(7),
	// khoảng cách về lý thuyết là vô hạn.
	lastSeen := make(map[ShapeType]int)
	maxGap := 0

	for i := 0; i < 700; i++ {
		shape := bag.Next().Type
		if prev, ok := lastSeen[shape]; ok {
			if gap := i - prev; gap > maxGap {
				maxGap = gap
			}
		}
		lastSeen[shape] = i
	}

	if maxGap > 13 {
		t.Errorf("khoảng cách xa nhất giữa hai khối cùng loại = %d, "+
			"7-bag phải đảm bảo tối đa 13", maxGap)
	}
}

func TestBag_PeekKhongLamMatKhoi(t *testing.T) {
	bag := NewBag(rand.New(rand.NewSource(3)))

	preview := bag.Peek(5)
	if len(preview) != 5 {
		t.Fatalf("Peek(5) trả %d khối, muốn 5", len(preview))
	}

	for i, want := range preview {
		if got := bag.Next().Type; got != want.Type {
			t.Errorf("khối thứ %d: Next() = %v, nhưng Peek() báo trước là %v", i, got, want.Type)
		}
	}
}

func TestBag_PeekVuotQuaMotTui(t *testing.T) {
	bag := NewBag(rand.New(rand.NewSource(5)))

	// Xem trước 10 khối, tức là vượt qua ranh giới túi 7 khối.
	preview := bag.Peek(10)
	if len(preview) != 10 {
		t.Fatalf("Peek(10) trả %d khối, muốn 10", len(preview))
	}

	for i, want := range preview {
		if got := bag.Next().Type; got != want.Type {
			t.Errorf("khối thứ %d: Next() = %v, Peek() báo %v", i, got, want.Type)
		}
	}
}

func TestBag_CungSeedChoCungKetQua(t *testing.T) {
	a := NewBag(rand.New(rand.NewSource(42)))
	b := NewBag(rand.New(rand.NewSource(42)))

	for i := 0; i < 20; i++ {
		if a.Next().Type != b.Next().Type {
			t.Fatalf("cùng seed nhưng dãy khối khác nhau ở vị trí %d", i)
		}
	}
}
