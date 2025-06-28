package main

import (
	"crypto/md5"
	"fmt"
	"go/ast"
	"go/token"
)

// Code duplication types
type DuplicateInfo struct {
	Similarity float64           `json:"similarity"`
	Locations  []DuplicateLocation `json:"locations"`
	Content    string            `json:"content"`
}

type DuplicateLocation struct {
	File     string   `json:"file"`
	Function string   `json:"function"`
	Position Position `json:"position"`
}

func findDuplicates(dir string, threshold float64) ([]DuplicateInfo, error) {
	var duplicates []DuplicateInfo
	functionBodies := make(map[string][]DuplicateLocation)

	err := walkGoFiles(dir, func(path string, src []byte, file *ast.File, fset *token.FileSet) error {
		ast.Inspect(file, func(n ast.Node) bool {
			if fn, ok := n.(*ast.FuncDecl); ok && fn.Body != nil {
				body := extractFunctionBody(fn.Body, fset)
				hash := fmt.Sprintf("%x", md5.Sum([]byte(body)))
				
				pos := fset.Position(fn.Pos())
				location := DuplicateLocation{
					File:     path,
					Function: fn.Name.Name,
					Position: newPosition(pos),
				}
				
				functionBodies[hash] = append(functionBodies[hash], location)
			}
			return true
		})
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Find duplicates
	for hash, locations := range functionBodies {
		if len(locations) > 1 {
			duplicates = append(duplicates, DuplicateInfo{
				Similarity: 1.0,
				Locations:  locations,
				Content:    hash,
			})
		}
	}

	return duplicates, nil
}

func extractFunctionBody(body *ast.BlockStmt, fset *token.FileSet) string {
	start := fset.Position(body.Pos())
	end := fset.Position(body.End())
	return fmt.Sprintf("%d-%d", start.Line, end.Line)
}