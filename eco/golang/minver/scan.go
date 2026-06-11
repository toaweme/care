package minver

import (
	"context"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/packages"
)

// loadMode is the package information the analyzer needs: syntax trees plus full
// type info (to resolve which symbol or language feature a node uses) and the
// module the packages belong to.
const loadMode = packages.NeedName |
	packages.NeedFiles |
	packages.NeedSyntax |
	packages.NeedTypes |
	packages.NeedTypesInfo |
	packages.NeedImports |
	packages.NeedDeps |
	packages.NeedModule

// Scanner computes the minimum Go version for a module's own code. It is built
// once from a History (the $GOROOT/api data) and reused.
type Scanner struct {
	// Tests includes _test.go files in the analysis when true.
	Tests bool
	hist  *History
}

// NewScanner returns a Scanner backed by hist. hist may be nil, in which case only
// language-feature detection runs (no stdlib-symbol versions); callers that want
// the full analysis should pass a History from LoadHistory and skip the check when
// it returns ErrNoAPI.
func NewScanner(hist *History) *Scanner { return &Scanner{hist: hist} }

// run accumulates the deciding version across one ScanDir, keeping every reason at
// the current maximum so the result can explain the floor.
type run struct {
	hist    *History
	info    *types.Info
	fset    *token.FileSet
	min     int
	reasons []Reason
}

// track folds one (minor, desc) requirement into the running maximum, replacing the
// reason set when it rises and appending when it ties.
func (r *run) track(minor int, desc string, pos token.Pos) {
	if minor < r.min {
		return
	}
	reason := Reason{Minor: minor, Desc: desc}
	if pos.IsValid() && r.fset != nil {
		reason.Pos = r.fset.Position(pos).String()
	}
	if minor > r.min {
		r.min = minor
		r.reasons = []Reason{reason}
		return
	}
	r.reasons = append(r.reasons, reason)
}

// ScanDir loads every package under dir ("./...") with full type info and returns
// the lowest Go minor version the code can declare. It returns an error when the
// packages cannot be loaded or do not type-check, since the analysis needs type
// info to be correct; callers typically turn that into a skip (a non-compiling repo
// already fails the build check).
func (s *Scanner) ScanDir(ctx context.Context, dir string) (Result, error) {
	cfg := &packages.Config{
		Mode:    loadMode,
		Dir:     dir,
		Tests:   s.Tests,
		Context: ctx,
	}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return Result{}, fmt.Errorf("failed to load packages: %w", err)
	}
	if err := firstLoadError(pkgs); err != nil {
		return Result{}, err
	}

	r := &run{hist: s.hist, fset: nil, min: 0}
	for _, pkg := range pkgs {
		if pkg.TypesInfo == nil || len(pkg.Syntax) == 0 {
			continue
		}
		r.info = pkg.TypesInfo
		r.fset = pkg.Fset
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				r.visit(n)
				return true
			})
		}
	}
	return Result{Min: r.min, Reasons: r.reasons}, nil
}

// firstLoadError returns a load/type-check error if any package failed, so the
// caller does not analyze half-resolved type info.
func firstLoadError(pkgs []*packages.Package) error {
	for _, pkg := range pkgs {
		for _, e := range pkg.Errors {
			return fmt.Errorf("failed to type-check %s: %w", pkg.PkgPath, e)
		}
	}
	return nil
}

// visit dispatches one node to the language-feature checks and, when a History is
// present, the stdlib-symbol lookup.
func (r *run) visit(n ast.Node) {
	r.langFeature(n)
	if r.hist != nil {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			r.stdlibSelector(sel)
		}
	}
}

// stdlibSelector resolves a selector to a stdlib symbol and tracks the version it
// was introduced in. It handles both package-qualified references (pkg.Symbol) and
// method/field selections on a value (x.Method), keying the latter by the named
// type that owns the member.
func (r *run) stdlibSelector(sel *ast.SelectorExpr) {
	if selxn, ok := r.info.Selections[sel]; ok {
		named := namedOf(selxn.Recv())
		if named == nil || named.Obj().Pkg() == nil {
			return
		}
		pkg := named.Obj().Pkg().Path()
		typeName := named.Obj().Name()
		if v, ok := r.hist.lookupMember(pkg, typeName, sel.Sel.Name); ok {
			r.track(v, fmt.Sprintf("%q.%s.%s", pkg, typeName, sel.Sel.Name), sel.Sel.Pos())
		}
		return
	}

	xIdent, ok := sel.X.(*ast.Ident)
	if !ok {
		return
	}
	pkgName, ok := r.info.Uses[xIdent].(*types.PkgName)
	if !ok {
		return
	}
	pkg := pkgName.Imported().Path()
	if v, ok := r.hist.lookup(pkg, sel.Sel.Name); ok {
		r.track(v, fmt.Sprintf("%q.%s", pkg, sel.Sel.Name), sel.Sel.Pos())
	}
}

// namedOf unwraps pointers and aliases to the underlying named type, or nil when
// the type is not named (e.g. a struct literal type).
func namedOf(t types.Type) *types.Named {
	switch x := t.(type) {
	case *types.Pointer:
		return namedOf(x.Elem())
	case *types.Alias:
		return namedOf(types.Unalias(x))
	case *types.Named:
		return x
	default:
		return nil
	}
}
