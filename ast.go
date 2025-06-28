package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type Symbol struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Package  string `json:"package"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	Exported bool   `json:"exported"`
}

type TypeInfo struct {
	Name      string                 `json:"name"`
	Package   string                 `json:"package"`
	File      string                 `json:"file"`
	Line      int                    `json:"line"`
	Kind      string                 `json:"kind"`
	Fields    []FieldInfo            `json:"fields,omitempty"`
	Methods   []MethodInfo           `json:"methods,omitempty"`
	Embedded  []string               `json:"embedded,omitempty"`
	Interface []MethodInfo           `json:"interface,omitempty"`
	Underlying string                `json:"underlying,omitempty"`
}

type FieldInfo struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Tag      string `json:"tag,omitempty"`
	Exported bool   `json:"exported"`
}

type MethodInfo struct {
	Name      string `json:"name"`
	Signature string `json:"signature"`
	Receiver  string `json:"receiver,omitempty"`
	Exported  bool   `json:"exported"`
}

type Reference struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	Context  string `json:"context"`
	Kind     string `json:"kind"`
}

type Package struct {
	ImportPath string   `json:"import_path"`
	Name       string   `json:"name"`
	Dir        string   `json:"dir"`
	GoFiles    []string `json:"go_files"`
	Imports    []string `json:"imports"`
}

type fileVisitor func(path string, src []byte, file *ast.File, fset *token.FileSet) error

func walkGoFiles(dir string, visitor fileVisitor) error {
	fset := token.NewFileSet()

	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.Contains(path, "vendor/") {
			return nil
		}

		src, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		file, err := parser.ParseFile(fset, path, src, parser.ParseComments)
		if err != nil {
			return nil
		}

		return visitor(path, src, file, fset)
	})
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
						File:     path,
						Line:     pos.Line,
						Column:   pos.Column,
						Exported: ast.IsExported(name),
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
								File:     path,
								Line:     pos.Line,
								Column:   pos.Column,
								Exported: ast.IsExported(name),
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
									File:     path,
									Line:     pos.Line,
									Column:   pos.Column,
									Exported: ast.IsExported(name.Name),
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
							Name:    typeName,
							Package: file.Name.Name,
							File:    path,
							Line:    pos.Line,
						}

						switch t := ts.Type.(type) {
						case *ast.StructType:
							info.Kind = "struct"
							info.Fields = extractFields(t)
							info.Embedded = extractEmbedded(t)

						case *ast.InterfaceType:
							info.Kind = "interface"
							info.Interface = extractInterfaceMethods(t)

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

						info.Methods = extractMethods(file, typeName)
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
						File:    path,
						Line:    pos.Line,
						Column:  pos.Column,
						Context: context,
						Kind:    kind,
					})
				}

			case *ast.SelectorExpr:
				if node.Sel.Name == symbol {
					pos := fset.Position(node.Sel.Pos())
					context := extractContext(src, pos)
					
					refs = append(refs, Reference{
						File:    path,
						Line:    pos.Line,
						Column:  pos.Column,
						Context: context,
						Kind:    "selector",
					})
				}
			}
			return true
		})

		return nil
	})

	return refs, err
}

func listPackages(dir string, includeTests bool) ([]Package, error) {
	packages := make(map[string]*Package)

	err := walkGoFiles(dir, func(path string, src []byte, file *ast.File, fset *token.FileSet) error {
		// Skip test files if not requested
		if !includeTests && strings.HasSuffix(path, "_test.go") {
			return nil
		}

		pkgDir := filepath.Dir(path)
		
		// Initialize package if not seen before
		if _, exists := packages[pkgDir]; !exists {
			importPath := strings.TrimPrefix(pkgDir, dir)
			importPath = strings.TrimPrefix(importPath, "/")
			if importPath == "" {
				importPath = "."
			}

			packages[pkgDir] = &Package{
				ImportPath: importPath,
				Name:       file.Name.Name,
				Dir:        pkgDir,
				GoFiles:    []string{},
				Imports:    []string{},
			}
		}

		// Add file to package
		fileName := filepath.Base(path)
		packages[pkgDir].GoFiles = append(packages[pkgDir].GoFiles, fileName)
		
		// Collect unique imports
		imports := make(map[string]bool)
		for _, imp := range file.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)
			imports[importPath] = true
		}
		
		// Merge imports into package
		existingImports := make(map[string]bool)
		for _, imp := range packages[pkgDir].Imports {
			existingImports[imp] = true
		}
		for imp := range imports {
			if !existingImports[imp] {
				packages[pkgDir].Imports = append(packages[pkgDir].Imports, imp)
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	var result []Package
	for _, pkg := range packages {
		result = append(result, *pkg)
	}

	return result, nil
}

func matchesPattern(name, pattern string) bool {
	if pattern == "" {
		return true
	}
	pattern = strings.ToLower(pattern)
	name = strings.ToLower(name)
	return strings.Contains(name, pattern)
}

func extractFields(st *ast.StructType) []FieldInfo {
	var fields []FieldInfo
	
	for _, field := range st.Fields.List {
		fieldType := exprToString(field.Type)
		tag := ""
		if field.Tag != nil {
			tag = field.Tag.Value
		}

		if len(field.Names) == 0 {
			fields = append(fields, FieldInfo{
				Name:     "",
				Type:     fieldType,
				Tag:      tag,
				Exported: true,
			})
		} else {
			for _, name := range field.Names {
				fields = append(fields, FieldInfo{
					Name:     name.Name,
					Type:     fieldType,
					Tag:      tag,
					Exported: ast.IsExported(name.Name),
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

func extractInterfaceMethods(it *ast.InterfaceType) []MethodInfo {
	var methods []MethodInfo
	
	for _, method := range it.Methods.List {
		if len(method.Names) > 0 {
			for _, name := range method.Names {
				sig := exprToString(method.Type)
				methods = append(methods, MethodInfo{
					Name:      name.Name,
					Signature: sig,
					Exported:  ast.IsExported(name.Name),
				})
			}
		}
	}
	
	return methods
}

func extractMethods(file *ast.File, typeName string) []MethodInfo {
	var methods []MethodInfo
	
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv != nil {
			for _, recv := range fn.Recv.List {
				recvType := exprToString(recv.Type)
				if strings.Contains(recvType, typeName) {
					sig := funcSignature(fn.Type)
					methods = append(methods, MethodInfo{
						Name:      fn.Name.Name,
						Signature: sig,
						Receiver:  recvType,
						Exported:  ast.IsExported(fn.Name.Name),
					})
				}
			}
		}
	}
	
	return methods
}

func exprToString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return "*" + exprToString(e.X)
	case *ast.SelectorExpr:
		return exprToString(e.X) + "." + e.Sel.Name
	case *ast.ArrayType:
		if e.Len == nil {
			return "[]" + exprToString(e.Elt)
		}
		return "[" + exprToString(e.Len) + "]" + exprToString(e.Elt)
	case *ast.MapType:
		return "map[" + exprToString(e.Key) + "]" + exprToString(e.Value)
	case *ast.InterfaceType:
		if len(e.Methods.List) == 0 {
			return "interface{}"
		}
		return "interface{...}"
	case *ast.FuncType:
		return funcSignature(e)
	case *ast.ChanType:
		switch e.Dir {
		case ast.SEND:
			return "chan<- " + exprToString(e.Value)
		case ast.RECV:
			return "<-chan " + exprToString(e.Value)
		default:
			return "chan " + exprToString(e.Value)
		}
	case *ast.BasicLit:
		return e.Value
	default:
		return fmt.Sprintf("%T", expr)
	}
}

func funcSignature(fn *ast.FuncType) string {
	params := fieldListToString(fn.Params)
	results := fieldListToString(fn.Results)
	
	if results == "" {
		return fmt.Sprintf("func(%s)", params)
	}
	return fmt.Sprintf("func(%s) %s", params, results)
}

func fieldListToString(fl *ast.FieldList) string {
	if fl == nil || len(fl.List) == 0 {
		return ""
	}
	
	var parts []string
	for _, field := range fl.List {
		fieldType := exprToString(field.Type)
		if len(field.Names) == 0 {
			parts = append(parts, fieldType)
		} else {
			for _, name := range field.Names {
				parts = append(parts, name.Name+" "+fieldType)
			}
		}
	}
	
	if len(parts) == 1 && !strings.Contains(parts[0], " ") {
		return parts[0]
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

func identifyReferenceKind(ident *ast.Ident) string {
	return "identifier"
}

func extractContext(src []byte, pos token.Position) string {
	lines := strings.Split(string(src), "\n")
	if pos.Line <= 0 || pos.Line > len(lines) {
		return ""
	}
	
	start := pos.Line - 2
	if start < 0 {
		start = 0
	}
	end := pos.Line + 1
	if end > len(lines) {
		end = len(lines)
	}
	
	context := strings.Join(lines[start:end], "\n")
	return strings.TrimSpace(context)
}