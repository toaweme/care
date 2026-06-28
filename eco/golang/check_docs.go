package golang

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/toaweme/care"
)

// DefaultDocCoverage is the doc-comment coverage below which the docs check warns when an
// operator has not configured their own `checks.docs.options.min`. To silence the check
// entirely, disable it (`checks.docs.disabled: true`) rather than setting a zero threshold.
const DefaultDocCoverage = 80

type docsCheck struct {
	care.BaseCheck
	minCoverage float64 // warn below this percent; <=0 falls back to DefaultDocCoverage
}

var _ care.Docs = (*docsCheck)(nil)

// NewDocs is the Docs feature for Go: it walks the source with go/ast and reports
// what fraction of exported declarations (funcs, methods, types, consts, vars) carry
// a doc comment, warning when coverage falls below minCoverage (or DefaultDocCoverage
// when minCoverage is unset).
func NewDocs(minCoverage float64) care.Docs {
	return &docsCheck{BaseCheck: care.NewBaseCheck("go-docs"), minCoverage: minCoverage}
}

func (f *docsCheck) Applies(dir string) bool { return hasGoMod(dir) }

func (f *docsCheck) Run(_ context.Context, dir string, _ care.RunOptions) care.Output[care.DocsReport] {
	report, err := docCoverage(dir)
	if err != nil {
		return care.Errored[care.DocsReport]("walk failed", fmt.Errorf("failed to scan exported symbols in %q: %w", dir, err))
	}
	if report.Total == 0 {
		return care.Pass(report)
	}
	minCov := f.minCoverage
	if minCov <= 0 {
		minCov = DefaultDocCoverage
	}
	pct := float64(report.Documented) / float64(report.Total) * 100
	if pct < minCov {
		return care.Warn(report)
	}
	return care.Pass(report)
}

// docCoverage walks the module's non-test, non-vendored Go files and tallies which
// exported declarations carry a doc comment.
func docCoverage(dir string) (care.DocsReport, error) {
	var report care.DocsReport
	fset := token.NewFileSet()
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDir(d.Name()) && path != dir {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return nil //nolint:nilerr // unparseable file is the build check's problem, not ours
		}
		rel, _ := filepath.Rel(dir, path)
		scanDecls(file, fset, rel, &report)
		return nil
	})
	return report, err
}

// scanDecls tallies the exported declarations in one file, appending the
// undocumented ones to the report.
func scanDecls(file *ast.File, fset *token.FileSet, rel string, report *care.DocsReport) {
	record := func(documented bool, kind, name string, pos token.Pos) {
		report.Total++
		if documented {
			report.Documented++
			return
		}
		report.Missing = append(report.Missing, care.DocSymbol{File: rel, Line: fset.Position(pos).Line, Kind: kind, Name: name})
	}
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if !d.Name.IsExported() {
				continue
			}
			kind, name := "func", d.Name.Name
			if d.Recv != nil {
				kind, name = "method", recvType(d.Recv)+"."+d.Name.Name
			}
			record(d.Doc != nil, kind, name, d.Pos())
		case *ast.GenDecl:
			scanGenDecl(d, record)
		}
	}
}

// scanGenDecl tallies the exported specs of a const/var/type declaration. A grouped
// declaration carries docs on each spec; a single one carries it on the GenDecl, so a
// spec counts as documented when either is present.
func scanGenDecl(d *ast.GenDecl, record func(documented bool, kind, name string, pos token.Pos)) {
	if d.Tok == token.IMPORT {
		return
	}
	for _, spec := range d.Specs {
		switch s := spec.(type) {
		case *ast.TypeSpec:
			if s.Name.IsExported() {
				record(s.Doc != nil || d.Doc != nil, "type", s.Name.Name, s.Pos())
			}
		case *ast.ValueSpec:
			kind := "var"
			if d.Tok == token.CONST {
				kind = "const"
			}
			for _, n := range s.Names {
				if n.IsExported() {
					record(s.Doc != nil || d.Doc != nil, kind, n.Name, n.Pos())
				}
			}
		}
	}
}

// recvType returns the receiver type name (without the pointer star) for a method.
func recvType(recv *ast.FieldList) string {
	if len(recv.List) == 0 {
		return ""
	}
	switch t := recv.List[0].Type.(type) {
	case *ast.StarExpr:
		if id, ok := t.X.(*ast.Ident); ok {
			return id.Name
		}
	case *ast.Ident:
		return t.Name
	}
	return ""
}

// skipDir reports whether a directory should be skipped during the source walk:
// hidden dirs, vendored deps and testdata fixtures.
func skipDir(name string) bool {
	return name == "vendor" || name == "testdata" || (strings.HasPrefix(name, ".") && name != ".")
}
