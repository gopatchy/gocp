package main

import (
	"go/ast"
	"go/token"
	"strings"
)

type DeferUsage struct {
	Statement   string   `json:"statement"`
	Position    Position `json:"position"`
	InLoop      bool     `json:"in_loop"`
	InFunction  string   `json:"in_function"`
	Context     string   `json:"context"`
}

type DeferAnalysis struct {
	Defers []DeferUsage  `json:"defers"`
	Issues []DeferIssue  `json:"issues"`
}

type DeferIssue struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Position    Position `json:"position"`
}

func analyzeDeferPatterns(dir string) (*DeferAnalysis, error) {
	analysis := &DeferAnalysis{
		Defers: []DeferUsage{},
		Issues: []DeferIssue{},
	}

	err := walkGoFiles(dir, func(path string, src []byte, file *ast.File, fset *token.FileSet) error {
		var currentFunc string

		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.FuncDecl:
				currentFunc = node.Name.Name
				analyzeFunctionDefers(node, fset, src, analysis)

			case *ast.FuncLit:
				currentFunc = "anonymous function"
				analyzeFunctionDefers(&ast.FuncDecl{Body: node.Body}, fset, src, analysis)

			case *ast.DeferStmt:
				pos := fset.Position(node.Pos())
				usage := DeferUsage{
					Statement:  extractDeferStatement(node),
					Position:   newPosition(pos),
					InLoop:     isInLoop(file, node),
					InFunction: currentFunc,
					Context:    extractContext(src, pos),
				}
				analysis.Defers = append(analysis.Defers, usage)

				// Check for issues
				if usage.InLoop {
					issue := DeferIssue{
						Type:        "defer_in_loop",
						Description: "defer in loop will accumulate until function returns",
						Position:    newPosition(pos),
					}
					analysis.Issues = append(analysis.Issues, issue)
				}

				// Check for defer of result of function call
				if hasNestedCall(node.Call) {
					issue := DeferIssue{
						Type:        "defer_nested_call",
						Description: "defer evaluates function arguments immediately - nested calls execute now",
						Position:    newPosition(pos),
					}
					analysis.Issues = append(analysis.Issues, issue)
				}

				// Check for useless defer patterns
				checkUselessDefer(node, file, fset, analysis)
			}
			return true
		})

		return nil
	})

	return analysis, err
}

func extractDeferStatement(deferStmt *ast.DeferStmt) string {
	switch call := deferStmt.Call.Fun.(type) {
	case *ast.Ident:
		return "defer " + call.Name + "(...)"
	case *ast.SelectorExpr:
		return "defer " + exprToString(call) + "(...)"
	case *ast.FuncLit:
		return "defer func() { ... }"
	default:
		return "defer <unknown>"
	}
}

func hasNestedCall(call *ast.CallExpr) bool {
	// Check if any argument is a function call
	for _, arg := range call.Args {
		if _, ok := arg.(*ast.CallExpr); ok {
			return true
		}
	}
	return false
}

func analyzeFunctionDefers(fn *ast.FuncDecl, fset *token.FileSet, src []byte, analysis *DeferAnalysis) {
	if fn.Body == nil {
		return
	}

	var defers []*ast.DeferStmt
	var hasReturn bool
	var returnPos token.Position

	// Collect all defers and check for early returns
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.DeferStmt:
			defers = append(defers, node)
		case *ast.ReturnStmt:
			hasReturn = true
			returnPos = fset.Position(node.Pos())
		case *ast.FuncLit:
			// Don't analyze nested functions
			return false
		}
		return true
	})

	// Check defer ordering issues
	if len(defers) > 1 {
		checkDeferOrdering(defers, fset, analysis)
	}

	// Check for defer after return path
	if hasReturn {
		for _, def := range defers {
			defPos := fset.Position(def.Pos())
			if defPos.Line > returnPos.Line {
				issue := DeferIssue{
					Type:        "unreachable_defer",
					Description: "defer statement after return is unreachable",
					Position:    newPosition(defPos),
				}
				analysis.Issues = append(analysis.Issues, issue)
			}
		}
	}

	// Check for missing defer on resource cleanup
	checkMissingDefers(fn, fset, analysis)
}

func checkDeferOrdering(defers []*ast.DeferStmt, fset *token.FileSet, analysis *DeferAnalysis) {
	// Check for dependent defers in wrong order
	for i := 0; i < len(defers)-1; i++ {
		for j := i + 1; j < len(defers); j++ {
			if areDefersDependentWrongOrder(defers[i], defers[j]) {
				pos := fset.Position(defers[j].Pos())
				issue := DeferIssue{
					Type:        "defer_order_issue",
					Description: "defer statements may execute in wrong order (LIFO)",
					Position:    newPosition(pos),
				}
				analysis.Issues = append(analysis.Issues, issue)
			}
		}
	}
}

func areDefersDependentWrongOrder(first, second *ast.DeferStmt) bool {
	// Simple heuristic: check for Close() after Flush() or similar patterns
	firstName := extractMethodName(first.Call)
	secondName := extractMethodName(second.Call)

	// Common patterns where order matters
	orderPatterns := map[string]string{
		"Flush": "Close",
		"Unlock": "Lock",
		"Done": "Add",
	}

	for before, after := range orderPatterns {
		if firstName == after && secondName == before {
			return true
		}
	}

	return false
}

func extractMethodName(call *ast.CallExpr) string {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		return fun.Name
	case *ast.SelectorExpr:
		return fun.Sel.Name
	}
	return ""
}

func checkUselessDefer(deferStmt *ast.DeferStmt, file *ast.File, fset *token.FileSet, analysis *DeferAnalysis) {
	// Check if defer is the last statement before return
	ast.Inspect(file, func(n ast.Node) bool {
		if block, ok := n.(*ast.BlockStmt); ok {
			for i, stmt := range block.List {
				if stmt == deferStmt && i < len(block.List)-1 {
					// Check if next statement is return
					if _, ok := block.List[i+1].(*ast.ReturnStmt); ok {
						pos := fset.Position(deferStmt.Pos())
						issue := DeferIssue{
							Type:        "unnecessary_defer",
							Description: "defer immediately before return is unnecessary",
							Position:    newPosition(pos),
						}
						analysis.Issues = append(analysis.Issues, issue)
						return false
					}
				}
			}
		}
		return true
	})
}

func checkMissingDefers(fn *ast.FuncDecl, fset *token.FileSet, analysis *DeferAnalysis) {
	// Look for resource acquisition without corresponding defer
	resources := make(map[string]token.Position) // resource var -> position
	deferred := make(map[string]bool)

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.AssignStmt:
			// Check for resource acquisition
			for i, lhs := range node.Lhs {
				if ident, ok := lhs.(*ast.Ident); ok && i < len(node.Rhs) {
					if isResourceAcquisition(node.Rhs[i]) {
						resources[ident.Name] = fset.Position(node.Pos())
					}
				}
			}

		case *ast.DeferStmt:
			// Check if defer releases a resource
			if varName := extractDeferredResourceVar(node.Call); varName != "" {
				deferred[varName] = true
			}
		}
		return true
	})

	// Report resources without defers
	for resource, pos := range resources {
		if !deferred[resource] {
			issue := DeferIssue{
				Type:        "missing_defer",
				Description: "Resource '" + resource + "' acquired but not deferred for cleanup",
				Position:    newPosition(pos),
			}
			analysis.Issues = append(analysis.Issues, issue)
		}
	}
}

func isResourceAcquisition(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}

	// Check for common resource acquisition patterns
	switch fun := call.Fun.(type) {
	case *ast.SelectorExpr:
		method := fun.Sel.Name
		resourceMethods := []string{"Open", "Create", "Dial", "Connect", "Lock", "RLock", "Begin"}
		for _, rm := range resourceMethods {
			if method == rm || strings.HasPrefix(method, "Open") || strings.HasPrefix(method, "New") {
				return true
			}
		}
	}
	return false
}

func extractDeferredResourceVar(call *ast.CallExpr) string {
	// Extract the variable being cleaned up in defer
	switch fun := call.Fun.(type) {
	case *ast.SelectorExpr:
		if ident, ok := fun.X.(*ast.Ident); ok {
			method := fun.Sel.Name
			if method == "Close" || method == "Unlock" || method == "RUnlock" || 
			   method == "Done" || method == "Release" {
				return ident.Name
			}
		}
	}
	return ""
}