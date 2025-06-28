package main

import (
	"go/ast"
	"go/token"
)

type PanicRecoverUsage struct {
	Type       string   `json:"type"` // "panic" or "recover"
	Position   Position `json:"position"`
	InDefer    bool     `json:"in_defer"`
	Message    string   `json:"message,omitempty"`
	Context    string   `json:"context"`
}

type PanicRecoverAnalysis struct {
	Usages []PanicRecoverUsage  `json:"usages"`
	Issues []PanicRecoverIssue  `json:"issues"`
}

type PanicRecoverIssue struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Position    Position `json:"position"`
}

func findPanicRecover(dir string) (*PanicRecoverAnalysis, error) {
	analysis := &PanicRecoverAnalysis{
		Usages: []PanicRecoverUsage{},
		Issues: []PanicRecoverIssue{},
	}

	err := walkGoFiles(dir, func(path string, src []byte, file *ast.File, fset *token.FileSet) error {
		// Track function boundaries and defer statements
		var currentFunc *ast.FuncDecl
		deferDepth := 0

		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.FuncDecl:
				currentFunc = node
				deferDepth = 0
			
			case *ast.DeferStmt:
				deferDepth++
				// Check for recover in defer
				ast.Inspect(node, func(inner ast.Node) bool {
					if call, ok := inner.(*ast.CallExpr); ok {
						if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == "recover" {
							pos := fset.Position(call.Pos())
							usage := PanicRecoverUsage{
								Type:     "recover",
								Position: newPosition(pos),
								InDefer:  true,
								Context:  extractContext(src, pos),
							}
							analysis.Usages = append(analysis.Usages, usage)
						}
					}
					return true
				})
				deferDepth--
			
			case *ast.CallExpr:
				if ident, ok := node.Fun.(*ast.Ident); ok {
					pos := fset.Position(node.Pos())
					
					switch ident.Name {
					case "panic":
						message := extractPanicMessage(node)
						usage := PanicRecoverUsage{
							Type:     "panic",
							Position: newPosition(pos),
							InDefer:  deferDepth > 0,
							Message:  message,
							Context:  extractContext(src, pos),
						}
						analysis.Usages = append(analysis.Usages, usage)
						
						// Check if panic is in main or init
						if currentFunc != nil && (currentFunc.Name.Name == "main" || currentFunc.Name.Name == "init") {
							issue := PanicRecoverIssue{
								Type:        "panic_in_main_init",
								Description: "Panic in " + currentFunc.Name.Name + " function will crash the program",
								Position:    newPosition(pos),
							}
							analysis.Issues = append(analysis.Issues, issue)
						}
					
					case "recover":
						if deferDepth == 0 {
							issue := PanicRecoverIssue{
								Type:        "recover_outside_defer",
								Description: "recover() called outside defer statement - it will always return nil",
								Position:    newPosition(pos),
							}
							analysis.Issues = append(analysis.Issues, issue)
						}
						
						usage := PanicRecoverUsage{
							Type:     "recover",
							Position: newPosition(pos),
							InDefer:  deferDepth > 0,
							Context:  extractContext(src, pos),
						}
						analysis.Usages = append(analysis.Usages, usage)
					}
				}
			}
			return true
		})

		// Check for functions with panic but no recover
		checkPanicWithoutRecover(file, fset, analysis)

		return nil
	})

	return analysis, err
}

func extractPanicMessage(call *ast.CallExpr) string {
	if len(call.Args) > 0 {
		switch arg := call.Args[0].(type) {
		case *ast.BasicLit:
			return arg.Value
		case *ast.Ident:
			return arg.Name
		case *ast.SelectorExpr:
			return exprToString(arg)
		default:
			return "complex expression"
		}
	}
	return ""
}

func checkPanicWithoutRecover(file *ast.File, fset *token.FileSet, analysis *PanicRecoverAnalysis) {
	// For each function, check if it has panic but no recover
	ast.Inspect(file, func(n ast.Node) bool {
		funcDecl, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}

		hasPanic := false
		hasRecover := false
		var panicPos token.Position

		ast.Inspect(funcDecl, func(inner ast.Node) bool {
			if call, ok := inner.(*ast.CallExpr); ok {
				if ident, ok := call.Fun.(*ast.Ident); ok {
					if ident.Name == "panic" {
						hasPanic = true
						panicPos = fset.Position(call.Pos())
					} else if ident.Name == "recover" {
						hasRecover = true
					}
				}
			}
			return true
		})

		if hasPanic && !hasRecover && funcDecl.Name.Name != "main" && funcDecl.Name.Name != "init" {
			issue := PanicRecoverIssue{
				Type:        "panic_without_recover",
				Description: "Function " + funcDecl.Name.Name + " calls panic() but has no recover() - consider adding error handling",
				Position:    newPosition(panicPos),
			}
			analysis.Issues = append(analysis.Issues, issue)
		}

		return true
	})
}