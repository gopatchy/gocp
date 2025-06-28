package main

import (
	"go/ast"
	"go/token"
	"path/filepath"
	"strings"
)

// Import analysis types
type ImportInfo struct {
	Package      string            `json:"package"`
	File         string            `json:"file"`
	Imports      []ImportDetail    `json:"imports"`
	UnusedImports []string         `json:"unused_imports,omitempty"`
}

type ImportDetail struct {
	Path     string   `json:"path"`
	Alias    string   `json:"alias,omitempty"`
	Used     []string `json:"used_symbols,omitempty"`
	Position Position `json:"position"`
}

func findImports(dir string) ([]ImportInfo, error) {
	var imports []ImportInfo

	err := walkGoFiles(dir, func(path string, src []byte, file *ast.File, fset *token.FileSet) error {
		info := ImportInfo{
			Package: file.Name.Name,
			File:    path,
			Imports: []ImportDetail{},
		}

		// Collect all imports
		importMap := make(map[string]*ImportDetail)
		for _, imp := range file.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)
			pos := fset.Position(imp.Pos())
			detail := &ImportDetail{
				Path:     importPath,
				Position: newPosition(pos),
			}
			if imp.Name != nil {
				detail.Alias = imp.Name.Name
			}
			importMap[importPath] = detail
			info.Imports = append(info.Imports, *detail)
		}

		// Track which imports are used
		usedImports := make(map[string]map[string]bool)
		ast.Inspect(file, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.SelectorExpr:
				if ident, ok := x.X.(*ast.Ident); ok {
					pkgName := ident.Name
					symbol := x.Sel.Name
					
					// Find matching import
					for importPath, detail := range importMap {
						importName := filepath.Base(importPath)
						if detail.Alias != "" && detail.Alias == pkgName {
							if usedImports[importPath] == nil {
								usedImports[importPath] = make(map[string]bool)
							}
							usedImports[importPath][symbol] = true
						} else if importName == pkgName {
							if usedImports[importPath] == nil {
								usedImports[importPath] = make(map[string]bool)
							}
							usedImports[importPath][symbol] = true
						}
					}
				}
			}
			return true
		})

		// Update import details with used symbols
		for i, imp := range info.Imports {
			if used, ok := usedImports[imp.Path]; ok {
				for symbol := range used {
					info.Imports[i].Used = append(info.Imports[i].Used, symbol)
				}
			} else if !strings.HasSuffix(imp.Path, "_test") && imp.Alias != "_" {
				info.UnusedImports = append(info.UnusedImports, imp.Path)
			}
		}

		if len(info.Imports) > 0 {
			imports = append(imports, info)
		}
		return nil
	})

	return imports, err
}