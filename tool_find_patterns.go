package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"
)

// Design pattern types
type PatternInfo struct {
	Pattern     string             `json:"pattern"`
	Occurrences []PatternOccurrence `json:"occurrences"`
}

type PatternOccurrence struct {
	File        string   `json:"file"`
	Description string   `json:"description"`
	Quality     string   `json:"quality"`
	Position    Position `json:"position"`
}

func findPatterns(dir string) ([]PatternInfo, error) {
	var patterns []PatternInfo

	err := walkGoFiles(dir, func(path string, src []byte, file *ast.File, fset *token.FileSet) error {
		// Look for singleton pattern
		singletonPattern := PatternInfo{Pattern: "singleton"}
		
		// Look for factory pattern
		factoryPattern := PatternInfo{Pattern: "factory"}

		ast.Inspect(file, func(n ast.Node) bool {
			if fn, ok := n.(*ast.FuncDecl); ok {
				name := strings.ToLower(fn.Name.Name)
				
				// Detect factory pattern
				if strings.HasPrefix(name, "new") || strings.HasPrefix(name, "create") {
					pos := fset.Position(fn.Pos())
					factoryPattern.Occurrences = append(factoryPattern.Occurrences, PatternOccurrence{
						File:        path,
						Description: fmt.Sprintf("Factory function: %s", fn.Name.Name),
						Quality:     "good",
						Position:    newPosition(pos),
					})
				}

				// Detect singleton pattern (simplified)
				if strings.Contains(name, "instance") && fn.Type.Results != nil {
					pos := fset.Position(fn.Pos())
					singletonPattern.Occurrences = append(singletonPattern.Occurrences, PatternOccurrence{
						File:        path,
						Description: fmt.Sprintf("Potential singleton: %s", fn.Name.Name),
						Quality:     "review",
						Position:    newPosition(pos),
					})
				}
			}
			return true
		})

		if len(singletonPattern.Occurrences) > 0 {
			patterns = append(patterns, singletonPattern)
		}
		if len(factoryPattern.Occurrences) > 0 {
			patterns = append(patterns, factoryPattern)
		}

		return nil
	})

	return patterns, err
}