// Command blockpuzzle-demo chạy một ván Block Puzzle do AI tự chơi,
// in board sau mỗi nước để thấy logic hoạt động.
//
//	go run ./cmd/blockpuzzle-demo
package main

import (
	"fmt"
	"math/rand"
	"time"

	"game-livecode/blockpuzzle"
)

func main() {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	board := blockpuzzle.NewBoard(9, 9)

	fmt.Println("=== BLOCK PUZZLE ===")
	fmt.Println("Board 9x9. AI tự chọn nước đi, xoá cả HÀNG và CỘT khi đầy.")
	fmt.Println("Lưu ý: không có gravity — ô còn lại đứng yên tại chỗ.")
	fmt.Println()

	score := 0
	for turn := 1; ; turn++ {
		// Mỗi lượt phát 3 block, giống game thật.
		hand := []blockpuzzle.Block{
			blockpuzzle.RandomBlock(rng),
			blockpuzzle.RandomBlock(rng),
			blockpuzzle.RandomBlock(rng),
		}

		if blockpuzzle.IsGameOver(board, hand) {
			fmt.Printf("GAME OVER ở lượt %d — không block nào đặt được.\n", turn)
			fmt.Printf("Tổng điểm: %d\n", score)
			return
		}

		for len(hand) > 0 {
			index, move, ok := pickBestFromHand(board, hand)
			if !ok {
				break // các block còn lại đều kẹt, chờ lượt sau
			}

			block := hand[index]
			blockpuzzle.PlaceBlock(&board, move.Block, move.At.X, move.At.Y)
			cleared := blockpuzzle.ClearFullRowsAndCols(&board)
			score += blockpuzzle.CalculateScore(block, cleared)

			fmt.Printf("Lượt %d — đặt %s tại (%d,%d)", turn, block.Name, move.At.X, move.At.Y)
			if cleared.Total() > 0 {
				fmt.Printf("  ->  xoá %d hàng, %d cột", cleared.Rows, cleared.Cols)
			}
			fmt.Printf("   [điểm: %d]\n", score)
			fmt.Println(board)

			hand = append(hand[:index], hand[index+1:]...)
		}

		if turn > 40 {
			fmt.Printf("Dừng sau 40 lượt. Tổng điểm: %d\n", score)
			return
		}
	}
}

// pickBestFromHand chọn block nào trong tay cho nước đi tốt nhất.
func pickBestFromHand(board blockpuzzle.Board, hand []blockpuzzle.Block) (int, blockpuzzle.Move, bool) {
	bestIndex := -1
	var best blockpuzzle.Move

	for i, block := range hand {
		move, ok := blockpuzzle.FindBestMove(board, block, true)
		if !ok {
			continue
		}
		if bestIndex == -1 || move.Score > best.Score {
			bestIndex, best = i, move
		}
	}
	return bestIndex, best, bestIndex != -1
}
