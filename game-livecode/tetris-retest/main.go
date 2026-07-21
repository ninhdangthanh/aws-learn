package tetristest

import (
	"math/rand"
	"time"
)

type Point struct {
	X int
	Y int
}

type Board struct {
	Width  int
	Height int
	Cells  [][]bool
}

func newBoard(w, h int) *Board {
	c := make([][]bool, h)
	for i := range c {
		c[i] = make([]bool, w)
	}
	return &Board{Width: w, Height: h, Cells: c}
}

type Block struct {
	points []Point
}

type BlockPosition struct {
	Block Block
	Left  int
	Top   int
}

type Game struct {
	currentBl   *BlockPosition
	board       *Board
	elapsedTime int
}

var blocks = []Block{
	// I
	{
		points: []Point{
			{0, 0},
			{1, 0},
			{2, 0},
			{3, 0},
		},
	},

	// O
	{
		points: []Point{
			{0, 0},
			{1, 0},
			{0, 1},
			{1, 1},
		},
	},

	// T
	{
		points: []Point{
			{1, 0},
			{0, 1},
			{1, 1},
			{2, 1},
		},
	},

	// L
	{
		points: []Point{
			{0, 0},
			{0, 1},
			{0, 2},
			{1, 2},
		},
	},

	// J
	{
		points: []Point{
			{1, 0},
			{1, 1},
			{1, 2},
			{0, 2},
		},
	},

	// S
	{
		points: []Point{
			{1, 0},
			{2, 0},
			{0, 1},
			{1, 1},
		},
	},

	// Z
	{
		points: []Point{
			{0, 0},
			{1, 0},
			{1, 1},
			{2, 1},
		},
	},
}

func main() {
	game := &Game{
		currentBl:   nil,
		board:       newBoard(10, 30),
		elapsedTime: 0,
	}

	for {
		time.Sleep(1 * time.Second)
		game.elapsedTime += 1

		if game.currentBl == nil {
			game.currentBl = NewBlock(game.board.Width)
			if !CanPlace(game) {
				//end game
				return
			}
		}

		if CanMoveDown(game) {
			game.currentBl.Top += 1
		} else {
			// Merge Block to Board cells
			MergeBlock(game)
			// remove block
			game.currentBl = nil
			// clear lines
			ClearLines(game)
		}

	}
}

func ClearLines(game *Game) {
	for i := game.board.Height - 1; i >= 0; i-- {
		full := true
		for _, x := range game.board.Cells[i] {
			if !x {
				full = false
			}
		}

		if full {
			// move down above lines
			ClearLine(game, i)
			i++
		}
	}
}

func ClearLine(game *Game, line int) {
	for i := line; i >= 1; i-- {
		game.board.Cells[i] = game.board.Cells[i-1]
	}
	game.board.Cells[0] = make([]bool, game.board.Width)
}

func MergeBlock(game *Game) {
	position := game.currentBl

	for _, bl := range position.Block.points {
		game.board.Cells[bl.Y+game.currentBl.Top][bl.X+game.currentBl.Left] = true
	}
}

func CanMoveDown(game *Game) bool {
	position := game.currentBl
	for _, b := range position.Block.points {
		if isOccupiedCell(game.board, b.X+position.Left, b.Y+position.Top+1) {
			return false
		}
	}

	return true
}

func CanPlace(game *Game) bool {
	position := game.currentBl
	for _, b := range position.Block.points {
		if isOccupiedCell(game.board, b.X+position.Left, b.Y) {
			return false
		}
	}

	return true
}

func isOccupiedCell(board *Board, x, y int) bool {
	if x < 0 || y < 0 || x >= board.Width || y >= board.Height {
		return true
	}

	return board.Cells[y][x]
}

func NewBlock(boardWidth int) *BlockPosition {
	var rng = rand.New(rand.NewSource(time.Now().UnixNano()))

	b := blocks[rng.Intn(len(blocks))]

	return &BlockPosition{
		Block: b,
		Left:  boardWidth/2 - 2,
		Top:   0,
	}
}
