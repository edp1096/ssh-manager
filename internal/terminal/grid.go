package terminal

import "fmt"

// PlanGrid places input hosts in row-major order. Row anchors must be created
// first; returned arguments are in actual launch order (also used for failures).
func PlanGrid(args []SshClientArgument, columns int) ([]SshClientArgument, error) {
	if len(args) < 1 || len(args) > 32 || columns < 1 || columns > 32 {
		return nil, fmt.Errorf("grid requires 1–32 hosts and 1–32 columns")
	}
	rows := (len(args) + columns - 1) / columns
	result := make([]SshClientArgument, 0, len(args))
	for row := 0; row < rows; row++ {
		a := args[row*columns]
		a.GridColumns = columns
		a.GridTarget = row
		a.SplitVertical = true
		a.SplitSize = float64(rows-row) / float64(rows-row+1)
		result = append(result, a)
	}
	for row := 0; row < rows; row++ {
		count := min(columns, len(args)-row*columns)
		target := row + 1
		for col := 1; col < count; col++ {
			a := args[row*columns+col]
			a.GridColumns = columns
			a.GridTarget = target
			a.SplitVertical = false
			a.SplitSize = float64(count-col) / float64(count-col+1)
			result = append(result, a)
			target = len(result)
		}
	}
	return result, nil
}
