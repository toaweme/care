package golang

import (
	"bufio"
	"bytes"
	"strconv"
	"strings"
)

// diagnostic is one tool diagnostic line shared by the build and vet parsers: a
// location and a message. Both `go build` and `go vet` emit the canonical
// `file:line:col: message` form on stderr.
type diagnostic struct {
	File    string
	Line    int
	Col     int
	Message string
}

// parseDiagnostics distills `go build` / `go vet` stderr into one diagnostic per
// located line. It skips the `# package` headers and the indented continuation lines
// the compiler emits after an error (e.g. "have ()" / "want (int)"), keeping only the
// lines that carry a file location.
func parseDiagnostics(out []byte) []diagnostic {
	var diags []diagnostic
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		raw := sc.Text()
		// continuation lines are indented under their error; the package header
		// starts with '#'. Neither carries a location.
		if raw == "" || raw[0] == '#' || raw[0] == '\t' || raw[0] == ' ' {
			continue
		}
		if d, ok := parseDiagLine(raw); ok {
			diags = append(diags, d)
		}
	}
	return diags
}

// parseDiagLine parses a single "file.go:line[:col]: message" diagnostic, returning
// false for any line that does not carry a .go file location.
func parseDiagLine(line string) (diagnostic, bool) {
	gi := strings.Index(line, ".go:")
	if gi < 0 {
		return diagnostic{}, false
	}
	file := line[:gi+len(".go")]
	rest := line[gi+len(".go")+1:] // past "file.go:"
	// rest is "line[:col]: message"
	msgAt := strings.Index(rest, ": ")
	if msgAt < 0 {
		return diagnostic{}, false
	}
	loc, msg := rest[:msgAt], strings.TrimSpace(rest[msgAt+2:])
	d := diagnostic{File: file, Message: msg}
	lineStr, colStr, hasCol := strings.Cut(loc, ":")
	d.Line, _ = strconv.Atoi(lineStr)
	if hasCol {
		d.Col, _ = strconv.Atoi(colStr)
	}
	if d.Line == 0 {
		return diagnostic{}, false
	}
	return d, true
}
