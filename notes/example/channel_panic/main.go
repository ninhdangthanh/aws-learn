// Package main - Ví dụ về PANIC khi thao tác sai với channel trong Go
// và cách dùng recover() để "bắt" panic, tránh crash cả chương trình.
//
// ============================================================================
// KIẾN THỨC NỀN TẢNG
// ============================================================================
//
// Channel trong Go có 3 trạng thái quan trọng: nil, open (đang mở), closed (đã đóng).
// Bảng dưới đây tóm tắt hành vi của từng thao tác theo trạng thái channel:
//
//   Thao tác        | nil channel      | open channel        | closed channel
//   ----------------|------------------|---------------------|---------------------------
//   Gửi (ch <- x)   | block mãi mãi    | gửi bình thường     | PANIC ⚠️
//   Nhận (<-ch)     | block mãi mãi    | nhận bình thường    | trả về zero-value ngay (ok=false)
//   close(ch)       | PANIC ⚠️         | đóng bình thường    | PANIC ⚠️
//
// => Có 3 lỗi runtime kinh điển gây PANIC với channel:
//   1. Gửi message vào channel ĐÃ ĐÓNG          -> "send on closed channel"
//   2. Đóng một channel ĐÃ ĐÓNG (close 2 lần)    -> "close of closed channel"
//   3. Đóng một channel nil                       -> "close of nil channel"
//
// Panic KHÔNG thể tránh bằng cách "kiểm tra trước khi gửi", vì không có hàm
// nào cho biết channel đã đóng hay chưa khi GỬI. Cách xử lý đúng là:
//   - Thiết kế lại luồng (chỉ 1 nơi được quyền close, dùng sync.Once, done channel...)
//   - Hoặc dùng recover() ở tầng ngoài để chương trình không sập.
//
// ============================================================================
// RECOVER HOẠT ĐỘNG NHƯ THẾ NÀO?
// ============================================================================
//
//   - recover() CHỈ có tác dụng khi được gọi BÊN TRONG một hàm defer.
//   - Khi panic xảy ra, Go dừng thực thi bình thường và bắt đầu "unwind" stack,
//     chạy lần lượt các hàm defer. Nếu một defer gọi recover(), panic được
//     "dập tắt" và chương trình tiếp tục chạy từ SAU hàm chứa defer đó.
//   - recover() trả về giá trị được truyền vào panic (thường là error). Nếu
//     không có panic, recover() trả về nil.
//
// Chạy thử:  go run ./example/channel_panic
// Từ trong thư mục này:  go run .
package main

import (
	"errors"
	"fmt"
	"sync"
)

func main() {
	fmt.Println("=== VÍ DỤ 1: Gửi message vào channel đã đóng (KHÔNG recover -> crash) ===")
	// Dòng dưới bị comment vì nó sẽ làm SẬP toàn bộ chương trình.
	// Bỏ comment để tự trải nghiệm panic "send on closed channel".
	// sendOnClosedChannelNoRecover()

	fmt.Println("\n=== VÍ DỤ 2: Gửi message vào channel đã đóng (CÓ recover) ===")
	if err := sendOnClosedChannel(); err != nil {
		fmt.Println("  [recovered] Đã bắt được panic:", err)
	}

	fmt.Println("\n=== VÍ DỤ 3: Đóng một channel đã đóng (CÓ recover) ===")
	if err := closeClosedChannel(); err != nil {
		fmt.Println("  [recovered] Đã bắt được panic:", err)
	}

	fmt.Println("\n=== VÍ DỤ 4: Đóng channel nil (CÓ recover) ===")
	if err := closeNilChannel(); err != nil {
		fmt.Println("  [recovered] Đã bắt được panic:", err)
	}

	fmt.Println("\n=== VÍ DỤ 5: CÁCH LÀM ĐÚNG - dùng sync.Once để chỉ close 1 lần ===")
	safeCloseWithOnce()

	fmt.Println("\n=== VÍ DỤ 6: CÁCH LÀM ĐÚNG - chỉ producer được quyền close ===")
	correctProducerConsumer()

	fmt.Println("\n>>> Chương trình chạy tới cuối main mà KHÔNG bị crash nhờ recover + thiết kế đúng.")
}

// ----------------------------------------------------------------------------
// VÍ DỤ 1 (tham khảo): gửi vào channel đã đóng mà KHÔNG recover.
// Hàm này sẽ làm crash chương trình với thông báo:
//   panic: send on closed channel
// Chỉ để minh hoạ, mặc định không được gọi trong main.
// ----------------------------------------------------------------------------
func sendOnClosedChannelNoRecover() {
	ch := make(chan int, 1)
	close(ch)
	ch <- 1 // PANIC: send on closed channel -> không có recover nên chương trình sập
}

// ----------------------------------------------------------------------------
// VÍ DỤ 2: gửi vào channel đã đóng NHƯNG có recover.
// Kỹ thuật: bọc thao tác nguy hiểm trong một hàm có defer + recover.
// recover() được đặt trong defer nên khi panic xảy ra ở "ch <- 1", panic sẽ bị
// bắt lại, biến thành error trả về thay vì làm sập chương trình.
// (err là named return value để defer có thể gán giá trị cho nó.)
// ----------------------------------------------------------------------------
func sendOnClosedChannel() (err error) {
	defer func() {
		// r nhận giá trị được truyền vào panic (ở đây runtime panic là 1 error).
		if r := recover(); r != nil {
			err = fmt.Errorf("panic khi gửi vào channel đã đóng: %v", r)
		}
	}()

	ch := make(chan int, 1)
	close(ch)
	ch <- 1 // <-- PANIC "send on closed channel" phát sinh tại đây

	// Dòng dưới KHÔNG BAO GIỜ chạy tới vì panic đã nhảy thẳng vào defer ở trên.
	fmt.Println("  dòng này không bao giờ được in ra")
	return nil
}

// ----------------------------------------------------------------------------
// VÍ DỤ 3: đóng một channel đã đóng (close 2 lần) -> "close of closed channel".
// ----------------------------------------------------------------------------
func closeClosedChannel() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic khi đóng channel đã đóng: %v", r)
		}
	}()

	ch := make(chan int)
	close(ch)
	close(ch) // <-- PANIC "close of closed channel"

	return nil
}

// ----------------------------------------------------------------------------
// VÍ DỤ 4: đóng một channel nil -> "close of nil channel".
// ----------------------------------------------------------------------------
func closeNilChannel() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic khi đóng channel nil: %v", r)
		}
	}()

	var ch chan int // channel chưa make -> giá trị zero là nil
	close(ch)       // <-- PANIC "close of nil channel"

	return nil
}

// ----------------------------------------------------------------------------
// VÍ DỤ 5: CÁCH LÀM ĐÚNG #1 — dùng sync.Once đảm bảo close CHỈ chạy 1 lần.
// Dù gọi safeClose() bao nhiêu lần, close(ch) thực tế chỉ được thực thi đúng 1
// lần -> không bao giờ panic "close of closed channel".
// Đây là pattern hữu dụng khi nhiều goroutine đều có thể muốn đóng channel.
// ----------------------------------------------------------------------------
func safeCloseWithOnce() {
	ch := make(chan int, 3)
	var once sync.Once

	safeClose := func() {
		once.Do(func() {
			close(ch)
			fmt.Println("  channel được đóng (chỉ 1 lần duy nhất)")
		})
	}

	ch <- 10
	ch <- 20

	// Gọi close nhiều lần một cách "an toàn" — lần 2, 3 không làm gì cả.
	safeClose()
	safeClose()
	safeClose()

	// Sau khi đóng vẫn NHẬN được các giá trị còn trong buffer, rồi tới zero-value.
	for v := range ch {
		fmt.Println("  nhận được:", v)
	}
}

// ----------------------------------------------------------------------------
// VÍ DỤ 6: CÁCH LÀM ĐÚNG #2 — nguyên tắc vàng của channel:
//   "Chỉ PRODUCER (bên gửi) mới được quyền close channel, và close SAU KHI
//    đã gửi xong. Consumer (bên nhận) KHÔNG BAO GIỜ close."
// Nhờ vậy sẽ không có ai gửi vào channel đã đóng, và không ai close 2 lần.
// Consumer dùng `for range` để nhận cho tới khi channel đóng thì tự thoát.
// ----------------------------------------------------------------------------
func correctProducerConsumer() {
	ch := make(chan int)
	var wg sync.WaitGroup

	// Producer: gửi dữ liệu rồi tự đóng channel khi xong.
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(ch) // producer đóng channel để báo hiệu "hết dữ liệu"
		for i := 1; i <= 3; i++ {
			ch <- i * 100
		}
	}()

	// Consumer: nhận cho tới khi channel đóng thì vòng lặp tự kết thúc.
	// `v, ok := <-ch` với ok=false nghĩa là channel đã đóng và hết dữ liệu.
	for v := range ch {
		fmt.Println("  consumer nhận:", v)
	}

	wg.Wait()
	fmt.Println("  producer đã đóng channel an toàn, consumer thoát sạch sẽ")
}

// Biến demo minh hoạ recover cũng bắt được panic do người dùng chủ động tạo.
// (Không gọi trong main, chỉ để tham khảo cách recover trả về đúng error gốc.)
var errCustom = errors.New("lỗi tự định nghĩa")

func panicWithError() (err error) {
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(error); ok {
				err = e // lấy lại đúng error đã panic
			} else {
				err = fmt.Errorf("panic không phải error: %v", r)
			}
		}
	}()
	panic(errCustom)
}
