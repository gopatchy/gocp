package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"
)

type TypeInfo struct {
	Name      string                 `json:"name"`
	Package   string                 `json:"package"`
	Kind      string                 `json:"kind"`
	Position  Position               `json:"position"`
	Fields    []FieldInfo            `json:"fields,omitempty"`
	Methods   []MethodInfo           `json:"methods,omitempty"`
	Embedded  []string               `json:"embedded,omitempty"`
	Interface []MethodInfo           `json:"interface,omitempty"`
	Underlying string                `json:"underlying,omitempty"`
}

type FieldInfo struct {
	Name     string   `json:"name"`
	Type     string   `json:"type"`
	Tag      string   `json:"tag,omitempty"`
	Exported bool     `json:"exported"`
	Position Position `json:"position"`
}

type MethodInfo struct {
	Name      string   `json:"name"`
	Signature string   `json:"signature"`
	Receiver  string   `json:"receiver,omitempty"`
	Exported  bool     `json:"exported"`
	Position  Position `json:"position"`
}

func getTypeInfo(dir string, typeName string) (*TypeInfo, error) {
	var result *TypeInfo
	
	err := walkGoFiles(dir, func(path string, src []byte, file *ast.File, fset *token.FileSet) error {
		if result != nil {
			return nil
		}

		ast.Inspect(file, func(n ast.Node) bool {
			if result != nil {
				return false
			}

			switch decl := n.(type) {
			case *ast.GenDecl:
				for _, spec := range decl.Specs {
					if ts, ok := spec.(*ast.TypeSpec); ok && ts.Name.Name == typeName {
						pos := fset.Position(ts.Pos())
						info := &TypeInfo{
							Name:     typeName,
							Package:  file.Name.Name,
							Position: newPosition(pos),
						}

						switch t := ts.Type.(type) {
						case *ast.StructType:
							info.Kind = "struct"
							info.Fields = extractFields(t, fset)
							info.Embedded = extractEmbedded(t)

						case *ast.InterfaceType:
							info.Kind = "interface"
							info.Interface = extractInterfaceMethods(t, fset)

						case *ast.Ident:
							info.Kind = "alias"
							info.Underlying = t.Name

						case *ast.SelectorExpr:
							info.Kind = "alias"
							if x, ok := t.X.(*ast.Ident); ok {
								info.Underlying = x.Name + "." + t.Sel.Name
							}

						default:
							info.Kind = "other"
						}

						info.Methods = extractMethods(file, typeName, fset)
						result = info
						return false
					}
				}
			}
			return true
		})

		return nil
	})

	if result == nil && err == nil {
		return nil, fmt.Errorf("type %s not found", typeName)
	}

	return result, err
}

func extractFields(st *ast.StructType, fset *token.FileSet) []FieldInfo {
	var fields []FieldInfo
	
	for _, field := range st.Fields.List {
		fieldType := exprToString(field.Type)
		tag := ""
		if field.Tag != nil {
			tag = field.Tag.Value
		}

		if len(field.Names) == 0 {
			pos := fset.Position(field.Pos())
			fields = append(fields, FieldInfo{
				Name:     "",
				Type:     fieldType,
				Tag:      tag,
				Exported: true,
				Position: newPosition(pos),
			})
		} else {
			for _, name := range field.Names {
				pos := fset.Position(name.Pos())
				fields = append(fields, FieldInfo{
					Name:     name.Name,
					Type:     fieldType,
					Tag:      tag,
					Exported: ast.IsExported(name.Name),
					Position: newPosition(pos),
				})
			}
		}
	}
	
	return fields
}

func extractEmbedded(st *ast.StructType) []string {
	var embedded []string
	
	for _, field := range st.Fields.List {
		if len(field.Names) == 0 {
			embedded = append(embedded, exprToString(field.Type))
		}
	}
	
	return embedded
}

func extractInterfaceMethods(it *ast.InterfaceType, fset *token.FileSet) []MethodInfo {
	var methods []MethodInfo
	
	for _, method := range it.Methods.List {
		if len(method.Names) > 0 {
			for _, name := range method.Names {
				sig := exprToString(method.Type)
				pos := fset.Position(name.Pos())
				methods = append(methods, MethodInfo{
					Name:      name.Name,
					Signature: sig,
					Exported:  ast.IsExported(name.Name),
					Position:  newPosition(pos),
				})
			}
		}
	}
	
	return methods
}

func extractMethods(file *ast.File, typeName string, fset *token.FileSet) []MethodInfo {
	var methods []MethodInfo
	
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv != nil {
			for _, recv := range fn.Recv.List {
				recvType := exprToString(recv.Type)
				if strings.Contains(recvType, typeName) {
					sig := funcSignature(fn.Type)
					pos := fset.Position(fn.Name.Pos())
					methods = append(methods, MethodInfo{
						Name:      fn.Name.Name,
						Signature: sig,
						Receiver:  recvType,
						Exported:  ast.IsExported(fn.Name.Name),
						Position:  newPosition(pos),
					})
				}
			}
		}
	}
	
	return methods
}