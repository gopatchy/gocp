package main

import (
	"go/ast"
	"go/token"
	"strings"
)

type MemoryAllocation struct {
	Type        string   `json:"type"` // "make", "new", "composite", "append", "string_concat"
	Description string   `json:"description"`
	InLoop      bool     `json:"in_loop"`
	Position    Position `json:"position"`
	Context     string   `json:"context"`
}

type AllocationAnalysis struct {
	Allocations []MemoryAllocation  `json:"allocations"`
	Issues      []AllocationIssue   `json:"issues"`
}

type AllocationIssue struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Position    Position `json:"position"`
}

func analyzeMemoryAllocations(dir string) (*AllocationAnalysis, error) {
	analysis := &AllocationAnalysis{
		Allocations: []MemoryAllocation{},
		Issues:      []AllocationIssue{},
	}

	err := walkGoFiles(dir, func(path string, src []byte, file *ast.File, fset *token.FileSet) error {
		// Analyze allocations
		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.CallExpr:
				analyzeCallExpr(node, file, fset, src, analysis)
			
			case *ast.CompositeLit:
				pos := fset.Position(node.Pos())
				alloc := MemoryAllocation{
					Type:        "composite",
					Description: "Composite literal: " + exprToString(node.Type),
					InLoop:      isInLoop(file, node),
					Position:    newPosition(pos),
					Context:     extractContext(src, pos),
				}
				analysis.Allocations = append(analysis.Allocations, alloc)
				
				if alloc.InLoop {
					issue := AllocationIssue{
						Type:        "allocation_in_loop",
						Description: "Composite literal allocation inside loop",
						Position:    newPosition(pos),
					}
					analysis.Issues = append(analysis.Issues, issue)
				}
			
			case *ast.BinaryExpr:
				if node.Op == token.ADD {
					analyzeStringConcat(node, file, fset, src, analysis)
				}
			
			case *ast.UnaryExpr:
				if node.Op == token.AND {
					// Taking address of value causes allocation
					pos := fset.Position(node.Pos())
					if isEscaping(file, node) {
						alloc := MemoryAllocation{
							Type:        "address_of",
							Description: "Taking address of value (escapes to heap)",
							InLoop:      isInLoop(file, node),
							Position:    newPosition(pos),
							Context:     extractContext(src, pos),
						}
						analysis.Allocations = append(analysis.Allocations, alloc)
					}
				}
			}
			return true
		})

		// Look for specific patterns
		findAllocationPatterns(file, fset, src, analysis)

		return nil
	})

	return analysis, err
}

func analyzeCallExpr(call *ast.CallExpr, file *ast.File, fset *token.FileSet, src []byte, analysis *AllocationAnalysis) {
	ident, ok := call.Fun.(*ast.Ident)
	if !ok {
		// Check for method calls like strings.Builder
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			analyzeMethodCall(sel, call, file, fset, src, analysis)
		}
		return
	}

	pos := fset.Position(call.Pos())
	inLoop := isInLoop(file, call)

	switch ident.Name {
	case "make":
		if len(call.Args) > 0 {
			typeStr := exprToString(call.Args[0])
			sizeStr := ""
			if len(call.Args) > 1 {
				sizeStr = " with size"
			}
			
			alloc := MemoryAllocation{
				Type:        "make",
				Description: "make(" + typeStr + ")" + sizeStr,
				InLoop:      inLoop,
				Position:    newPosition(pos),
				Context:     extractContext(src, pos),
			}
			analysis.Allocations = append(analysis.Allocations, alloc)
			
			if inLoop {
				issue := AllocationIssue{
					Type:        "make_in_loop",
					Description: "make() called inside loop - consider pre-allocating",
					Position:    newPosition(pos),
				}
				analysis.Issues = append(analysis.Issues, issue)
			}
		}
	
	case "new":
		if len(call.Args) > 0 {
			typeStr := exprToString(call.Args[0])
			
			alloc := MemoryAllocation{
				Type:        "new",
				Description: "new(" + typeStr + ")",
				InLoop:      inLoop,
				Position:    newPosition(pos),
				Context:     extractContext(src, pos),
			}
			analysis.Allocations = append(analysis.Allocations, alloc)
			
			if inLoop {
				issue := AllocationIssue{
					Type:        "new_in_loop",
					Description: "new() called inside loop - consider pre-allocating",
					Position:    newPosition(pos),
				}
				analysis.Issues = append(analysis.Issues, issue)
			}
		}
	
	case "append":
		alloc := MemoryAllocation{
			Type:        "append",
			Description: "append() may cause reallocation",
			InLoop:      inLoop,
			Position:    newPosition(pos),
			Context:     extractContext(src, pos),
		}
		analysis.Allocations = append(analysis.Allocations, alloc)
		
		if inLoop && !hasPreallocation(file, call) {
			issue := AllocationIssue{
				Type:        "append_in_loop",
				Description: "append() in loop without pre-allocation - consider pre-allocating slice",
				Position:    newPosition(pos),
			}
			analysis.Issues = append(analysis.Issues, issue)
		}
	}
}

func analyzeMethodCall(sel *ast.SelectorExpr, call *ast.CallExpr, file *ast.File, fset *token.FileSet, src []byte, analysis *AllocationAnalysis) {
	// Check for common allocation patterns in method calls
	methodName := sel.Sel.Name
	
	// Check for strings.Builder inefficiencies
	if methodName == "WriteString" || methodName == "Write" {
		if ident, ok := sel.X.(*ast.Ident); ok {
			if isStringBuilderType(file, ident) && isInLoop(file, call) {
				// This is okay - strings.Builder is designed for this
				return
			}
		}
	}
}

func analyzeStringConcat(binExpr *ast.BinaryExpr, file *ast.File, fset *token.FileSet, src []byte, analysis *AllocationAnalysis) {
	// Check if this is string concatenation
	if !isStringType(binExpr.X) && !isStringType(binExpr.Y) {
		return
	}
	
	pos := fset.Position(binExpr.Pos())
	inLoop := isInLoop(file, binExpr)
	
	if inLoop {
		alloc := MemoryAllocation{
			Type:        "string_concat",
			Description: "String concatenation with +",
			InLoop:      true,
			Position:    newPosition(pos),
			Context:     extractContext(src, pos),
		}
		analysis.Allocations = append(analysis.Allocations, alloc)
		
		issue := AllocationIssue{
			Type:        "string_concat_in_loop",
			Description: "String concatenation in loop - use strings.Builder instead",
			Position:    newPosition(pos),
		}
		analysis.Issues = append(analysis.Issues, issue)
	}
}

func isStringType(expr ast.Expr) bool {
	// Simple heuristic - check for string literals or string-like identifiers
	switch e := expr.(type) {
	case *ast.BasicLit:
		return e.Kind == token.STRING
	case *ast.Ident:
		// This is a simplification - ideally we'd have type info
		return strings.Contains(strings.ToLower(e.Name), "str") || 
		       strings.Contains(strings.ToLower(e.Name), "msg") ||
		       strings.Contains(strings.ToLower(e.Name), "text")
	}
	return false
}

func isEscaping(file *ast.File, unary *ast.UnaryExpr) bool {
	// Simple escape analysis - if address is assigned or passed to function
	var escapes bool
	
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.AssignStmt:
			for _, rhs := range node.Rhs {
				if rhs == unary {
					escapes = true
					return false
				}
			}
		case *ast.CallExpr:
			for _, arg := range node.Args {
				if arg == unary {
					escapes = true
					return false
				}
			}
		case *ast.ReturnStmt:
			for _, result := range node.Results {
				if result == unary {
					escapes = true
					return false
				}
			}
		}
		return true
	})
	
	return escapes
}

func hasPreallocation(file *ast.File, appendCall *ast.CallExpr) bool {
	// Check if the slice being appended to was pre-allocated
	if len(appendCall.Args) == 0 {
		return false
	}
	
	// Get the slice being appended to
	sliceName := extractSliceName(appendCall.Args[0])
	if sliceName == "" {
		return false
	}
	
	// Look for make() call with capacity
	var hasCapacity bool
	ast.Inspect(file, func(n ast.Node) bool {
		if assign, ok := n.(*ast.AssignStmt); ok {
			for i, lhs := range assign.Lhs {
				if ident, ok := lhs.(*ast.Ident); ok && ident.Name == sliceName {
					if i < len(assign.Rhs) {
						if call, ok := assign.Rhs[i].(*ast.CallExpr); ok {
							if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == "make" {
								// Check if make has capacity argument
								if len(call.Args) >= 3 {
									hasCapacity = true
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
	
	return hasCapacity
}

func extractSliceName(expr ast.Expr) string {
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
}

func isStringBuilderType(file *ast.File, ident *ast.Ident) bool {
	// Check if identifier is of type strings.Builder
	var isBuilder bool
	
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.ValueSpec:
			for i, name := range node.Names {
				if name.Name == ident.Name {
					if node.Type != nil {
						if sel, ok := node.Type.(*ast.SelectorExpr); ok {
							if pkg, ok := sel.X.(*ast.Ident); ok {
								isBuilder = pkg.Name == "strings" && sel.Sel.Name == "Builder"
								return false
							}
						}
					} else if i < len(node.Values) {
						// Check initialization
						if comp, ok := node.Values[i].(*ast.CompositeLit); ok {
							if sel, ok := comp.Type.(*ast.SelectorExpr); ok {
								if pkg, ok := sel.X.(*ast.Ident); ok {
									isBuilder = pkg.Name == "strings" && sel.Sel.Name == "Builder"
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
	
	return isBuilder
}

func findAllocationPatterns(file *ast.File, fset *token.FileSet, src []byte, analysis *AllocationAnalysis) {
	// Look for interface{} allocations
	ast.Inspect(file, func(n ast.Node) bool {
		if callExpr, ok := n.(*ast.CallExpr); ok {
			// Check for fmt.Sprintf and similar
			if sel, ok := callExpr.Fun.(*ast.SelectorExpr); ok {
				if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "fmt" {
					if strings.HasPrefix(sel.Sel.Name, "Sprint") {
						pos := fset.Position(callExpr.Pos())
						alloc := MemoryAllocation{
							Type:        "fmt_sprintf",
							Description: "fmt." + sel.Sel.Name + " allocates for interface{} conversions",
							InLoop:      isInLoop(file, callExpr),
							Position:    newPosition(pos),
							Context:     extractContext(src, pos),
						}
						analysis.Allocations = append(analysis.Allocations, alloc)
					}
				}
			}
		}
		return true
	})
}