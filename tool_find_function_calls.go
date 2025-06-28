package main

import (
	"go/ast"
	"go/token"
)

// Function call types
type FunctionCall struct {
	Caller   string   `json:"caller"`
	Context  string   `json:"context"`
	Position Position `json:"position"`
}

func findFunctionCalls(dir string, functionName string) ([]FunctionCall, error) {
	var calls []FunctionCall

	err := walkGoFiles(dir, func(path string, src []byte, file *ast.File, fset *token.FileSet) error {
		currentFunc := ""
		
		ast.Inspect(file, func(n ast.Node) bool {
			// Track current function context
			if fn, ok := n.(*ast.FuncDecl); ok {
				currentFunc = fn.Name.Name
				return true
			}

			// Find function calls
			switch x := n.(type) {
			case *ast.CallExpr:
				var calledName string
				switch fun := x.Fun.(type) {
				case *ast.Ident:
					calledName = fun.Name
				case *ast.SelectorExpr:
					calledName = fun.Sel.Name
				}

				if calledName == functionName {
					pos := fset.Position(x.Pos())
					context := extractContext(src, pos)
					
					calls = append(calls, FunctionCall{
						Caller:   currentFunc,
						Context:  context,
						Position: newPosition(pos),
					})
				}
			}
			return true
		})

		return nil
	})

	return calls, err
}