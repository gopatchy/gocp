package main

import (
	"go/ast"
	"go/token"
	"sort"
	"strings"
)

type InitFunction struct {
	Package      string       `json:"package"`
	FilePath     string       `json:"file_path"`
	Position     Position     `json:"position"`
	Dependencies []string     `json:"dependencies"` // Packages this init might depend on
	HasSideEffects bool       `json:"has_side_effects"`
	Context      string       `json:"context"`
}

type InitAnalysis struct {
	InitFunctions []InitFunction  `json:"init_functions"`
	Issues        []InitIssue     `json:"issues"`
	InitOrder     []string        `json:"init_order"` // Suggested initialization order
}

type InitIssue struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Position    Position `json:"position"`
}

func findInitFunctions(dir string) (*InitAnalysis, error) {
	analysis := &InitAnalysis{
		InitFunctions: []InitFunction{},
		Issues:        []InitIssue{},
		InitOrder:     []string{},
	}

	packageInits := make(map[string][]InitFunction) // package -> init functions

	err := walkGoFiles(dir, func(path string, src []byte, file *ast.File, fset *token.FileSet) error {
		pkgName := file.Name.Name

		ast.Inspect(file, func(n ast.Node) bool {
			if funcDecl, ok := n.(*ast.FuncDecl); ok && funcDecl.Name.Name == "init" {
				pos := fset.Position(funcDecl.Pos())
				
				initFunc := InitFunction{
					Package:        pkgName,
					FilePath:       path,
					Position:       newPosition(pos),
					Dependencies:   extractInitDependencies(file, funcDecl),
					HasSideEffects: hasInitSideEffects(funcDecl),
					Context:        extractContext(src, pos),
				}
				
				analysis.InitFunctions = append(analysis.InitFunctions, initFunc)
				packageInits[pkgName] = append(packageInits[pkgName], initFunc)

				// Analyze init function for issues
				analyzeInitFunction(funcDecl, fset, analysis)
			}
			return true
		})

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Analyze package-level init dependencies
	analyzeInitDependencies(analysis, packageInits)

	// Sort init functions by package and file
	sort.Slice(analysis.InitFunctions, func(i, j int) bool {
		if analysis.InitFunctions[i].Package != analysis.InitFunctions[j].Package {
			return analysis.InitFunctions[i].Package < analysis.InitFunctions[j].Package
		}
		return analysis.InitFunctions[i].FilePath < analysis.InitFunctions[j].FilePath
	})

	return analysis, nil
}

func extractInitDependencies(file *ast.File, init *ast.FuncDecl) []string {
	deps := make(map[string]bool)

	// Look for package references in init
	ast.Inspect(init, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.SelectorExpr:
			if ident, ok := node.X.(*ast.Ident); ok {
				// Check if this is a package reference
				if isPackageIdent(file, ident.Name) {
					deps[ident.Name] = true
				}
			}
		case *ast.CallExpr:
			// Check for function calls that might depend on other packages
			if sel, ok := node.Fun.(*ast.SelectorExpr); ok {
				if ident, ok := sel.X.(*ast.Ident); ok {
					if isPackageIdent(file, ident.Name) {
						deps[ident.Name] = true
					}
				}
			}
		}
		return true
	})

	// Convert to slice
	var result []string
	for dep := range deps {
		result = append(result, dep)
	}
	sort.Strings(result)
	return result
}

func isPackageIdent(file *ast.File, name string) bool {
	// Check if name matches an import
	for _, imp := range file.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)
		pkgName := importPath
		if idx := strings.LastIndex(importPath, "/"); idx >= 0 {
			pkgName = importPath[idx+1:]
		}
		
		if imp.Name != nil {
			// Named import
			if imp.Name.Name == name {
				return true
			}
		} else if pkgName == name {
			// Default import name
			return true
		}
	}
	return false
}

func hasInitSideEffects(init *ast.FuncDecl) bool {
	var hasSideEffects bool

	ast.Inspect(init, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.AssignStmt:
			// Check for global variable assignments
			hasSideEffects = true
			return false
		case *ast.CallExpr:
			// Function calls likely have side effects
			hasSideEffects = true
			return false
		case *ast.GoStmt:
			// Starting goroutines
			hasSideEffects = true
			return false
		case *ast.SendStmt:
			// Channel operations
			hasSideEffects = true
			return false
		}
		return true
	})

	return hasSideEffects
}

func analyzeInitFunction(init *ast.FuncDecl, fset *token.FileSet, analysis *InitAnalysis) {
	// Check for complex init logic
	stmtCount := countStatements(init.Body)
	if stmtCount > 20 {
		pos := fset.Position(init.Pos())
		issue := InitIssue{
			Type:        "complex_init",
			Description: "init() function is complex - consider refactoring",
			Position:    newPosition(pos),
		}
		analysis.Issues = append(analysis.Issues, issue)
	}

	// Check for blocking operations
	ast.Inspect(init, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CallExpr:
			if sel, ok := node.Fun.(*ast.SelectorExpr); ok {
				funcName := sel.Sel.Name
				// Check for potentially blocking operations
				if isBlockingCall(sel) {
					pos := fset.Position(node.Pos())
					issue := InitIssue{
						Type:        "blocking_init",
						Description: "init() contains potentially blocking call: " + funcName,
						Position:    newPosition(pos),
					}
					analysis.Issues = append(analysis.Issues, issue)
				}
			}

		case *ast.GoStmt:
			pos := fset.Position(node.Pos())
			issue := InitIssue{
				Type:        "goroutine_in_init",
				Description: "init() starts a goroutine - may cause race conditions",
				Position:    newPosition(pos),
			}
			analysis.Issues = append(analysis.Issues, issue)

		case *ast.ForStmt:
			if node.Cond == nil {
				pos := fset.Position(node.Pos())
				issue := InitIssue{
					Type:        "infinite_loop_in_init",
					Description: "init() contains infinite loop - will block program startup",
					Position:    newPosition(pos),
				}
				analysis.Issues = append(analysis.Issues, issue)
			}
		}
		return true
	})
}

func countStatements(block *ast.BlockStmt) int {
	count := 0
	ast.Inspect(block, func(n ast.Node) bool {
		switch n.(type) {
		case ast.Stmt:
			count++
		}
		return true
	})
	return count
}

func isBlockingCall(sel *ast.SelectorExpr) bool {
	// Check for common blocking operations
	methodName := sel.Sel.Name
	blockingMethods := []string{
		"Sleep", "Wait", "Lock", "RLock", "Dial", "Connect",
		"Open", "Create", "ReadFile", "WriteFile", "Sync",
	}
	
	for _, blocking := range blockingMethods {
		if strings.Contains(methodName, blocking) {
			return true
		}
	}

	// Check for HTTP/network operations
	if ident, ok := sel.X.(*ast.Ident); ok {
		if ident.Name == "http" || ident.Name == "net" {
			return true
		}
	}

	return false
}

func analyzeInitDependencies(analysis *InitAnalysis, packageInits map[string][]InitFunction) {
	// Build dependency graph
	depGraph := make(map[string][]string)
	
	for pkg, inits := range packageInits {
		deps := make(map[string]bool)
		for _, init := range inits {
			for _, dep := range init.Dependencies {
				deps[dep] = true
			}
		}
		
		var depList []string
		for dep := range deps {
			depList = append(depList, dep)
		}
		depGraph[pkg] = depList
	}

	// Check for circular dependencies
	for pkg := range depGraph {
		if hasCycle, cycle := detectCycle(pkg, depGraph, make(map[string]bool), []string{pkg}); hasCycle {
			issue := InitIssue{
				Type:        "circular_init_dependency",
				Description: "Circular init dependency detected: " + strings.Join(cycle, " -> "),
				Position:    Position{}, // Package-level issue
			}
			analysis.Issues = append(analysis.Issues, issue)
		}
	}

	// Suggest initialization order (topological sort)
	analysis.InitOrder = topologicalSort(depGraph)
}

func detectCycle(current string, graph map[string][]string, visited map[string]bool, path []string) (bool, []string) {
	if visited[current] {
		// Find where the cycle starts
		for i, pkg := range path {
			if pkg == current {
				return true, path[i:]
			}
		}
	}

	visited[current] = true
	
	for _, dep := range graph[current] {
		newPath := append(path, dep)
		if hasCycle, cycle := detectCycle(dep, graph, visited, newPath); hasCycle {
			return true, cycle
		}
	}
	
	delete(visited, current)
	return false, nil
}

func topologicalSort(graph map[string][]string) []string {
	// Simple topological sort for init order
	var result []string
	visited := make(map[string]bool)
	temp := make(map[string]bool)

	var visit func(string) bool
	visit = func(pkg string) bool {
		if temp[pkg] {
			return false // Cycle detected
		}
		if visited[pkg] {
			return true
		}

		temp[pkg] = true
		for _, dep := range graph[pkg] {
			if !visit(dep) {
				return false
			}
		}
		temp[pkg] = false
		visited[pkg] = true
		result = append([]string{pkg}, result...) // Prepend
		return true
	}

	var packages []string
	for pkg := range graph {
		packages = append(packages, pkg)
	}
	sort.Strings(packages)

	for _, pkg := range packages {
		if !visited[pkg] {
			visit(pkg)
		}
	}

	return result
}