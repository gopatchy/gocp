package main

import (
	"go/ast"
	"go/token"
	"strings"
)

// Context usage types
type ContextInfo struct {
	File            string             `json:"file"`
	MissingContext  []ContextUsage     `json:"missing_context,omitempty"`
	ProperUsage     []ContextUsage     `json:"proper_usage,omitempty"`
	ImproperUsage   []ContextUsage     `json:"improper_usage,omitempty"`
}

type ContextUsage struct {
	Function    string   `json:"function"`
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Position    Position `json:"position"`
}

func findContextUsage(dir string) ([]ContextInfo, error) {
	var contextInfo []ContextInfo

	err := walkGoFiles(dir, func(path string, src []byte, file *ast.File, fset *token.FileSet) error {
		info := ContextInfo{
			File: path,
		}

		ast.Inspect(file, func(n ast.Node) bool {
			if fn, ok := n.(*ast.FuncDecl); ok && fn.Type.Params != nil {
				hasContext := false
				for _, param := range fn.Type.Params.List {
					if exprToString(param.Type) == "context.Context" {
						hasContext = true
						break
					}
				}

				// Check if function should have context
				if !hasContext && shouldHaveContext(fn) {
					pos := fset.Position(fn.Pos())
					info.MissingContext = append(info.MissingContext, ContextUsage{
						Function:    fn.Name.Name,
						Type:        "missing",
						Description: "Function should accept context.Context",
						Position:    newPosition(pos),
					})
				}
			}
			return true
		})

		if len(info.MissingContext) > 0 || len(info.ProperUsage) > 0 || len(info.ImproperUsage) > 0 {
			contextInfo = append(contextInfo, info)
		}
		return nil
	})

	return contextInfo, err
}

func shouldHaveContext(fn *ast.FuncDecl) bool {
	// Simple heuristic: functions that might do I/O
	name := strings.ToLower(fn.Name.Name)
	return strings.Contains(name, "get") || strings.Contains(name, "fetch") || 
		   strings.Contains(name, "load") || strings.Contains(name, "save")
}