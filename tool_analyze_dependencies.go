package main

import (
	"go/ast"
	"go/token"
	"path/filepath"
	"strings"
)

// Dependency analysis types
type DependencyInfo struct {
	Package      string         `json:"package"`
	Dir          string         `json:"dir"`
	Dependencies []string       `json:"dependencies"`
	Dependents   []string       `json:"dependents,omitempty"`
	Cycles       [][]string     `json:"cycles,omitempty"`
}

func analyzeDependencies(dir string) ([]DependencyInfo, error) {
	depMap := make(map[string]*DependencyInfo)

	// First pass: collect all packages and their imports
	err := walkGoFiles(dir, func(path string, src []byte, file *ast.File, fset *token.FileSet) error {
		pkgDir := filepath.Dir(path)
		if _, exists := depMap[pkgDir]; !exists {
			depMap[pkgDir] = &DependencyInfo{
				Package:      file.Name.Name,
				Dir:          pkgDir,
				Dependencies: []string{},
			}
		}

		// Add imports
		for _, imp := range file.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)
			if !contains(depMap[pkgDir].Dependencies, importPath) {
				depMap[pkgDir].Dependencies = append(depMap[pkgDir].Dependencies, importPath)
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Build dependency graph and find cycles
	var deps []DependencyInfo
	for _, dep := range depMap {
		// Find internal dependencies
		for _, imp := range dep.Dependencies {
			// Check if this is an internal package
			for otherDir, otherDep := range depMap {
				if strings.HasSuffix(imp, otherDep.Package) && otherDir != dep.Dir {
					otherDep.Dependents = append(otherDep.Dependents, dep.Package)
				}
			}
		}
		deps = append(deps, *dep)
	}

	// Simple cycle detection (could be enhanced)
	for i := range deps {
		deps[i].Cycles = findCycles(&deps[i], depMap)
	}

	return deps, nil
}

func contains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

func findCycles(dep *DependencyInfo, depMap map[string]*DependencyInfo) [][]string {
	// Simple DFS-based cycle detection
	var cycles [][]string
	visited := make(map[string]bool)
	recStack := make(map[string]bool)
	path := []string{}

	var dfs func(pkg string) bool
	dfs = func(pkg string) bool {
		visited[pkg] = true
		recStack[pkg] = true
		path = append(path, pkg)

		// Find dependencies for this package
		for _, d := range depMap {
			if d.Package == pkg {
				for _, imp := range d.Dependencies {
					for _, otherDep := range depMap {
						if strings.HasSuffix(imp, otherDep.Package) {
							if !visited[otherDep.Package] {
								if dfs(otherDep.Package) {
									return true
								}
							} else if recStack[otherDep.Package] {
								// Found a cycle
								cycleStart := -1
								for i, p := range path {
									if p == otherDep.Package {
										cycleStart = i
										break
									}
								}
								if cycleStart >= 0 {
									cycle := append([]string{}, path[cycleStart:]...)
									cycles = append(cycles, cycle)
								}
								return true
							}
						}
					}
				}
				break
			}
		}

		path = path[:len(path)-1]
		recStack[pkg] = false
		return false
	}

	dfs(dep.Package)
	return cycles
}