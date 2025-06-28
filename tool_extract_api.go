package main

import (
	"go/ast"
	"go/token"
	"strings"
)

// API analysis types
type ApiInfo struct {
	Package   string        `json:"package"`
	Functions []ApiFunction `json:"functions"`
	Types     []ApiType     `json:"types"`
	Constants []ApiConstant `json:"constants"`
	Variables []ApiVariable `json:"variables"`
}

type ApiFunction struct {
	Name      string   `json:"name"`
	Signature string   `json:"signature"`
	Doc       string   `json:"doc,omitempty"`
	Position  Position `json:"position"`
}

type ApiType struct {
	Name     string   `json:"name"`
	Kind     string   `json:"kind"`
	Doc      string   `json:"doc,omitempty"`
	Position Position `json:"position"`
}

type ApiConstant struct {
	Name     string   `json:"name"`
	Type     string   `json:"type"`
	Value    string   `json:"value,omitempty"`
	Doc      string   `json:"doc,omitempty"`
	Position Position `json:"position"`
}

type ApiVariable struct {
	Name     string   `json:"name"`
	Type     string   `json:"type"`
	Doc      string   `json:"doc,omitempty"`
	Position Position `json:"position"`
}

func extractApi(dir string) ([]ApiInfo, error) {
	var apis []ApiInfo

	err := walkGoFiles(dir, func(path string, src []byte, file *ast.File, fset *token.FileSet) error {
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}

		api := ApiInfo{
			Package: file.Name.Name,
		}

		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if ast.IsExported(d.Name.Name) {
					pos := fset.Position(d.Pos())
					api.Functions = append(api.Functions, ApiFunction{
						Name:      d.Name.Name,
						Signature: funcSignature(d.Type),
						Doc:       extractDocString(d.Doc),
						Position:  newPosition(pos),
					})
				}

			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						if ast.IsExported(s.Name.Name) {
							pos := fset.Position(s.Pos())
							kind := "type"
							switch s.Type.(type) {
							case *ast.StructType:
								kind = "struct"
							case *ast.InterfaceType:
								kind = "interface"
							}
							api.Types = append(api.Types, ApiType{
								Name:     s.Name.Name,
								Kind:     kind,
								Doc:      extractDocString(d.Doc),
								Position: newPosition(pos),
							})
						}

					case *ast.ValueSpec:
						for _, name := range s.Names {
							if ast.IsExported(name.Name) {
								pos := fset.Position(name.Pos())
								if d.Tok == token.CONST {
									value := ""
									if len(s.Values) > 0 {
										value = exprToString(s.Values[0])
									}
									api.Constants = append(api.Constants, ApiConstant{
										Name:     name.Name,
										Type:     exprToString(s.Type),
										Value:    value,
										Doc:      extractDocString(d.Doc),
										Position: newPosition(pos),
									})
								} else {
									api.Variables = append(api.Variables, ApiVariable{
										Name:     name.Name,
										Type:     exprToString(s.Type),
										Doc:      extractDocString(d.Doc),
										Position: newPosition(pos),
									})
								}
							}
						}
					}
				}
			}
		}

		if len(api.Functions) > 0 || len(api.Types) > 0 || len(api.Constants) > 0 || len(api.Variables) > 0 {
			apis = append(apis, api)
		}
		return nil
	})

	return apis, err
}