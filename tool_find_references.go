package main

import (
	"go/ast"
	"go/token"
)

type Reference struct {
	Context  string   `json:"context"`
	Kind     string   `json:"kind"`
	Position Position `json:"position"`
}

func findReferences(dir string, symbol string) ([]Reference, error) {
	var refs []Reference

	err := walkGoFiles(dir, func(path string, src []byte, file *ast.File, fset *token.FileSet) error {

		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.Ident:
				if node.Name == symbol {
					pos := fset.Position(node.Pos())
					kind := identifyReferenceKind(node)
					context := extractContext(src, pos)
					
					refs = append(refs, Reference{
						Context:  context,
						Kind:     kind,
						Position: newPosition(pos),
					})
				}

			case *ast.SelectorExpr:
				if node.Sel.Name == symbol {
					pos := fset.Position(node.Sel.Pos())
					context := extractContext(src, pos)
					
					refs = append(refs, Reference{
						Context:  context,
						Kind:     "selector",
						Position: newPosition(pos),
					})
				}
			}
			return true
		})

		return nil
	})

	return refs, err
}

func identifyReferenceKind(ident *ast.Ident) string {
	return "identifier"
}