package main

import (
	"go/ast"
	"go/token"
	"strings"
)

// Deprecated usage types
type DeprecatedInfo struct {
	File  string              `json:"file"`
	Usage []DeprecatedUsage   `json:"usage"`
}

type DeprecatedUsage struct {
	Item        string   `json:"item"`
	Alternative string   `json:"alternative,omitempty"`
	Reason      string   `json:"reason,omitempty"`
	Position    Position `json:"position"`
}

func findDeprecated(dir string) ([]DeprecatedInfo, error) {
	var deprecated []DeprecatedInfo

	err := walkGoFiles(dir, func(path string, src []byte, file *ast.File, fset *token.FileSet) error {
		info := DeprecatedInfo{
			File: path,
		}

		// Look for deprecated comments
		for _, cg := range file.Comments {
			for _, c := range cg.List {
				if strings.Contains(strings.ToLower(c.Text), "deprecated") {
					pos := fset.Position(c.Pos())
					info.Usage = append(info.Usage, DeprecatedUsage{
						Item:     "deprecated_comment",
						Reason:   c.Text,
						Position: newPosition(pos),
					})
				}
			}
		}

		if len(info.Usage) > 0 {
			deprecated = append(deprecated, info)
		}
		return nil
	})

	return deprecated, err
}