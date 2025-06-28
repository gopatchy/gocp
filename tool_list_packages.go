package main

import (
	"go/ast"
	"go/token"
	"path/filepath"
	"strings"
)

type Package struct {
	ImportPath string   `json:"import_path"`
	Name       string   `json:"name"`
	Dir        string   `json:"dir"`
	GoFiles    []string `json:"go_files"`
	Imports    []string `json:"imports"`
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