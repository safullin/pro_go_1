package main

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var analyzer = &analysis.Analyzer{
	Name: "exitcheck",
	Doc:  "reports panic calls and process termination outside the main function",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	visitor := &callVisitor{pass: pass}
	for _, file := range pass.Files {
		ast.Walk(visitor, file)
	}
	return nil, nil
}

type callVisitor struct {
	pass      *analysis.Pass
	allowExit bool
}

func (v *callVisitor) Visit(node ast.Node) ast.Visitor {
	switch current := node.(type) {
	case *ast.FuncDecl:
		if current.Body != nil {
			allowExit := v.pass.Pkg.Name() == "main" && current.Recv == nil && current.Name.Name == "main"
			ast.Walk(&callVisitor{pass: v.pass, allowExit: allowExit}, current.Body)
		}
		return nil
	case *ast.FuncLit:
		ast.Walk(&callVisitor{pass: v.pass}, current.Body)
		return nil
	case *ast.CallExpr:
		v.checkCall(current)
	}
	return v
}

func (v *callVisitor) checkCall(call *ast.CallExpr) {
	if identifier, ok := call.Fun.(*ast.Ident); ok {
		if builtin, ok := v.pass.TypesInfo.Uses[identifier].(*types.Builtin); ok && builtin.Name() == "panic" {
			v.pass.Reportf(call.Fun.Pos(), "use of built-in panic is prohibited")
			return
		}
	}

	function, ok := calledObject(v.pass.TypesInfo, call.Fun).(*types.Func)
	if !ok || function.Pkg() == nil {
		return
	}

	packagePath := function.Pkg().Path()
	functionName := function.Name()
	forbidden := packagePath == "os" && functionName == "Exit" || packagePath == "log" && functionName == "Fatal"
	if forbidden && !v.allowExit {
		v.pass.Reportf(call.Fun.Pos(), "%s.%s may only be called from main function of package main", function.Pkg().Name(), functionName)
	}
}

func calledObject(info *types.Info, expression ast.Expr) types.Object {
	switch current := expression.(type) {
	case *ast.Ident:
		return info.Uses[current]
	case *ast.SelectorExpr:
		return info.Uses[current.Sel]
	default:
		return nil
	}
}
