package main

import (
	"go/ast"
	"go/token"
	"regexp"
)

// Comment analysis types
type CommentInfo struct {
	File         string         `json:"file"`
	TODOs        []CommentItem  `json:"todos,omitempty"`
	Undocumented []CommentItem  `json:"undocumented,omitempty"`
}

type CommentItem struct {
	Name     string   `json:"name"`
	Comment  string   `json:"comment,omitempty"`
	Type     string   `json:"type"`
	Position Position `json:"position"`
}

func findComments(dir string, commentType string) ([]CommentInfo, error) {
	var comments []CommentInfo

	err := walkGoFiles(dir, func(path string, src []byte, file *ast.File, fset *token.FileSet) error {
		info := CommentInfo{
			File: path,
		}

		// Find TODOs in comments
		if commentType == "todo" || commentType == "all" {
			todoRegex := regexp.MustCompile(`(?i)\b(todo|fixme|hack|bug|xxx)\b`)
			for _, cg := range file.Comments {
				for _, c := range cg.List {
					if todoRegex.MatchString(c.Text) {
						pos := fset.Position(c.Pos())
						info.TODOs = append(info.TODOs, CommentItem{
							Comment:  c.Text,
							Type:     "todo",
							Position: newPosition(pos),
						})
					}
				}
			}
		}

		// Find undocumented exported symbols
		if commentType == "undocumented" || commentType == "all" {
			ast.Inspect(file, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.FuncDecl:
					if ast.IsExported(x.Name.Name) && x.Doc == nil {
						pos := fset.Position(x.Pos())
						info.Undocumented = append(info.Undocumented, CommentItem{
							Name:     x.Name.Name,
							Type:     "function",
							Position: newPosition(pos),
						})
					}
				case *ast.GenDecl:
					for _, spec := range x.Specs {
						switch s := spec.(type) {
						case *ast.TypeSpec:
							if ast.IsExported(s.Name.Name) && x.Doc == nil && s.Doc == nil {
								pos := fset.Position(s.Pos())
								info.Undocumented = append(info.Undocumented, CommentItem{
									Name:     s.Name.Name,
									Type:     "type",
									Position: newPosition(pos),
								})
							}
						case *ast.ValueSpec:
							for _, name := range s.Names {
								if ast.IsExported(name.Name) && x.Doc == nil && s.Doc == nil {
									pos := fset.Position(name.Pos())
									info.Undocumented = append(info.Undocumented, CommentItem{
										Name:     name.Name,
										Type:     "value",
										Position: newPosition(pos),
									})
								}
							}
						}
					}
				}
				return true
			})
		}

		if len(info.TODOs) > 0 || len(info.Undocumented) > 0 {
			comments = append(comments, info)
		}
		return nil
	})

	return comments, err
}