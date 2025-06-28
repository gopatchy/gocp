package main

import (
	"go/ast"
	"go/token"
	"strings"
)

type GoroutineUsage struct {
	Position    Position `json:"position"`
	Function    string   `json:"function"`
	InLoop      bool     `json:"in_loop"`
	HasWaitGroup bool    `json:"has_wait_group"`
	Context     string   `json:"context"`
}

type GoroutineAnalysis struct {
	Goroutines []GoroutineUsage  `json:"goroutines"`
	Issues     []GoroutineIssue  `json:"issues"`
}

type GoroutineIssue struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Position    Position `json:"position"`
}

func analyzeGoroutines(dir string) (*GoroutineAnalysis, error) {
	analysis := &GoroutineAnalysis{
		Goroutines: []GoroutineUsage{},
		Issues:     []GoroutineIssue{},
	}

	err := walkGoFiles(dir, func(path string, src []byte, file *ast.File, fset *token.FileSet) error {
		// Track WaitGroup usage
		waitGroupVars := make(map[string]bool)
		hasWaitGroupImport := false

		// Check imports
		for _, imp := range file.Imports {
			if imp.Path != nil && imp.Path.Value == `"sync"` {
				hasWaitGroupImport = true
				break
			}
		}

		// First pass: find WaitGroup variables
		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.ValueSpec:
				for i, name := range node.Names {
					if i < len(node.Values) {
						if isWaitGroupType(node.Type) || isWaitGroupExpr(node.Values[i]) {
							waitGroupVars[name.Name] = true
						}
					}
				}
			case *ast.AssignStmt:
				for i, lhs := range node.Lhs {
					if ident, ok := lhs.(*ast.Ident); ok && i < len(node.Rhs) {
						if isWaitGroupExpr(node.Rhs[i]) {
							waitGroupVars[ident.Name] = true
						}
					}
				}
			}
			return true
		})

		// Second pass: analyze goroutines
		ast.Inspect(file, func(n ast.Node) bool {
			if goStmt, ok := n.(*ast.GoStmt); ok {
				pos := fset.Position(goStmt.Pos())
				funcName := extractFunctionName(goStmt.Call)
				inLoop := isInLoop(file, goStmt)
				hasWG := hasNearbyWaitGroup(file, goStmt, waitGroupVars)
				
				usage := GoroutineUsage{
					Position:     newPosition(pos),
					Function:     funcName,
					InLoop:       inLoop,
					HasWaitGroup: hasWG,
					Context:      extractContext(src, pos),
				}
				analysis.Goroutines = append(analysis.Goroutines, usage)

				// Check for issues
				if inLoop && !hasWG {
					issue := GoroutineIssue{
						Type:        "goroutine_leak_risk",
						Description: "Goroutine launched in loop without WaitGroup may cause resource leak",
						Position:    newPosition(pos),
					}
					analysis.Issues = append(analysis.Issues, issue)
				}

				// Check for goroutines without synchronization
				if !hasWG && !hasChannelCommunication(goStmt.Call) && hasWaitGroupImport {
					issue := GoroutineIssue{
						Type:        "missing_synchronization",
						Description: "Goroutine launched without apparent synchronization mechanism",
						Position:    newPosition(pos),
					}
					analysis.Issues = append(analysis.Issues, issue)
				}
			}
			return true
		})

		return nil
	})

	return analysis, err
}

func isWaitGroupType(expr ast.Expr) bool {
	if expr == nil {
		return false
	}
	if sel, ok := expr.(*ast.SelectorExpr); ok {
		if ident, ok := sel.X.(*ast.Ident); ok {
			return ident.Name == "sync" && sel.Sel.Name == "WaitGroup"
		}
	}
	return false
}

func isWaitGroupExpr(expr ast.Expr) bool {
	switch e := expr.(type) {
	case *ast.CompositeLit:
		return isWaitGroupType(e.Type)
	case *ast.UnaryExpr:
		if e.Op == token.AND {
			return isWaitGroupExpr(e.X)
		}
	}
	return false
}

func extractFunctionName(call *ast.CallExpr) string {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		return fun.Name
	case *ast.SelectorExpr:
		return exprToString(fun.X) + "." + fun.Sel.Name
	case *ast.FuncLit:
		return "anonymous function"
	default:
		return "unknown"
	}
}

func isInLoop(file *ast.File, target ast.Node) bool {
	var inLoop bool
	ast.Inspect(file, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.ForStmt, *ast.RangeStmt:
			// Check if target is within this loop
			if containsNode(n, target) {
				inLoop = true
				return false
			}
		}
		return true
	})
	return inLoop
}

func containsNode(parent, child ast.Node) bool {
	var found bool
	ast.Inspect(parent, func(n ast.Node) bool {
		if n == child {
			found = true
			return false
		}
		return true
	})
	return found
}

func hasNearbyWaitGroup(file *ast.File, goStmt *ast.GoStmt, waitGroupVars map[string]bool) bool {
	// Look for WaitGroup.Add calls in the same block or parent function
	var hasWG bool
	
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CallExpr:
			if sel, ok := node.Fun.(*ast.SelectorExpr); ok {
				if ident, ok := sel.X.(*ast.Ident); ok {
					if waitGroupVars[ident.Name] && sel.Sel.Name == "Add" {
						// Check if this Add call is near the goroutine
						if isNearby(file, node, goStmt) {
							hasWG = true
							return false
						}
					}
				}
			}
		}
		return true
	})
	
	return hasWG
}

func isNearby(file *ast.File, node1, node2 ast.Node) bool {
	// Simple proximity check - in same function
	var func1, func2 *ast.FuncDecl
	
	ast.Inspect(file, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok {
			if containsNode(fn, node1) {
				func1 = fn
			}
			if containsNode(fn, node2) {
				func2 = fn
			}
		}
		return true
	})
	
	return func1 == func2 && func1 != nil
}

func hasChannelCommunication(call *ast.CallExpr) bool {
	// Check if the function likely uses channels for synchronization
	hasChannel := false
	ast.Inspect(call, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.ChanType, *ast.SendStmt:
			hasChannel = true
			return false
		}
		if ident, ok := n.(*ast.Ident); ok {
			if strings.Contains(strings.ToLower(ident.Name), "chan") {
				hasChannel = true
				return false
			}
		}
		return true
	})
	return hasChannel
}