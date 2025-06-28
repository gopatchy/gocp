package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Position represents a location in source code
type Position struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
	Offset int    `json:"offset"` // byte offset in file
}

// newPosition creates a Position from a token.Position
func newPosition(pos token.Position) Position {
	return Position{
		File:   pos.Filename,
		Line:   pos.Line,
		Column: pos.Column,
		Offset: pos.Offset,
	}
}

type fileVisitor func(path string, src []byte, file *ast.File, fset *token.FileSet) error

func walkGoFiles(dir string, visitor fileVisitor) error {
	fset := token.NewFileSet()

	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.Contains(path, "vendor/") {
			return nil
		}

		src, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		file, err := parser.ParseFile(fset, path, src, parser.ParseComments)
		if err != nil {
			return nil
		}

		return visitor(path, src, file, fset)
	})
}

func exprToString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return "*" + exprToString(e.X)
	case *ast.SelectorExpr:
		return exprToString(e.X) + "." + e.Sel.Name
	case *ast.ArrayType:
		if e.Len == nil {
			return "[]" + exprToString(e.Elt)
		}
		return "[" + exprToString(e.Len) + "]" + exprToString(e.Elt)
	case *ast.MapType:
		return "map[" + exprToString(e.Key) + "]" + exprToString(e.Value)
	case *ast.InterfaceType:
		if len(e.Methods.List) == 0 {
			return "interface{}"
		}
		return "interface{...}"
	case *ast.FuncType:
		return funcSignature(e)
	case *ast.ChanType:
		switch e.Dir {
		case ast.SEND:
			return "chan<- " + exprToString(e.Value)
		case ast.RECV:
			return "<-chan " + exprToString(e.Value)
		default:
			return "chan " + exprToString(e.Value)
		}
	case *ast.BasicLit:
		return e.Value
	default:
		return "unknown"
	}
}

func funcSignature(fn *ast.FuncType) string {
	params := fieldListToString(fn.Params)
	results := fieldListToString(fn.Results)
	
	if results == "" {
		return "func(" + params + ")"
	}
	return "func(" + params + ") " + results
}

func fieldListToString(fl *ast.FieldList) string {
	if fl == nil || len(fl.List) == 0 {
		return ""
	}
	
	var parts []string
	for _, field := range fl.List {
		fieldType := exprToString(field.Type)
		if len(field.Names) == 0 {
			parts = append(parts, fieldType)
		} else {
			for _, name := range field.Names {
				parts = append(parts, name.Name+" "+fieldType)
			}
		}
	}
	
	if len(parts) == 1 && !strings.Contains(parts[0], " ") {
		return parts[0]
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

func extractContext(src []byte, pos token.Position) string {
	lines := strings.Split(string(src), "\n")
	if pos.Line <= 0 || pos.Line > len(lines) {
		return ""
	}
	
	start := pos.Line - 2
	if start < 0 {
		start = 0
	}
	end := pos.Line + 1
	if end > len(lines) {
		end = len(lines)
	}
	
	context := strings.Join(lines[start:end], "\n")
	return strings.TrimSpace(context)
}

func extractDocString(doc *ast.CommentGroup) string {
	if doc == nil {
		return ""
	}
	var text strings.Builder
	for _, comment := range doc.List {
		text.WriteString(strings.TrimPrefix(comment.Text, "//"))
		text.WriteString(" ")
	}
	return strings.TrimSpace(text.String())
}

func isErrorCheck(ifStmt *ast.IfStmt) bool {
	// Check if this is an "if err != nil" pattern
	if binExpr, ok := ifStmt.Cond.(*ast.BinaryExpr); ok {
		if binExpr.Op == token.NEQ {
			if ident, ok := binExpr.X.(*ast.Ident); ok && (ident.Name == "err" || strings.Contains(ident.Name, "error")) {
				if ident2, ok := binExpr.Y.(*ast.Ident); ok && ident2.Name == "nil" {
					return true
				}
			}
		}
	}
	return false
}