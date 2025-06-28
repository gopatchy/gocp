package main

import (
	"go/ast"
	"go/token"
)

// Embedding analysis types
type EmbeddingInfo struct {
	File         string              `json:"file"`
	Structs      []StructEmbedding   `json:"structs,omitempty"`
	Interfaces   []InterfaceEmbedding `json:"interfaces,omitempty"`
}

type StructEmbedding struct {
	Name        string   `json:"name"`
	Embedded    []string `json:"embedded"`
	Methods     []string `json:"promoted_methods"`
	Position    Position `json:"position"`
}

type InterfaceEmbedding struct {
	Name        string   `json:"name"`
	Embedded    []string `json:"embedded"`
	Methods     []string `json:"methods"`
	Position    Position `json:"position"`
}

func analyzeEmbedding(dir string) ([]EmbeddingInfo, error) {
	var embedding []EmbeddingInfo

	err := walkGoFiles(dir, func(path string, src []byte, file *ast.File, fset *token.FileSet) error {
		info := EmbeddingInfo{
			File: path,
		}

		ast.Inspect(file, func(n ast.Node) bool {
			switch decl := n.(type) {
			case *ast.GenDecl:
				for _, spec := range decl.Specs {
					if ts, ok := spec.(*ast.TypeSpec); ok {
						if st, ok := ts.Type.(*ast.StructType); ok {
							pos := fset.Position(ts.Pos())
							structEmb := StructEmbedding{
								Name:     ts.Name.Name,
								Position: newPosition(pos),
							}

							for _, field := range st.Fields.List {
								if len(field.Names) == 0 {
									structEmb.Embedded = append(structEmb.Embedded, exprToString(field.Type))
								}
							}

							if len(structEmb.Embedded) > 0 {
								info.Structs = append(info.Structs, structEmb)
							}
						}

						if it, ok := ts.Type.(*ast.InterfaceType); ok {
							pos := fset.Position(ts.Pos())
							ifaceEmb := InterfaceEmbedding{
								Name:     ts.Name.Name,
								Position: newPosition(pos),
							}

							for _, method := range it.Methods.List {
								if len(method.Names) == 0 {
									ifaceEmb.Embedded = append(ifaceEmb.Embedded, exprToString(method.Type))
								} else {
									for _, name := range method.Names {
										ifaceEmb.Methods = append(ifaceEmb.Methods, name.Name)
									}
								}
							}

							if len(ifaceEmb.Embedded) > 0 || len(ifaceEmb.Methods) > 0 {
								info.Interfaces = append(info.Interfaces, ifaceEmb)
							}
						}
					}
				}
			}
			return true
		})

		if len(info.Structs) > 0 || len(info.Interfaces) > 0 {
			embedding = append(embedding, info)
		}
		return nil
	})

	return embedding, err
}