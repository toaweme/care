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

func (p *Pretty) FlatBlock(icon, label string, labelWidth int, rows [][]string, maxWidth int) {
	if len(rows) == 0 {
		return
	}
	// the spine is icon + space + padded label + two spaces; its visual width is
	// where every item row's content begins, so continuation rows indent to match.
	contentCol := labelWidth + 4
	spine := fmt.Sprintf("%s %s  ", icon, padRight(label, labelWidth))
	blank := strings.Repeat(" ", contentCol)

	cols := 0
	for _, r := range rows {
		if len(r) > cols {
			cols = len(r)
		}
	}
	widths := make([]int, cols)
	for _, r := range rows {
		for i, c := range r {
			if i < len(widths) && len(c) > widths[i] {
				widths[i] = len(c)
			}
		}
	}

	for ri, r := range rows {
		prefix := blank
		if ri == 0 {
			prefix = spine
		}
		fmt.Println(prefix + renderCells(r, widths, contentCol, maxWidth))
	}
}

// renderCells lays out one item row: every cell but the last is padded to its
// column width, the first cell is plain and the rest dim, and the final cell is
// truncated so the whole line fits within maxWidth columns.
func renderCells(cells []string, widths []int, startCol, maxWidth int) string {
	var b strings.Builder
	col := startCol
	for i, c := range cells {
		if i > 0 {
			b.WriteString("  ")
			col += 2
		}
		cell := c
		last := i == len(cells)-1
		if !last {
			cell = padRight(c, widths[i])
		} else if maxWidth > 0 {
			if avail := maxWidth - col; avail > 1 && len(cell) > avail {
				cell = truncate(cell, avail)
			}
		}
		if i == 0 {
			b.WriteString(cell)
		} else {
			b.WriteString(DimStyle.Render(cell))
		}
		col += len(cell)
	}
	return b.String()
}

// truncate shortens s to width display columns, replacing the tail with an
// ellipsis. It assumes single-width runes, which holds for the ASCII paths,
// file names and identifiers in the report.
func truncate(s string, width int) string {
	if width <= 1 || len(s) <= width {
		return s
	}
	r := []rune(s)
	if width-1 > len(r) {
		return s
	}
	return string(r[:width-1]) + "…"
}

func (p *Pretty) Newline() {
	fmt.Println()
}
