package output

import (
	"fmt"
	"strings"
)

// Pretty prints styled terminal output using lipgloss.
type Pretty struct{}

func NewPretty() *Pretty { return &Pretty{} }

func (p *Pretty) Section(title string, meta ...string) {
	line := HeaderStyle.Render(title)
	for _, m := range meta {
		line += DimStyle.Render("  │  ") + m
	}
	fmt.Println(line)
}

// SubHeader prints a dim line beneath a section header (the version-control
// identity line), indented to sit under the section title.
func (p *Pretty) SubHeader(text string) {
	fmt.Println(DimStyle.Render("  ") + text)
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

func (p *Pretty) CheckRow(icon, label string, labelWidth int, detail string) {
	if detail == "" {
		fmt.Printf("%s %s\n", icon, label)
		return
	}
	fmt.Printf("%s %s  %s\n", icon, padRight(label, labelWidth), DimStyle.Render(detail))
}

// subRowIndent is the fixed indent of an expanded check's item rows: enough to sit
// them under the check label (past the "icon " spine) and read as a group, but not
// aligned to the variable, often-wide label column, so the rows keep the full
// terminal width for their content.
const subRowIndent = 2

// ItemRows prints a check's expanded item rows beneath its summary line, indented
// by subRowIndent. When the rows use the blank-first-cell convention (a leading
// column repeated across consecutive rows, blanked on repeat - as the lint check
// does per file), it switches to grouped layout: the shared key gets its own line
// and its rows nest under it, so a wide key column (a file path) no longer pushes
// every message to a fixed far-right gutter. Otherwise the rows render flat,
// column-aligned.
func (p *Pretty) ItemRows(rows [][]string) {
	if len(rows) == 0 {
		return
	}
	base := strings.Repeat(" ", subRowIndent)

	if isGrouped(rows) {
		p.groupedRows(rows, base)
		return
	}
	widths := colWidths(rows, 0)
	for _, r := range rows {
		fmt.Println(base + renderCells(r, widths))
	}
}

// isGrouped reports whether the rows use the blank-first-cell convention: a
// multi-column block where a later row leaves its first cell empty to continue the
// previous row's group (the lint check blanks the file path on repeats). Flat
// blocks, where every row carries a distinct first cell, are left alone.
func isGrouped(rows [][]string) bool {
	for i, r := range rows {
		if i > 0 && len(r) > 1 && r[0] == "" {
			return true
		}
	}
	return false
}

// groupedRows renders the blank-first-cell convention as nested groups: a row with
// a non-empty first cell starts a group (its key on its own line at base indent),
// and every row's remaining cells render as a member one level deeper, aligned
// across the whole block.
func (p *Pretty) groupedRows(rows [][]string, base string) {
	member := base + strings.Repeat(" ", subRowIndent)
	widths := colWidths(rows, 1)
	for _, r := range rows {
		if len(r) > 0 && r[0] != "" {
			fmt.Println(base + r[0])
		}
		if len(r) > 1 {
			fmt.Println(member + renderCells(r[1:], widths))
		}
	}
}

// colWidths returns the display width of each column across rows, considering only
// cells from index `from` onward (so grouped layout can measure the member columns
// while ignoring the group-key column).
func colWidths(rows [][]string, from int) []int {
	cols := 0
	for _, r := range rows {
		if n := len(r) - from; n > cols {
			cols = n
		}
	}
	if cols < 0 {
		cols = 0
	}
	widths := make([]int, cols)
	for _, r := range rows {
		for i := from; i < len(r); i++ {
			if w := i - from; len(r[i]) > widths[w] {
				widths[w] = len(r[i])
			}
		}
	}
	return widths
}

// renderCells lays out one item row: every cell but the last is padded to its
// column width, the first cell is plain and the rest dim. The final cell is
// emitted in full and never truncated, so an over-long message (a golangci
// diagnostic, say) is left for the terminal to soft-wrap rather than being clipped
// at a fixed column the user cannot widen past.
func renderCells(cells []string, widths []int) string {
	var b strings.Builder
	for i, c := range cells {
		if i > 0 {
			b.WriteString("  ")
		}
		cell := c
		if i < len(cells)-1 {
			cell = padRight(c, widths[i])
		}
		if i == 0 {
			b.WriteString(cell)
		} else {
			b.WriteString(DimStyle.Render(cell))
		}
	}
	return b.String()
}

func (p *Pretty) Newline() {
	fmt.Println()
}
