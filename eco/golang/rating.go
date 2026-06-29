package golang

import "github.com/toaweme/care"

// DefaultRating is care's published grading opinion for Go repos: security and a
// broken build dominate, style and docs are minor, benchmarks are informational
// (weight 0). The caps reflect severity, not dev state: a committed secret caps at F
// (a live exposure), a reachable vulnerability at C (real but often transitive); a
// broken build has no cap on purpose, so a transient non-compiling module in active
// development is only weighted, not graded as a hard failure. These are the values an
// operator's care.yml health block overlays via Rating.Overlay; a second ecosystem
// ships its own defaults for the same shared types.
func DefaultRating() care.Rating {
	return care.Rating{
		Weights: care.Weights{
			VersionControl:  5,
			Build:           20,
			Lint:            20,
			Dependencies:    8,
			Docs:            5,
			Tests:           15,
			Benchmarks:      0,
			Secrets:         20,
			Vulnerabilities: 20,
		},
		Caps: care.Caps{
			Secrets:         40,
			Vulnerabilities: 72,
		},
	}
}
