package main

import (
	"go/ast"
	"go/token"
	"strings"
)

// Error handling types
type ErrorInfo struct {
	File           string         `json:"file"`
	UnhandledErrors []ErrorContext `json:"unhandled_errors,omitempty"`
	ErrorChecks    []ErrorContext `json:"error_checks,omitempty"`
	ErrorReturns   []ErrorContext `json:"error_returns,omitempty"`
}

type ErrorContext struct {
	Context  string   `json:"context"`
	Type     string   `json:"type"`
	Position Position `json:"position"`
}

func findErrors(dir string) ([]ErrorInfo, error) {
	var errors []ErrorInfo

	err := walkGoFiles(dir, func(path string, src []byte, file *ast.File, fset *token.FileSet) error {
		info := ErrorInfo{
			File: path,
		}

		ast.Inspect(file, func(n ast.Node) bool {
			switch x := n.(type) {
			// Find function calls that return errors but aren't checked
			case *ast.ExprStmt:
				if call, ok := x.X.(*ast.CallExpr); ok {
					// Check if this function likely returns an error
					if callReturnsError(call) {
						pos := fset.Position(call.Pos())
						context := extractContext(src, pos)
						info.UnhandledErrors = append(info.UnhandledErrors, ErrorContext{
							Context:  context,
							Type:     "unchecked_call",
							Position: newPosition(pos),
						})
					}
				}

			// Find error checks
			case *ast.IfStmt:
				if isErrorCheck(x) {
					pos := fset.Position(x.Pos())
					context := extractContext(src, pos)
					info.ErrorChecks = append(info.ErrorChecks, ErrorContext{
						Context:  context,
						Type:     "error_check",
						Position: newPosition(pos),
					})
				}

			// Find error returns
			case *ast.ReturnStmt:
				for _, result := range x.Results {
					if ident, ok := result.(*ast.Ident); ok && (ident.Name == "err" || strings.Contains(ident.Name, "error")) {
						pos := fset.Position(x.Pos())
						context := extractContext(src, pos)
						info.ErrorReturns = append(info.ErrorReturns, ErrorContext{
							Context:  context,
							Type:     "error_return",
							Position: newPosition(pos),
						})
						break
					}
				}
			}
			return true
		})

		if len(info.UnhandledErrors) > 0 || len(info.ErrorChecks) > 0 || len(info.ErrorReturns) > 0 {
			errors = append(errors, info)
		}
		return nil
	})

	return errors, err
}

func callReturnsError(call *ast.CallExpr) bool {
	// Simple heuristic: check if the function name suggests it returns an error
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		name := fun.Name
		return strings.HasPrefix(name, "New") || strings.HasPrefix(name, "Create") ||
			strings.HasPrefix(name, "Open") || strings.HasPrefix(name, "Read") ||
			strings.HasPrefix(name, "Write") || strings.HasPrefix(name, "Parse")
	case *ast.SelectorExpr:
		name := fun.Sel.Name
		return strings.HasPrefix(name, "New") || strings.HasPrefix(name, "Create") ||
			strings.HasPrefix(name, "Open") || strings.HasPrefix(name, "Read") ||
			strings.HasPrefix(name, "Write") || strings.HasPrefix(name, "Parse")
	}
	return false
}