package main

import (
	"go/ast"
	"go/token"
	"strings"
)

// Dead code analysis types
type DeadCodeInfo struct {
	File           string         `json:"file"`
	UnusedVars     []UnusedItem   `json:"unused_vars,omitempty"`
	UnreachableCode []CodeLocation `json:"unreachable_code,omitempty"`
	DeadBranches   []CodeLocation `json:"dead_branches,omitempty"`
}

type UnusedItem struct {
	Name     string   `json:"name"`
	Type     string   `json:"type"`
	Position Position `json:"position"`
}

type CodeLocation struct {
	Description string   `json:"description"`
	Position    Position `json:"position"`
}

func findDeadCode(dir string) ([]DeadCodeInfo, error) {
	var deadCode []DeadCodeInfo

	err := walkGoFiles(dir, func(path string, src []byte, file *ast.File, fset *token.FileSet) error {
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}

		info := DeadCodeInfo{
			File: path,
		}

		// Track variable usage
		declaredVars := make(map[string]*ast.ValueSpec)
		usedVars := make(map[string]bool)

		// First pass: collect declared variables
		ast.Inspect(file, func(n ast.Node) bool {
			if genDecl, ok := n.(*ast.GenDecl); ok && genDecl.Tok == token.VAR {
				for _, spec := range genDecl.Specs {
					if valueSpec, ok := spec.(*ast.ValueSpec); ok {
						for _, name := range valueSpec.Names {
							if name.Name != "_" && !ast.IsExported(name.Name) {
								declaredVars[name.Name] = valueSpec
							}
						}
					}
				}
			}
			return true
		})

		// Second pass: track usage
		ast.Inspect(file, func(n ast.Node) bool {
			if ident, ok := n.(*ast.Ident); ok {
				usedVars[ident.Name] = true
			}
			return true
		})

		// Find unused variables
		for varName, valueSpec := range declaredVars {
			if !usedVars[varName] {
				for _, name := range valueSpec.Names {
					if name.Name == varName {
						pos := fset.Position(name.Pos())
						info.UnusedVars = append(info.UnusedVars, UnusedItem{
							Name:     varName,
							Type:     "variable",
							Position: newPosition(pos),
						})
					}
				}
			}
		}

		// Find unreachable code (simplified detection)
		ast.Inspect(file, func(n ast.Node) bool {
			if blockStmt, ok := n.(*ast.BlockStmt); ok {
				for i, stmt := range blockStmt.List {
					if _, ok := stmt.(*ast.ReturnStmt); ok && i < len(blockStmt.List)-1 {
						pos := fset.Position(blockStmt.List[i+1].Pos())
						info.UnreachableCode = append(info.UnreachableCode, CodeLocation{
							Description: "Code after return statement",
							Position:    newPosition(pos),
						})
					}
				}
			}
			return true
		})

		if len(info.UnusedVars) > 0 || len(info.UnreachableCode) > 0 || len(info.DeadBranches) > 0 {
			deadCode = append(deadCode, info)
		}
		return nil
	})

	return deadCode, err
}