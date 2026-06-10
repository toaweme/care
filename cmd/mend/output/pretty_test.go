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
		NewPretty().FlatBlock("✓", "check.test", 14, rows, 0)
	})
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %q", len(lines), got)
	}
	// the content column starts at labelWidth+4; the first row sits on the spine,
	// the rest align beneath it, so the item key starts at the same column.
	const contentCol = 14 + 4
	if got := runeCol(lines[0], "root"); got != contentCol {
		t.Errorf("row 0 key column = %d, want %d (%q)", got, contentCol, lines[0])
	}
	if got := runeCol(lines[1], "./internal"); got != contentCol {
		t.Errorf("row 1 key column = %d, want %d (%q)", got, contentCol, lines[1])
	}
	// the value column aligns across rows: PASS sits past the widest key.
	if a, b := runeCol(lines[0], "PASS"), runeCol(lines[1], "PASS"); a != b {
		t.Errorf("PASS column misaligned: row0=%d row1=%d", a, b)
	}
}

func Test_FlatBlock_TruncatesLastCell(t *testing.T) {
	rows := [][]string{
		{"GO-2026-4599", "crypto/x509", "v1.26.0 -> v1.26.1", "Incorrect enforcement of email constraints in crypto/x509"},
	}
	got := capture(t, func() {
		NewPretty().FlatBlock("✗", "sec.vuln", 14, rows, 70)
	})
	line := strings.TrimRight(got, "\n")
	if w := len([]rune(line)); w > 70 {
		t.Errorf("line width = %d, want <= 70: %q", w, line)
	}
	if !strings.HasSuffix(line, "…") {
		t.Errorf("expected truncated line to end with ellipsis: %q", line)
	}
}

func Test_FlatBlock_NoTruncateWhenWidthZero(t *testing.T) {
	summary := "Incorrect enforcement of email constraints in crypto/x509"
	rows := [][]string{{"GO-2026-4599", "crypto/x509", "v1.26.0 -> v1.26.1", summary}}
	got := capture(t, func() {
		NewPretty().FlatBlock("✗", "sec.vuln", 14, rows, 0)
	})
	if !strings.Contains(got, summary) {
		t.Errorf("expected full summary when maxWidth is 0, got %q", got)
	}
}
