package main

import (
	"fmt"
	"go/ast"
	"go/token"
)

// Performance inefficiency types
type InefficiencyInfo struct {
	File           string               `json:"file"`
	StringConcat   []InefficiencyItem   `json:"string_concat,omitempty"`
	Conversions    []InefficiencyItem   `json:"unnecessary_conversions,omitempty"`
	Allocations    []InefficiencyItem   `json:"potential_allocations,omitempty"`
}

type InefficiencyItem struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Suggestion  string   `json:"suggestion"`
	Position    Position `json:"position"`
}

func findInefficiencies(dir string) ([]InefficiencyInfo, error) {
	var inefficiencies []InefficiencyInfo

	err := walkGoFiles(dir, func(path string, src []byte, file *ast.File, fset *token.FileSet) error {
		info := InefficiencyInfo{
			File: path,
		}

		ast.Inspect(file, func(n ast.Node) bool {
			// Find string concatenation in loops
			if forStmt, ok := n.(*ast.ForStmt); ok {
				ast.Inspect(forStmt.Body, func(inner ast.Node) bool {
					if binExpr, ok := inner.(*ast.BinaryExpr); ok && binExpr.Op == token.ADD {
						if isStringType(binExpr.X) || isStringType(binExpr.Y) {
							pos := fset.Position(binExpr.Pos())
							info.StringConcat = append(info.StringConcat, InefficiencyItem{
								Type:        "string_concatenation_in_loop",
								Description: "String concatenation in loop can be inefficient",
								Suggestion:  "Consider using strings.Builder",
								Position:    newPosition(pos),
							})
						}
					}
					return true
				})
			}

			// Find unnecessary type conversions
			if callExpr, ok := n.(*ast.CallExpr); ok {
				if len(callExpr.Args) == 1 {
					if ident, ok := callExpr.Fun.(*ast.Ident); ok {
						argType := getExprType(callExpr.Args[0])
						if ident.Name == argType {
							pos := fset.Position(callExpr.Pos())
							info.Conversions = append(info.Conversions, InefficiencyItem{
								Type:        "unnecessary_conversion",
								Description: fmt.Sprintf("Unnecessary conversion to %s", ident.Name),
								Suggestion:  "Remove unnecessary type conversion",
								Position:    newPosition(pos),
							})
						}
					}
				}
			}

			return true
		})

		if len(info.StringConcat) > 0 || len(info.Conversions) > 0 || len(info.Allocations) > 0 {
			inefficiencies = append(inefficiencies, info)
		}
		return nil
	})

	return inefficiencies, err
}

func isStringType(expr ast.Expr) bool {
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name == "string"
	}
	return false
}

func getExprType(expr ast.Expr) string {
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name
	}
	return "unknown"
}