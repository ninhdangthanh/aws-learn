// Command tetris-demo chạy một ván Tetris do AI tự chơi.
//
//	go run ./cmd/tetris-demo          # chạy tới khi game over
//	go run ./cmd/tetris-demo -slow    # dừng 300ms mỗi nước để xem rõ
//
// Ký hiệu: '.' ô trống, ':' ghost piece, chữ cái là loại khối.
package main

import (
	"flag"
	"fmt"
	"math/rand"
	"time"

	"game-livecode/tetris"
)

func main() {
	slow := flag.Bool("slow", false, "dừng giữa các nước để xem cho rõ")
	flag.Parse()

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	game := tetris.NewGame(10, 16, rng)

	fmt.Println("=== TETRIS ===")
	fmt.Println("Board 10x16. AI tự chọn cột và góc xoay, rồi hard drop.")
	fmt.Println("Khác Block Puzzle: chỉ xoá HÀNG, và có GRAVITY khi xoá.")
	fmt.Println()

	for piece := 1; !game.Over; piece++ {
		best, ok := tetris.FindBestPlacement(game.Board, game.Current.Shape)
		if !ok {
			break
		}

		// AI trả về vị trí đích, ta mô phỏng lại bằng thao tác của người
		// chơi thật: xoay cho đúng góc, dịch ngang, rồi hard drop.
		for game.Current.Rotation != best.Rotation {
			if !game.RotateCW() {
				break
			}
		}
		for game.Current.Position.X > best.Block.Position.X && game.MoveLeft() {
		}
		for game.Current.Position.X < best.Block.Position.X && game.MoveRight() {
		}

		shape := game.Current.Shape.Type
		cleared := game.HardDrop()

		fmt.Printf("Khối %d (%s)", piece, shape)
		if cleared > 0 {
			label := map[int]string{1: "single", 2: "double", 3: "triple", 4: "TETRIS!"}[cleared]
			fmt.Printf("  ->  xoá %d hàng (%s)", cleared, label)
		}
		fmt.Printf("   [điểm: %d | hàng: %d | level: %d]\n", game.Score, game.Lines, game.Level)
		fmt.Println(game.Board.Render(game.Current, true))

		if *slow {
			time.Sleep(300 * time.Millisecond)
		}
		if piece > 200 {
			break
		}
	}

	fmt.Printf("KẾT THÚC — điểm: %d, xoá được %d hàng, level %d\n",
		game.Score, game.Lines, game.Level)
}
