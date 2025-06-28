package main

import (
	"go/ast"
	"go/token"
	"strings"
)

// Test analysis types
type TestAnalysis struct {
	TestFiles         []TestFile      `json:"test_files"`
	ExportedFunctions []ExportedFunc  `json:"exported_functions"`
	TestCoverage      TestCoverage    `json:"coverage_summary"`
}

type TestFile struct {
	File      string   `json:"file"`
	Package   string   `json:"package"`
	Tests     []string `json:"tests"`
	Benchmarks []string `json:"benchmarks,omitempty"`
	Examples  []string `json:"examples,omitempty"`
}

type ExportedFunc struct {
	Name     string   `json:"name"`
	Package  string   `json:"package"`
	Tested   bool     `json:"tested"`
	Position Position `json:"position"`
}

type TestCoverage struct {
	TotalExported int     `json:"total_exported"`
	TotalTested   int     `json:"total_tested"`
	Percentage    float64 `json:"percentage"`
}

func analyzeTests(dir string) (*TestAnalysis, error) {
	analysis := &TestAnalysis{
		TestFiles:         []TestFile{},
		ExportedFunctions: []ExportedFunc{},
	}

	// Collect all exported functions
	exportedFuncs := make(map[string]*ExportedFunc)
	
	err := walkGoFiles(dir, func(path string, src []byte, file *ast.File, fset *token.FileSet) error {
		if strings.HasSuffix(path, "_test.go") {
			// Process test files
			testFile := TestFile{
				File:    path,
				Package: file.Name.Name,
			}

			for _, decl := range file.Decls {
				if fn, ok := decl.(*ast.FuncDecl); ok {
					name := fn.Name.Name
					if strings.HasPrefix(name, "Test") {
						testFile.Tests = append(testFile.Tests, name)
					} else if strings.HasPrefix(name, "Benchmark") {
						testFile.Benchmarks = append(testFile.Benchmarks, name)
					} else if strings.HasPrefix(name, "Example") {
						testFile.Examples = append(testFile.Examples, name)
					}
				}
			}

			if len(testFile.Tests) > 0 || len(testFile.Benchmarks) > 0 || len(testFile.Examples) > 0 {
				analysis.TestFiles = append(analysis.TestFiles, testFile)
			}
		} else {
			// Collect exported functions
			for _, decl := range file.Decls {
				if fn, ok := decl.(*ast.FuncDecl); ok && ast.IsExported(fn.Name.Name) {
					key := file.Name.Name + "." + fn.Name.Name
					pos := fset.Position(fn.Pos())
					exportedFuncs[key] = &ExportedFunc{
						Name:     fn.Name.Name,
						Package:  file.Name.Name,
						Tested:   false,
						Position: newPosition(pos),
					}
				}
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Check which functions are tested
	for _, testFile := range analysis.TestFiles {
		for _, testName := range testFile.Tests {
			// Simple heuristic: TestFunctionName tests FunctionName
			funcName := strings.TrimPrefix(testName, "Test")
			key := testFile.Package + "." + funcName
			if fn, exists := exportedFuncs[key]; exists {
				fn.Tested = true
			}
		}
	}

	// Convert map to slice and calculate coverage
	tested := 0
	for _, fn := range exportedFuncs {
		analysis.ExportedFunctions = append(analysis.ExportedFunctions, *fn)
		if fn.Tested {
			tested++
		}
	}

	analysis.TestCoverage = TestCoverage{
		TotalExported: len(exportedFuncs),
		TotalTested:   tested,
	}
	if len(exportedFuncs) > 0 {
		analysis.TestCoverage.Percentage = float64(tested) / float64(len(exportedFuncs)) * 100
	}

	return analysis, nil
}