package main

import (
	"go/ast"
	"go/token"
	"strings"
)

// Go idioms types
type IdiomsInfo struct {
	File         string        `json:"file"`
	Violations   []IdiomItem   `json:"violations,omitempty"`
	Suggestions  []IdiomItem   `json:"suggestions,omitempty"`
}

type IdiomItem struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Suggestion  string   `json:"suggestion"`
	Position    Position `json:"position"`
}

func analyzeGoIdioms(dir string) ([]IdiomsInfo, error) {
	var idioms []IdiomsInfo

	err := walkGoFiles(dir, func(path string, src []byte, file *ast.File, fset *token.FileSet) error {
		info := IdiomsInfo{
			File: path,
		}

		ast.Inspect(file, func(n ast.Node) bool {
			// Check for proper error handling
			if ifStmt, ok := n.(*ast.IfStmt); ok {
				if !isErrorCheck(ifStmt) {
					// Look for other patterns that might be non-idiomatic
					pos := fset.Position(ifStmt.Pos())
					info.Suggestions = append(info.Suggestions, IdiomItem{
						Type:        "error_handling",
						Description: "Consider Go error handling patterns",
						Suggestion:  "Use 'if err != nil' pattern",
						Position:    newPosition(pos),
					})
				}
			}

			// Check for receiver naming
			if fn, ok := n.(*ast.FuncDecl); ok && fn.Recv != nil {
				for _, recv := range fn.Recv.List {
					if len(recv.Names) > 0 {
						name := recv.Names[0].Name
						if len(name) > 1 && !isValidReceiverName(name) {
							pos := fset.Position(recv.Pos())
							info.Violations = append(info.Violations, IdiomItem{
								Type:        "receiver_naming",
								Description: "Receiver name should be short abbreviation",
								Suggestion:  "Use 1-2 character receiver names",
								Position:    newPosition(pos),
							})
						}
					}
				}
			}

			return true
		})

		if len(info.Violations) > 0 || len(info.Suggestions) > 0 {
			idioms = append(idioms, info)
		}
		return nil
	})

	return idioms, err
}

func isValidReceiverName(name string) bool {
	return len(name) <= 2 && strings.ToLower(name) == name
}