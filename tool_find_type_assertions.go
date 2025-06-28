package main

import (
	"go/ast"
	"go/token"
)

type TypeAssertion struct {
	Expression string   `json:"expression"`
	TargetType string   `json:"target_type"`
	HasOkCheck bool     `json:"has_ok_check"`
	Position   Position `json:"position"`
	Context    string   `json:"context"`
}

type TypeAssertionAnalysis struct {
	Assertions []TypeAssertion       `json:"assertions"`
	Issues     []TypeAssertionIssue  `json:"issues"`
}

type TypeAssertionIssue struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Position    Position `json:"position"`
}

func findTypeAssertions(dir string) (*TypeAssertionAnalysis, error) {
	analysis := &TypeAssertionAnalysis{
		Assertions: []TypeAssertion{},
		Issues:     []TypeAssertionIssue{},
	}

	err := walkGoFiles(dir, func(path string, src []byte, file *ast.File, fset *token.FileSet) error {
		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.TypeAssertExpr:
				pos := fset.Position(node.Pos())
				assertion := TypeAssertion{
					Expression: exprToString(node.X),
					TargetType: exprToString(node.Type),
					HasOkCheck: false, // Will be updated if in assignment
					Position:   newPosition(pos),
					Context:    extractContext(src, pos),
				}
				
				// Check if this assertion is used with ok check
				hasOk := isUsedWithOkCheck(file, node)
				assertion.HasOkCheck = hasOk
				
				analysis.Assertions = append(analysis.Assertions, assertion)
				
				// Report issue if no ok check
				if !hasOk && !isInSafeContext(file, node) {
					issue := TypeAssertionIssue{
						Type:        "unsafe_type_assertion",
						Description: "Type assertion without ok check may panic",
						Position:    newPosition(pos),
					}
					analysis.Issues = append(analysis.Issues, issue)
				}
			
			case *ast.TypeSwitchStmt:
				// Analyze type switch
				analyzeTypeSwitch(node, fset, src, analysis)
			}
			return true
		})
		
		return nil
	})

	return analysis, err
}

func isUsedWithOkCheck(file *ast.File, assertion *ast.TypeAssertExpr) bool {
	var hasOk bool
	
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.AssignStmt:
			// Check if assertion is on RHS and has 2 LHS values
			for _, rhs := range node.Rhs {
				if rhs == assertion && len(node.Lhs) >= 2 {
					hasOk = true
					return false
				}
			}
		case *ast.ValueSpec:
			// Check in var declarations
			for _, value := range node.Values {
				if value == assertion && len(node.Names) >= 2 {
					hasOk = true
					return false
				}
			}
		}
		return true
	})
	
	return hasOk
}

func isInSafeContext(file *ast.File, assertion *ast.TypeAssertExpr) bool {
	// Check if assertion is in a context where panic is acceptable
	var isSafe bool
	
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.FuncDecl:
			if containsNode(node, assertion) {
				// Check if function has recover
				hasRecover := false
				ast.Inspect(node, func(inner ast.Node) bool {
					if call, ok := inner.(*ast.CallExpr); ok {
						if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == "recover" {
							hasRecover = true
							return false
						}
					}
					return true
				})
				if hasRecover {
					isSafe = true
					return false
				}
			}
		case *ast.IfStmt:
			// Check if assertion is guarded by type check
			if containsNode(node, assertion) && isTypeCheckCondition(node.Cond) {
				isSafe = true
				return false
			}
		}
		return true
	})
	
	return isSafe
}

func isTypeCheckCondition(expr ast.Expr) bool {
	// Check for _, ok := x.(Type) pattern in condition
	switch e := expr.(type) {
	case *ast.BinaryExpr:
		// Check for ok == true or similar
		if ident, ok := e.X.(*ast.Ident); ok && ident.Name == "ok" {
			return true
		}
		if ident, ok := e.Y.(*ast.Ident); ok && ident.Name == "ok" {
			return true
		}
	case *ast.Ident:
		// Direct ok check
		return e.Name == "ok"
	}
	return false
}

func analyzeTypeSwitch(typeSwitch *ast.TypeSwitchStmt, fset *token.FileSet, src []byte, analysis *TypeAssertionAnalysis) {
	pos := fset.Position(typeSwitch.Pos())
	
	var expr string
	hasDefault := false
	caseCount := 0
	
	// Extract the expression being switched on
	switch assign := typeSwitch.Assign.(type) {
	case *ast.AssignStmt:
		if len(assign.Rhs) > 0 {
			if typeAssert, ok := assign.Rhs[0].(*ast.TypeAssertExpr); ok {
				expr = exprToString(typeAssert.X)
			}
		}
	case *ast.ExprStmt:
		if typeAssert, ok := assign.X.(*ast.TypeAssertExpr); ok {
			expr = exprToString(typeAssert.X)
		}
	}
	
	// Count cases and check for default
	for _, clause := range typeSwitch.Body.List {
		if cc, ok := clause.(*ast.CaseClause); ok {
			if cc.List == nil {
				hasDefault = true
			} else {
				caseCount += len(cc.List)
			}
		}
	}
	
	// Type switches are generally safe, but we can note them
	assertion := TypeAssertion{
		Expression: expr,
		TargetType: "type switch",
		HasOkCheck: true, // Type switches are inherently safe
		Position:   newPosition(pos),
		Context:    extractContext(src, pos),
	}
	analysis.Assertions = append(analysis.Assertions, assertion)
	
	// Check for single-case type switch
	if caseCount == 1 && !hasDefault {
		issue := TypeAssertionIssue{
			Type:        "single_case_type_switch",
			Description: "Type switch with single case - consider using type assertion instead",
			Position:    newPosition(pos),
		}
		analysis.Issues = append(analysis.Issues, issue)
	}
}