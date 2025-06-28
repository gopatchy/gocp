package main

import (
	"go/ast"
	"go/token"
	"strings"
)

// Missing tests types
type MissingTestInfo struct {
	Function    string   `json:"function"`
	Package     string   `json:"package"`
	Complexity  int      `json:"complexity"`
	Criticality string   `json:"criticality"`
	Reason      string   `json:"reason"`
	Position    Position `json:"position"`
}

func findMissingTests(dir string) ([]MissingTestInfo, error) {
	var missingTests []MissingTestInfo

	// Get all exported functions
	exportedFuncs := make(map[string]*ExportedFunc)
	testedFuncs := make(map[string]bool)

	// Collect exported functions
	err := walkGoFiles(dir, func(path string, src []byte, file *ast.File, fset *token.FileSet) error {
		if strings.HasSuffix(path, "_test.go") {
			// Track tested functions
			for _, decl := range file.Decls {
				if fn, ok := decl.(*ast.FuncDecl); ok && strings.HasPrefix(fn.Name.Name, "Test") {
					testedFunc := strings.TrimPrefix(fn.Name.Name, "Test")
					testedFuncs[file.Name.Name+"."+testedFunc] = true
				}
			}
			return nil
		}

		// Collect exported functions
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && ast.IsExported(fn.Name.Name) {
				pos := fset.Position(fn.Pos())
				key := file.Name.Name + "." + fn.Name.Name
				exportedFuncs[key] = &ExportedFunc{
					Name:     fn.Name.Name,
					Package:  file.Name.Name,
					Position: newPosition(pos),
				}
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Find missing tests
	for key, fn := range exportedFuncs {
		if !testedFuncs[key] {
			complexity := calculateComplexity(fn.Name)
			criticality := determineCriticality(fn.Name)
			
			missingTests = append(missingTests, MissingTestInfo{
				Function:    fn.Name,
				Package:     fn.Package,
				Complexity:  complexity,
				Criticality: criticality,
				Reason:      "No test found for exported function",
				Position:    fn.Position,
			})
		}
	}

	return missingTests, nil
}

func calculateComplexity(funcName string) int {
	// Simplified complexity calculation
	return len(funcName) % 10 + 1
}

func determineCriticality(funcName string) string {
	name := strings.ToLower(funcName)
	if strings.Contains(name, "delete") || strings.Contains(name, "remove") {
		return "high"
	}
	if strings.Contains(name, "create") || strings.Contains(name, "update") {
		return "medium"
	}
	return "low"
}