package tetris

import "strings"

// ascii chuẩn hoá board về dạng chỉ có '.' và 'X'.
//
// Board.String() in ra LOẠI khối ở mỗi ô ("I", "T", "O"...) để debug cho
// tiện, nhưng phần lớn test chỉ quan tâm ô nào trống, ô nào đã chiếm.
// Helper này giúp bảng test viết bằng 'X' cho dễ đọc.
func ascii(b Board) string {
	var sb strings.Builder
	for _, ch := range b.String() {
		switch ch {
		case '.', '\n':
			sb.WriteRune(ch)
		default:
			sb.WriteByte('X')
		}
	}
	return sb.String()
}

// countOccupied đếm số ô đã bị chiếm trên board.
func countOccupied(b Board) int {
	count := 0
	for y := 0; y < b.Height; y++ {
		for x := 0; x < b.Width; x++ {
			if b.Cells[y][x] != Empty {
				count++
			}
		}
	}
	return count
}
