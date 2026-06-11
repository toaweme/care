package output

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// capture runs fn with stdout redirected and returns what it printed, stripped
// of ANSI styling so assertions read against the plain layout.
func capture(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = orig
	buf := make([]byte, 64*1024)
	n, _ := r.Read(buf)
	return ansiRE.ReplaceAllString(string(buf[:n]), "")
}

// runeCol returns the display column (rune offset) where sub first appears in s,
// or -1. Rune-based so multibyte glyphs like the status icon count as one column.
func runeCol(s, sub string) int {
	i := strings.Index(s, sub)
	if i < 0 {
		return -1
	}
	return len([]rune(s[:i]))
}

func Test_FlatBlock_Alignment(t *testing.T) {
	rows := [][]string{
		{"root", "PASS", "37.2%"},
		{"./internal/devops/git", "PASS", "14.8%"},
		{"./templates", "SKIP"},
	}
	got := capture(t, func() {
		NewPretty().ItemRows(rows)
	})
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 rows, got %d: %q", len(lines), got)
	}
	// item rows are indented a small fixed amount (not aligned to a wide label column).
	if got := runeCol(lines[0], "root"); got != subRowIndent {
		t.Errorf("row key column = %d, want %d (%q)", got, subRowIndent, lines[0])
	}
	if got := runeCol(lines[1], "./internal"); got != subRowIndent {
		t.Errorf("row key column = %d, want %d (%q)", got, subRowIndent, lines[1])
	}
	// the value column aligns across rows: PASS sits past the widest key.
	if a, b := runeCol(lines[0], "PASS"), runeCol(lines[1], "PASS"); a != b {
		t.Errorf("PASS column misaligned: row0=%d row1=%d", a, b)
	}
}

// rows using the blank-first-cell convention (lint, per file) render grouped: the
// shared key on its own line, its rows nested beneath, so a wide key column does
// not push messages to a far-right gutter.
func Test_FlatBlock_GroupsBlankFirstCell(t *testing.T) {
	rows := [][]string{
		{"a/very/long/path/file_one.go", "1:1", "missing comment"},
		{"", "19:1", "exported func"},
		{"short.go", "3:4", "unused var"},
	}
	got := capture(t, func() {
		NewPretty().ItemRows(rows)
	})
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	// file1 + 2 members + file2 + 1 member = 5 lines (no header; that is the
	// caller's summary line).
	if len(lines) != 5 {
		t.Fatalf("expected 5 lines, got %d: %q", len(lines), got)
	}
	// file keys sit at base indent; members one level deeper.
	if got := runeCol(lines[0], "a/very"); got != subRowIndent {
		t.Errorf("file key column = %d, want %d (%q)", got, subRowIndent, lines[0])
	}
	if got := runeCol(lines[1], "1:1"); got != subRowIndent*2 {
		t.Errorf("member column = %d, want %d (%q)", got, subRowIndent*2, lines[1])
	}
	// the wide path no longer dictates where messages start: the second file is a
	// header, and its member's message sits at the same shallow member column.
	if lines[3] != "  short.go" {
		t.Errorf("second file header = %q, want %q", lines[3], "  short.go")
	}
	if a, b := runeCol(lines[1], "missing"), runeCol(lines[4], "unused"); a != b {
		t.Errorf("message column misaligned across groups: %d vs %d", a, b)
	}
}

// the final cell is never clipped: a long message is emitted in full so the
// terminal can soft-wrap it, instead of being cut at a fixed column the user
// cannot widen past by resizing.
func Test_FlatBlock_NeverTruncatesLastCell(t *testing.T) {
	summary := "Incorrect enforcement of email constraints in crypto/x509 long enough to overflow any sensible terminal width and then some more"
	rows := [][]string{{"GO-2026-4599", "crypto/x509", "v1.26.0 -> v1.26.1", summary}}
	got := capture(t, func() {
		NewPretty().ItemRows(rows)
	})
	if !strings.Contains(got, summary) {
		t.Errorf("expected the full message to be emitted, got %q", got)
	}
	if strings.Contains(got, "…") {
		t.Errorf("expected no ellipsis (no truncation), got %q", got)
	}
}
