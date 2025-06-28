package main

import (
	"go/ast"
	"go/token"
)

// Interface analysis types
type InterfaceInfo struct {
	Name           string               `json:"name"`
	Package        string               `json:"package"`
	Position       Position             `json:"position"`
	Methods        []MethodInfo         `json:"methods"`
	Implementations []ImplementationType `json:"implementations,omitempty"`
}

type ImplementationType struct {
	Type     string   `json:"type"`
	Package  string   `json:"package"`
	Position Position `json:"position"`
}

func extractInterfaces(dir string, interfaceName string) ([]InterfaceInfo, error) {
	var interfaces []InterfaceInfo
	interfaceMap := make(map[string]*InterfaceInfo)

	// First pass: collect all interfaces
	err := walkGoFiles(dir, func(path string, src []byte, file *ast.File, fset *token.FileSet) error {
		ast.Inspect(file, func(n ast.Node) bool {
			if genDecl, ok := n.(*ast.GenDecl); ok {
				for _, spec := range genDecl.Specs {
					if typeSpec, ok := spec.(*ast.TypeSpec); ok {
						if iface, ok := typeSpec.Type.(*ast.InterfaceType); ok {
							name := typeSpec.Name.Name
							if interfaceName == "" || name == interfaceName {
								pos := fset.Position(typeSpec.Pos())
								info := &InterfaceInfo{
									Name:     name,
									Package:  file.Name.Name,
									Position: newPosition(pos),
									Methods:  extractInterfaceMethods(iface, fset),
								}
								interfaceMap[name] = info
							}
						}
					}
				}
			}
			return true
		})
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Second pass: find implementations
	if interfaceName != "" {
		iface, exists := interfaceMap[interfaceName]
		if exists {
			err = walkGoFiles(dir, func(path string, src []byte, file *ast.File, fset *token.FileSet) error {
				// Collect all types with methods
				types := make(map[string][]string)
				
				for _, decl := range file.Decls {
					if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv != nil {
						for _, recv := range fn.Recv.List {
							typeName := getTypeName(recv.Type)
							types[typeName] = append(types[typeName], fn.Name.Name)
						}
					}
				}

				// Check if any type implements the interface
				for typeName, methods := range types {
					if implementsInterface(methods, iface.Methods) {
						// Find type declaration
						ast.Inspect(file, func(n ast.Node) bool {
							if genDecl, ok := n.(*ast.GenDecl); ok {
								for _, spec := range genDecl.Specs {
									if typeSpec, ok := spec.(*ast.TypeSpec); ok && typeSpec.Name.Name == typeName {
										pos := fset.Position(typeSpec.Pos())
										iface.Implementations = append(iface.Implementations, ImplementationType{
											Type:     typeName,
											Package:  file.Name.Name,
											Position: newPosition(pos),
										})
									}
								}
							}
							return true
						})
					}
				}
				return nil
			})
		}
	}

	// Convert map to slice
	for _, iface := range interfaceMap {
		interfaces = append(interfaces, *iface)
	}

	return interfaces, err
}

func implementsInterface(methods []string, interfaceMethods []MethodInfo) bool {
	for _, im := range interfaceMethods {
		found := false
		for _, m := range methods {
			if m == im.Name {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}