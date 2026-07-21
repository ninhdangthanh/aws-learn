// block puzzle

package main

type Point struct {
	X int
	Y int
}

type Board struct {
	Width  int
	Height int
	Cells  [][]bool // y,x
}

type Block struct {
	Name   string
	points []Point
}

func isOccupiedCell(board *Board, x, y int) bool {
	if x < 0 || y < 0 || x >= board.Width || y >= board.Height {
		return true
	}

	return board.Cells[y][x]
}

func canPlaceAt(board *Board, block Block, x, y int) bool {
	for _, b := range block.points {
		if isOccupiedCell(board, b.X+x, b.Y+y) {
			return false
		}
	}

	return true
}

func placeBlock(board *Board, block Block, x, y int) bool {
	if !canPlaceAt(board, block, x, y) {
		return false
	}

	for _, b := range block.points {
		board.Cells[b.Y+y][b.X+x] = true
	}

	return true
}
