package output

import (
	"fmt"
	"io"
	"strings"
)

// PrintTable formats rows of string columns into aligned text
func PrintTable(w io.Writer, headers []string, rows [][]string) {
	if len(rows) == 0 {
		return
	}

	colWidths := make([]int, len(headers))
	for i, h := range headers {
		colWidths[i] = len(h)
	}

	for _, row := range rows {
		for i, cell := range row {
			if i < len(colWidths) && len(cell) > colWidths[i] {
				colWidths[i] = len(cell)
			}
		}
	}

	// Print headers
	for i, h := range headers {
		fmt.Fprintf(w, "%-*s  ", colWidths[i], h)
	}
	fmt.Fprintln(w)

	// Print separator
	for i := range headers {
		fmt.Fprintf(w, "%s  ", strings.Repeat("-", colWidths[i]))
	}
	fmt.Fprintln(w)

	// Print rows
	for _, row := range rows {
		for i, cell := range row {
			if i < len(colWidths) {
				fmt.Fprintf(w, "%-*s  ", colWidths[i], cell)
			}
		}
		fmt.Fprintln(w)
	}
}
