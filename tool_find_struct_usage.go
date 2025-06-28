package main

import (
	"go/ast"
	"go/token"
	"strings"
)

// Struct usage types
type StructUsage struct {
	File         string            `json:"file"`
	Literals     []StructLiteral   `json:"literals,omitempty"`
	FieldAccess  []FieldAccess     `json:"field_access,omitempty"`
	TypeUsage    []TypeUsage       `json:"type_usage,omitempty"`
}

type StructLiteral struct {
	Fields       []string `json:"fields_initialized"`
	IsComposite  bool     `json:"is_composite"`
	Position     Position `json:"position"`
}

type FieldAccess struct {
	Field    string   `json:"field"`
	Context  string   `json:"context"`
	Position Position `json:"position"`
}

type TypeUsage struct {
	Usage    string   `json:"usage"`
	Position Position `json:"position"`
}

func findStructUsage(dir string, structName string) ([]StructUsage, error) {
	var usages []StructUsage

	err := walkGoFiles(dir, func(path string, src []byte, file *ast.File, fset *token.FileSet) error {
		usage := StructUsage{
			File: path,
		}

		ast.Inspect(file, func(n ast.Node) bool {
			switch x := n.(type) {
			// Find struct literals
			case *ast.CompositeLit:
				if typeName := getTypeName(x.Type); typeName == structName {
					pos := fset.Position(x.Pos())
					lit := StructLiteral{
						IsComposite: len(x.Elts) > 0,
						Position:    newPosition(pos),
					}
					
					// Extract initialized fields
					for _, elt := range x.Elts {
						if kv, ok := elt.(*ast.KeyValueExpr); ok {
							if ident, ok := kv.Key.(*ast.Ident); ok {
								lit.Fields = append(lit.Fields, ident.Name)
							}
						}
					}
					
					usage.Literals = append(usage.Literals, lit)
				}

			// Find field access
			case *ast.SelectorExpr:
				if typeName := getTypeName(x.X); strings.Contains(typeName, structName) {
					pos := fset.Position(x.Sel.Pos())
					context := extractContext(src, pos)
					
					usage.FieldAccess = append(usage.FieldAccess, FieldAccess{
						Field:    x.Sel.Name,
						Context:  context,
						Position: newPosition(pos),
					})
				}

			// Find type usage in declarations
			case *ast.Field:
				if typeName := getTypeName(x.Type); typeName == structName {
					pos := fset.Position(x.Pos())
					usage.TypeUsage = append(usage.TypeUsage, TypeUsage{
						Usage:    "field",
						Position: newPosition(pos),
					})
				}
			}
			return true
		})

		if len(usage.Literals) > 0 || len(usage.FieldAccess) > 0 || len(usage.TypeUsage) > 0 {
			usages = append(usages, usage)
		}
		return nil
	})

	return usages, err
}

func getTypeName(expr ast.Expr) string {
	switch x := expr.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.StarExpr:
		return getTypeName(x.X)
	case *ast.SelectorExpr:
		return exprToString(x)
	}
	return ""
}