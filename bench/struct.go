package templates

import (
	"fmt"
)

func getBenchRows(n int) []BenchRow {
	rows := make([]BenchRow, n)
	for i := 0; i < n; i++ {
		rows[i] = BenchRow{
			ID:      i,
			Message: fmt.Sprintf("message %d", i),
			Print:   ((i & 1) == 0),
		}
	}
	return rows
}
