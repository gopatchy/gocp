package main

import (
	"go/ast"
	"go/token"
	"regexp"
	"strings"
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
	Context  []string `json:"context,omitempty"`
}

func getContext(src []byte, pos token.Position, contextLines int) []string {
	lines := strings.Split(string(src), "\n")
	startLine := pos.Line - contextLines - 1
	endLine := pos.Line + contextLines - 1
	
	if startLine < 0 {
		startLine = 0
	}
	if endLine >= len(lines) {
		endLine = len(lines) - 1
	}
	
	var context []string
	for i := startLine; i <= endLine; i++ {
		context = append(context, lines[i])
	}
	return context
}

func findComments(dir string, commentType string, filter string, includeContext bool) ([]CommentInfo, error) {
	var comments []CommentInfo

	err := walkGoFiles(dir, func(path string, src []byte, file *ast.File, fset *token.FileSet) error {
		info := CommentInfo{
			File: path,
		}

		// Find comments based on type
		if commentType == "todo" || commentType == "all" {
			var filterRegex *regexp.Regexp
			if filter != "" {
				var err error
				filterRegex, err = regexp.Compile(filter)
				if err != nil {
					return err
				}
			} else if commentType == "todo" {
				// Default TODO regex if no filter provided and type is "todo"
				filterRegex = regexp.MustCompile(`(?i)\b(todo|fixme|hack|bug|xxx)\b`)
			}
			
			for _, cg := range file.Comments {
				for _, c := range cg.List {
					// If no filter or filter matches, include the comment
					if filterRegex == nil || filterRegex.MatchString(c.Text) {
						pos := fset.Position(c.Pos())
						item := CommentItem{
							Comment:  c.Text,
							Type:     "comment",
							Position: newPosition(pos),
						}
						if includeContext {
							item.Context = getContext(src, pos, 3)
						}
						info.TODOs = append(info.TODOs, item)
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
						item := CommentItem{
							Name:     x.Name.Name,
							Type:     "function",
							Position: newPosition(pos),
						}
						if includeContext {
							item.Context = getContext(src, pos, 3)
						}
						info.Undocumented = append(info.Undocumented, item)
					}
				case *ast.GenDecl:
					for _, spec := range x.Specs {
						switch s := spec.(type) {
						case *ast.TypeSpec:
							if ast.IsExported(s.Name.Name) && x.Doc == nil && s.Doc == nil {
								pos := fset.Position(s.Pos())
								item := CommentItem{
									Name:     s.Name.Name,
									Type:     "type",
									Position: newPosition(pos),
								}
								if includeContext {
									item.Context = getContext(src, pos, 3)
								}
								info.Undocumented = append(info.Undocumented, item)
							}
						case *ast.ValueSpec:
							for _, name := range s.Names {
								if ast.IsExported(name.Name) && x.Doc == nil && s.Doc == nil {
									pos := fset.Position(name.Pos())
									item := CommentItem{
										Name:     name.Name,
										Type:     "value",
										Position: newPosition(pos),
									}
									if includeContext {
										item.Context = getContext(src, pos, 3)
									}
									info.Undocumented = append(info.Undocumented, item)
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