package main

import (
	"go/ast"
	"go/token"
	"strings"
)

// Test quality types
type TestQualityInfo struct {
	File         string           `json:"file"`
	TestMetrics  TestMetrics      `json:"metrics"`
	Issues       []TestIssue      `json:"issues,omitempty"`
	Suggestions  []string         `json:"suggestions,omitempty"`
}

type TestMetrics struct {
	TotalTests    int     `json:"total_tests"`
	TableDriven   int     `json:"table_driven"`
	Benchmarks    int     `json:"benchmarks"`
	Examples      int     `json:"examples"`
	Coverage      float64 `json:"estimated_coverage"`
}

type TestIssue struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Severity    string   `json:"severity"`
	Position    Position `json:"position"`
}

func analyzeTestQuality(dir string) ([]TestQualityInfo, error) {
	var testQuality []TestQualityInfo

	err := walkGoFiles(dir, func(path string, src []byte, file *ast.File, fset *token.FileSet) error {
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}

		info := TestQualityInfo{
			File: path,
		}

		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok {
				name := fn.Name.Name
				if strings.HasPrefix(name, "Test") {
					info.TestMetrics.TotalTests++

					// Check for table-driven tests
					if hasTableDrivenPattern(fn) {
						info.TestMetrics.TableDriven++
					}

					// Check for proper assertions
					if !hasProperAssertions(fn) {
						pos := fset.Position(fn.Pos())
						info.Issues = append(info.Issues, TestIssue{
							Type:        "weak_assertions",
							Description: "Test lacks proper assertions",
							Severity:    "medium",
							Position:    newPosition(pos),
						})
					}
				} else if strings.HasPrefix(name, "Benchmark") {
					info.TestMetrics.Benchmarks++
				} else if strings.HasPrefix(name, "Example") {
					info.TestMetrics.Examples++
				}
			}
		}

		if info.TestMetrics.TotalTests > 0 {
			testQuality = append(testQuality, info)
		}
		return nil
	})

	return testQuality, err
}

func hasTableDrivenPattern(fn *ast.FuncDecl) bool {
	// Look for table-driven test patterns
	found := false
	ast.Inspect(fn, func(n ast.Node) bool {
		if genDecl, ok := n.(*ast.GenDecl); ok {
			for _, spec := range genDecl.Specs {
				if valueSpec, ok := spec.(*ast.ValueSpec); ok {
					for _, name := range valueSpec.Names {
						if strings.Contains(name.Name, "test") || strings.Contains(name.Name, "case") {
							found = true
						}
					}
				}
			}
		}
		return true
	})
	return found
}

func hasProperAssertions(fn *ast.FuncDecl) bool {
	// Look for testing.T calls
	found := false
	ast.Inspect(fn, func(n ast.Node) bool {
		if callExpr, ok := n.(*ast.CallExpr); ok {
			if selExpr, ok := callExpr.Fun.(*ast.SelectorExpr); ok {
				if ident, ok := selExpr.X.(*ast.Ident); ok && ident.Name == "t" {
					if selExpr.Sel.Name == "Error" || selExpr.Sel.Name == "Fatal" || 
					   selExpr.Sel.Name == "Fail" {
						found = true
					}
				}
			}
		}
		return true
	})
	return found
}