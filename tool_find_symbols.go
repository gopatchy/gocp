package main

import (
	"go/ast"
	"go/token"
	"strings"
)

type Symbol struct {
	Name     string   `json:"name"`
	Type     string   `json:"type"`
	Package  string   `json:"package"`
	Exported bool     `json:"exported"`
	Position Position `json:"position"`
}

func findSymbols(dir string, pattern string) ([]Symbol, error) {
	var symbols []Symbol

	err := walkGoFiles(dir, func(path string, src []byte, file *ast.File, fset *token.FileSet) error {
		if strings.HasSuffix(path, "_test.go") && !strings.Contains(pattern, "Test") {
			return nil
		}

		pkgName := file.Name.Name

		ast.Inspect(file, func(n ast.Node) bool {
			switch decl := n.(type) {
			case *ast.FuncDecl:
				name := decl.Name.Name
				if matchesPattern(name, pattern) {
					pos := fset.Position(decl.Pos())
					symbols = append(symbols, Symbol{
						Name:     name,
						Type:     "function",
						Package:  pkgName,
						Exported: ast.IsExported(name),
						Position: newPosition(pos),
					})
				}

			case *ast.GenDecl:
				for _, spec := range decl.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						name := s.Name.Name
						if matchesPattern(name, pattern) {
							pos := fset.Position(s.Pos())
							kind := "type"
							switch s.Type.(type) {
							case *ast.InterfaceType:
								kind = "interface"
							case *ast.StructType:
								kind = "struct"
							}
							symbols = append(symbols, Symbol{
								Name:     name,
								Type:     kind,
								Package:  pkgName,
								Exported: ast.IsExported(name),
								Position: newPosition(pos),
							})
						}

					case *ast.ValueSpec:
						for _, name := range s.Names {
							if matchesPattern(name.Name, pattern) {
								pos := fset.Position(name.Pos())
								kind := "variable"
								if decl.Tok == token.CONST {
									kind = "constant"
								}
								symbols = append(symbols, Symbol{
									Name:     name.Name,
									Type:     kind,
									Package:  pkgName,
									Exported: ast.IsExported(name.Name),
									Position: newPosition(pos),
								})
							}
						}
					}
				}
			}
			return true
		})

		return nil
	})

	return symbols, err
}

func matchesPattern(name, pattern string) bool {
	if pattern == "" {
		return true
	}
	pattern = strings.ToLower(pattern)
	name = strings.ToLower(name)
	return strings.Contains(name, pattern)
}