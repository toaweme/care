package changelog

import "testing"

const fixtureChangelog = `# Changelog

All notable changes to this project are documented here.

## [1.2.0] - 2026-06-01

Some hand-written intro about this release.

### Features

- big new thing (@alice)

## [1.1.0] - 2026-05-01

Only prose here, no generated groups yet.

## [1.0.0] - 2026-04-01

### Features

- first release (@bob)
`

func Test_ParseDocument(t *testing.T) {
	doc := ParseDocument(fixtureChangelog)

	if len(doc.Versions) != 3 {
		t.Fatalf("got %d versions, want 3", len(doc.Versions))
	}
	if doc.Header == "" || doc.Versions[0].Semver != "1.2.0" {
		t.Fatalf("header/version order wrong: header=%q first=%q", doc.Header, doc.Versions[0].Semver)
	}

	v120 := doc.Versions[0]
	if v120.Date != "2026-06-01" {
		t.Errorf("date = %q, want 2026-06-01", v120.Date)
	}
	if !v120.HasGroups {
		t.Errorf("1.2.0 should report HasGroups")
	}
	if v120.Prose != "Some hand-written intro about this release." {
		t.Errorf("prose = %q", v120.Prose)
	}

	v110 := doc.Versions[1]
	if v110.HasGroups {
		t.Errorf("1.1.0 should not report HasGroups (prose only)")
	}

	if _, ok := doc.Find("1.0.0"); !ok {
		t.Errorf("Find(1.0.0) should succeed")
	}
	if doc.Has("9.9.9") {
		t.Errorf("Has(9.9.9) should be false")
	}
}
