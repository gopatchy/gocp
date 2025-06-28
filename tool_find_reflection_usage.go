package main

import (
	"go/ast"
	"go/token"
)

type ReflectionUsage struct {
	Type        string   `json:"type"` // "TypeOf", "ValueOf", "MethodByName", etc.
	Target      string   `json:"target"`
	Position    Position `json:"position"`
	Context     string   `json:"context"`
}

type ReflectionAnalysis struct {
	Usages []ReflectionUsage  `json:"usages"`
	Issues []ReflectionIssue  `json:"issues"`
}

type ReflectionIssue struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Position    Position `json:"position"`
}

func findReflectionUsage(dir string) (*ReflectionAnalysis, error) {
	analysis := &ReflectionAnalysis{
		Usages: []ReflectionUsage{},
		Issues: []ReflectionIssue{},
	}

	err := walkGoFiles(dir, func(path string, src []byte, file *ast.File, fset *token.FileSet) error {
		// Check if reflect package is imported
		hasReflectImport := false
		for _, imp := range file.Imports {
			if imp.Path != nil && imp.Path.Value == `"reflect"` {
				hasReflectImport = true
				break
			}
		}

		if !hasReflectImport {
			return nil
		}

		ast.Inspect(file, func(n ast.Node) bool {
			if callExpr, ok := n.(*ast.CallExpr); ok {
				analyzeReflectCall(callExpr, file, fset, src, analysis)
			}
			return true
		})

		return nil
	})

	return analysis, err
}

func analyzeReflectCall(call *ast.CallExpr, file *ast.File, fset *token.FileSet, src []byte, analysis *ReflectionAnalysis) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}

	// Check if it's a reflect package call
	ident, ok := sel.X.(*ast.Ident)
	if !ok || ident.Name != "reflect" {
		return
	}

	pos := fset.Position(call.Pos())
	methodName := sel.Sel.Name
	target := ""
	if len(call.Args) > 0 {
		target = exprToString(call.Args[0])
	}

	usage := ReflectionUsage{
		Type:     methodName,
		Target:   target,
		Position: newPosition(pos),
		Context:  extractContext(src, pos),
	}
	analysis.Usages = append(analysis.Usages, usage)

	// Analyze specific reflection patterns
	switch methodName {
	case "TypeOf", "ValueOf":
		if isInLoop(file, call) {
			issue := ReflectionIssue{
				Type:        "reflection_in_loop",
				Description: "reflect." + methodName + " called in loop - consider caching result",
				Position:    newPosition(pos),
			}
			analysis.Issues = append(analysis.Issues, issue)
		}

	case "MethodByName", "FieldByName":
		// These are particularly slow
		issue := ReflectionIssue{
			Type:        "slow_reflection",
			Description: "reflect." + methodName + " is slow - consider caching or avoiding if possible",
			Position:    newPosition(pos),
		}
		analysis.Issues = append(analysis.Issues, issue)

		if isInLoop(file, call) {
			issue := ReflectionIssue{
				Type:        "slow_reflection_in_loop",
				Description: "reflect." + methodName + " in loop is very inefficient",
				Position:    newPosition(pos),
			}
			analysis.Issues = append(analysis.Issues, issue)
		}

	case "DeepEqual":
		if isInHotPath(file, call) {
			issue := ReflectionIssue{
				Type:        "deep_equal_performance",
				Description: "reflect.DeepEqual is expensive - consider custom comparison for hot paths",
				Position:    newPosition(pos),
			}
			analysis.Issues = append(analysis.Issues, issue)
		}

	case "Copy", "AppendSlice", "MakeSlice", "MakeMap", "MakeChan":
		// These allocate memory
		if isInLoop(file, call) {
			issue := ReflectionIssue{
				Type:        "reflect_allocation_in_loop",
				Description: "reflect." + methodName + " allocates memory in loop",
				Position:    newPosition(pos),
			}
			analysis.Issues = append(analysis.Issues, issue)
		}
	}

	// Check for unsafe reflection patterns
	checkUnsafeReflection(call, file, fset, analysis)
}

func checkUnsafeReflection(call *ast.CallExpr, file *ast.File, fset *token.FileSet, analysis *ReflectionAnalysis) {
	// Look for patterns like Value.Interface() without type checking
	ast.Inspect(file, func(n ast.Node) bool {
		if selExpr, ok := n.(*ast.SelectorExpr); ok {
			if selExpr.Sel.Name == "Interface" {
				// Check if this is on a reflect.Value
				if isReflectValueExpr(file, selExpr.X) {
					// Check if result is used in type assertion without ok check
					if parent := findParentNode(file, selExpr); parent != nil {
						if typeAssert, ok := parent.(*ast.TypeAssertExpr); ok {
							if !isUsedWithOkCheck(file, typeAssert) {
								pos := fset.Position(typeAssert.Pos())
								issue := ReflectionIssue{
									Type:        "unsafe_interface_conversion",
									Description: "Type assertion on reflect.Value.Interface() without ok check",
									Position:    newPosition(pos),
								}
								analysis.Issues = append(analysis.Issues, issue)
							}
						}
					}
				}
			}
		}
		return true
	})
}

func isReflectValueExpr(file *ast.File, expr ast.Expr) bool {
	// Simple heuristic - check if expression contains "reflect.Value" operations
	switch e := expr.(type) {
	case *ast.CallExpr:
		if sel, ok := e.Fun.(*ast.SelectorExpr); ok {
			if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "reflect" {
				return sel.Sel.Name == "ValueOf" || sel.Sel.Name == "Indirect"
			}
		}
	case *ast.Ident:
		// Check if it's a variable of type reflect.Value
		return isReflectValueVar(file, e.Name)
	}
	return false
}

func isReflectValueVar(file *ast.File, varName string) bool {
	var isValue bool
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.ValueSpec:
			for i, name := range node.Names {
				if name.Name == varName {
					if node.Type != nil {
						if sel, ok := node.Type.(*ast.SelectorExpr); ok {
							if ident, ok := sel.X.(*ast.Ident); ok {
								isValue = ident.Name == "reflect" && sel.Sel.Name == "Value"
								return false
							}
						}
					} else if i < len(node.Values) {
						// Check if assigned from reflect.ValueOf
						if call, ok := node.Values[i].(*ast.CallExpr); ok {
							if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
								if ident, ok := sel.X.(*ast.Ident); ok {
									isValue = ident.Name == "reflect" && sel.Sel.Name == "ValueOf"
									return false
								}
							}
						}
					}
				}
			}
		}
		return true
	})
	return isValue
}

func findParentNode(file *ast.File, target ast.Node) ast.Node {
	var parent ast.Node
	ast.Inspect(file, func(n ast.Node) bool {
		// This is a simplified parent finder
		switch node := n.(type) {
		case *ast.TypeAssertExpr:
			if node.X == target {
				parent = node
				return false
			}
		case *ast.CallExpr:
			for _, arg := range node.Args {
				if arg == target {
					parent = node
					return false
				}
			}
		}
		return true
	})
	return parent
}

func isInHotPath(file *ast.File, node ast.Node) bool {
	// Check if node is in a function that looks like a hot path
	var inHotPath bool
	ast.Inspect(file, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok && containsNode(fn, node) {
			// Check function name for common hot path patterns
			name := fn.Name.Name
			if name == "ServeHTTP" || name == "Handle" || name == "Process" ||
				name == "Execute" || name == "Run" || name == "Do" {
				inHotPath = true
				return false
			}
			// Check if function is called frequently (in loops)
			if isFunctionCalledInLoop(file, fn.Name.Name) {
				inHotPath = true
				return false
			}
		}
		return true
	})
	return inHotPath
}

func isFunctionCalledInLoop(file *ast.File, funcName string) bool {
	var calledInLoop bool
	ast.Inspect(file, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == funcName {
				if isInLoop(file, call) {
					calledInLoop = true
					return false
				}
			}
		}
		return true
	})
	return calledInLoop
}