package main

import (
	"go/ast"
	"go/token"
)

// Generic types
type GenericInfo struct {
	Name        string       `json:"name"`
	Kind        string       `json:"kind"`
	Package     string       `json:"package"`
	Position    Position     `json:"position"`
	TypeParams  []TypeParam  `json:"type_params"`
	Instances   []Instance   `json:"instances,omitempty"`
}

type TypeParam struct {
	Name       string   `json:"name"`
	Constraint string   `json:"constraint"`
	Position   Position `json:"position"`
}

type Instance struct {
	Types    []string `json:"types"`
	Position Position `json:"position"`
}

func findGenerics(dir string) ([]GenericInfo, error) {
	var generics []GenericInfo

	err := walkGoFiles(dir, func(path string, src []byte, file *ast.File, fset *token.FileSet) error {
		ast.Inspect(file, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.GenDecl:
				for _, spec := range x.Specs {
					if ts, ok := spec.(*ast.TypeSpec); ok && ts.TypeParams != nil {
						pos := fset.Position(ts.Pos())
						info := GenericInfo{
							Name:     ts.Name.Name,
							Kind:     "type",
							Package:  file.Name.Name,
							Position: newPosition(pos),
						}

						// Extract type parameters
						for _, param := range ts.TypeParams.List {
							for _, name := range param.Names {
								namePos := fset.Position(name.Pos())
								tp := TypeParam{
									Name:     name.Name,
									Position: newPosition(namePos),
								}
								if param.Type != nil {
									tp.Constraint = exprToString(param.Type)
								}
								info.TypeParams = append(info.TypeParams, tp)
							}
						}

						generics = append(generics, info)
					}
				}

			case *ast.FuncDecl:
				if x.Type.TypeParams != nil {
					pos := fset.Position(x.Pos())
					info := GenericInfo{
						Name:     x.Name.Name,
						Kind:     "function",
						Package:  file.Name.Name,
						Position: newPosition(pos),
					}

					// Extract type parameters
					for _, param := range x.Type.TypeParams.List {
						for _, name := range param.Names {
							namePos := fset.Position(name.Pos())
							tp := TypeParam{
								Name:     name.Name,
								Position: newPosition(namePos),
							}
							if param.Type != nil {
								tp.Constraint = exprToString(param.Type)
							}
							info.TypeParams = append(info.TypeParams, tp)
						}
					}

					generics = append(generics, info)
				}
			}
			return true
		})
		return nil
	})

	return generics, err
}