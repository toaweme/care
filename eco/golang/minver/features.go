package minver

import (
	"go/ast"
	"go/token"
	"go/types"
)

// langFeature detects a Go language/syntax feature on one node and tracks the minor
// version it requires. Stdlib-symbol versions come from the api history; these are
// the syntax-level features that history cannot express, each with the version it
// landed in baked in from the Go release notes.
func (r *run) langFeature(n ast.Node) {
	switch x := n.(type) {
	case *ast.FuncDecl:
		if x.Type != nil && x.Type.TypeParams != nil && len(x.Type.TypeParams.List) > 0 {
			r.track(18, "generic function", x.Pos())
		}
	case *ast.TypeSpec:
		generic := x.TypeParams != nil && len(x.TypeParams.List) > 0
		alias := x.Assign.IsValid()
		switch {
		case generic && alias:
			r.track(24, "generic type alias", x.Pos())
		case generic:
			r.track(18, "generic type", x.Pos())
		case alias:
			r.track(9, "type alias", x.Pos())
		}
	case *ast.RangeStmt:
		r.rangeStmt(x)
	case *ast.CallExpr:
		r.builtinCall(x)
	case *ast.Ident:
		r.predeclaredAny(x)
	case *ast.UnaryExpr:
		if x.Op == token.TILDE {
			r.track(18, "type constraint element (~)", x.Pos())
		}
	case *ast.SliceExpr:
		if x.Slice3 {
			r.track(5, "three-index slice", x.Pos())
		}
	}
}

// rangeStmt flags ranging over an integer (1.22) or over a function value (1.23),
// both of which need the type of the range expression to disambiguate from the
// long-standing slice/map/chan/string forms.
func (r *run) rangeStmt(x *ast.RangeStmt) {
	if r.info == nil || x.X == nil {
		return
	}
	t := r.info.TypeOf(x.X)
	if t == nil {
		return
	}
	switch u := t.Underlying().(type) {
	case *types.Basic:
		if u.Info()&types.IsInteger != 0 {
			r.track(22, "range over integer", x.Pos())
		}
	case *types.Signature:
		r.track(23, "range over function", x.Pos())
	}
}

// builtinSince maps the builtins added after go1.0 to their introduction version.
var builtinSince = map[string]int{
	"min":   21,
	"max":   21,
	"clear": 21,
}

// builtinCall flags a call to a post-1.0 builtin (min/max/clear), resolving the
// callee through type info so a same-named local function is not misread.
func (r *run) builtinCall(x *ast.CallExpr) {
	if r.info == nil {
		return
	}
	ident, ok := x.Fun.(*ast.Ident)
	if !ok {
		return
	}
	if _, isBuiltin := r.info.Uses[ident].(*types.Builtin); !isBuiltin {
		return
	}
	if v, ok := builtinSince[ident.Name]; ok {
		r.track(v, "builtin "+ident.Name, x.Pos())
	}
}

// predeclaredAny flags use of the predeclared `any` (1.18). A predeclared name
// resolves to a universe-scope object with no package, which distinguishes it from
// a user-defined type named any.
func (r *run) predeclaredAny(x *ast.Ident) {
	if r.info == nil || x.Name != "any" {
		return
	}
	obj, ok := r.info.Uses[x].(*types.TypeName)
	if !ok || obj.Pkg() != nil {
		return
	}
	r.track(18, "predeclared any", x.Pos())
}
