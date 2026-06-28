// Package minver computes the lowest Go language version a module's own code can
// declare in its `go` directive: the oldest Go that still compiles it. It mirrors
// the approach of github.com/bobg/mingo without vendoring it: a static go/ast +
// go/types pass detects language features (generics, range-over-int, etc.) and
// stdlib-symbol usage, mapping each to the Go minor that introduced it; the running
// maximum is the answer.
//
// The stdlib version data is not embedded. It is read at run time from the Go
// installation's own $GOROOT/api/go1.N.txt files (the data `go tool api` uses for
// the Go 1 compatibility audit). When that directory is absent (some stripped Go
// images delete it), LoadHistory returns ErrNoAPI so the caller can skip the
// stdlib-symbol part rather than guess.
package minver

import "errors"

// ErrNoAPI is returned by LoadHistory when $GOROOT/api is unavailable. Callers
// should treat it as "skip this check", not as a failure: the data is a Go
// installation artifact care neither ships nor can synthesize.
var ErrNoAPI = errors.New("$GOROOT/api not found")

// Result is the computed minimum for a module: the highest minor version any
// single construct in the code forces, with the reasons that forced it.
type Result struct {
	// Min is the lowest Go minor version (the N in go1.N) the code can declare.
	// 0 means "nothing newer than go1.0 was detected" (any go directive is fine).
	Min int
	// Reasons are the constructs at the deciding version (Min), each naming what
	// it is and where, so a report can explain why the floor is where it is.
	Reasons []Reason
}

// Reason records one construct that requires a given Go minor version: a language
// feature ("range over integer") or a stdlib symbol (`"slices".Sort`), with its
// source position when known.
type Reason struct {
	Minor int    // the go1.N this construct needs
	Desc  string // human-readable name of the construct
	Pos   string // file:line:col, best effort (empty when unknown)
}
