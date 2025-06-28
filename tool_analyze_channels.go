package main

import (
	"go/ast"
	"go/token"
)

type ChannelUsage struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"` // "make", "send", "receive", "range", "select", "close"
	ChannelType string   `json:"channel_type"` // "unbuffered", "buffered", "unknown"
	BufferSize  int      `json:"buffer_size,omitempty"`
	Position    Position `json:"position"`
	Context     string   `json:"context"`
}

type ChannelAnalysis struct {
	Channels []ChannelUsage  `json:"channels"`
	Issues   []ChannelIssue  `json:"issues"`
}

type ChannelIssue struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Position    Position `json:"position"`
}

func analyzeChannels(dir string) (*ChannelAnalysis, error) {
	analysis := &ChannelAnalysis{
		Channels: []ChannelUsage{},
		Issues:   []ChannelIssue{},
	}

	err := walkGoFiles(dir, func(path string, src []byte, file *ast.File, fset *token.FileSet) error {
		// Track channel variables
		channelVars := make(map[string]*ChannelInfo)
		
		// First pass: identify channel declarations
		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.ValueSpec:
				for i, name := range node.Names {
					if isChanType(node.Type) {
						channelVars[name.Name] = &ChannelInfo{
							name:     name.Name,
							chanType: "unknown",
						}
					} else if i < len(node.Values) {
						if info := extractChannelMake(node.Values[i]); info != nil {
							info.name = name.Name
							channelVars[name.Name] = info
						}
					}
				}
			
			case *ast.AssignStmt:
				for i, lhs := range node.Lhs {
					if ident, ok := lhs.(*ast.Ident); ok && i < len(node.Rhs) {
						if info := extractChannelMake(node.Rhs[i]); info != nil {
							info.name = ident.Name
							channelVars[ident.Name] = info
							
							pos := fset.Position(node.Pos())
							usage := ChannelUsage{
								Name:        ident.Name,
								Type:        "make",
								ChannelType: info.chanType,
								BufferSize:  info.bufferSize,
								Position:    newPosition(pos),
								Context:     extractContext(src, pos),
							}
							analysis.Channels = append(analysis.Channels, usage)
						}
					}
				}
			}
			return true
		})

		// Second pass: analyze channel operations
		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.SendStmt:
				pos := fset.Position(node.Pos())
				chanName := extractChannelName(node.Chan)
				usage := ChannelUsage{
					Name:     chanName,
					Type:     "send",
					Position: newPosition(pos),
					Context:  extractContext(src, pos),
				}
				if info, ok := channelVars[chanName]; ok {
					usage.ChannelType = info.chanType
					usage.BufferSize = info.bufferSize
				}
				analysis.Channels = append(analysis.Channels, usage)
				
				// Check for potential deadlock
				if isInMainGoroutine(file, node) && !hasGoroutineNearby(file, node) {
					if info, ok := channelVars[chanName]; ok && info.chanType == "unbuffered" {
						issue := ChannelIssue{
							Type:        "potential_deadlock",
							Description: "Send on unbuffered channel without goroutine may deadlock",
							Position:    newPosition(pos),
						}
						analysis.Issues = append(analysis.Issues, issue)
					}
				}
			
			case *ast.UnaryExpr:
				if node.Op == token.ARROW {
					pos := fset.Position(node.Pos())
					chanName := extractChannelName(node.X)
					usage := ChannelUsage{
						Name:     chanName,
						Type:     "receive",
						Position: newPosition(pos),
						Context:  extractContext(src, pos),
					}
					if info, ok := channelVars[chanName]; ok {
						usage.ChannelType = info.chanType
						usage.BufferSize = info.bufferSize
					}
					analysis.Channels = append(analysis.Channels, usage)
				}
			
			case *ast.RangeStmt:
				if isChanExpression(node.X) {
					pos := fset.Position(node.Pos())
					chanName := extractChannelName(node.X)
					usage := ChannelUsage{
						Name:     chanName,
						Type:     "range",
						Position: newPosition(pos),
						Context:  extractContext(src, pos),
					}
					analysis.Channels = append(analysis.Channels, usage)
				}
			
			case *ast.CallExpr:
				if ident, ok := node.Fun.(*ast.Ident); ok && ident.Name == "close" {
					if len(node.Args) > 0 {
						pos := fset.Position(node.Pos())
						chanName := extractChannelName(node.Args[0])
						usage := ChannelUsage{
							Name:     chanName,
							Type:     "close",
							Position: newPosition(pos),
							Context:  extractContext(src, pos),
						}
						analysis.Channels = append(analysis.Channels, usage)
					}
				}
			
			case *ast.SelectStmt:
				analyzeSelectStatement(node, fset, src, analysis, channelVars)
			}
			return true
		})

		return nil
	})

	return analysis, err
}

type ChannelInfo struct {
	name       string
	chanType   string // "buffered", "unbuffered", "unknown"
	bufferSize int
}

func extractChannelMake(expr ast.Expr) *ChannelInfo {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return nil
	}
	
	ident, ok := call.Fun.(*ast.Ident)
	if !ok || ident.Name != "make" {
		return nil
	}
	
	if len(call.Args) < 1 || !isChanType(call.Args[0]) {
		return nil
	}
	
	info := &ChannelInfo{}
	
	if len(call.Args) == 1 {
		info.chanType = "unbuffered"
		info.bufferSize = 0
	} else if len(call.Args) >= 2 {
		info.chanType = "buffered"
		if lit, ok := call.Args[1].(*ast.BasicLit); ok && lit.Kind == token.INT {
			// Parse buffer size if it's a literal
			if size := lit.Value; size == "0" {
				info.chanType = "unbuffered"
			} else {
				info.bufferSize = 1 // Default to 1 if we can't parse
			}
		}
	}
	
	return info
}

func isChanType(expr ast.Expr) bool {
	_, ok := expr.(*ast.ChanType)
	return ok
}

func isChanExpression(expr ast.Expr) bool {
	// Simple check - could be improved
	switch expr.(type) {
	case *ast.Ident, *ast.SelectorExpr:
		return true
	}
	return false
}

func extractChannelName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return exprToString(e)
	default:
		return "unknown"
	}
}

func analyzeSelectStatement(sel *ast.SelectStmt, fset *token.FileSet, src []byte, analysis *ChannelAnalysis, channelVars map[string]*ChannelInfo) {
	pos := fset.Position(sel.Pos())
	hasDefault := false
	
	for _, clause := range sel.Body.List {
		comm, ok := clause.(*ast.CommClause)
		if !ok {
			continue
		}
		
		if comm.Comm == nil {
			hasDefault = true
			continue
		}
		
		// Analyze communication in select
		switch c := comm.Comm.(type) {
		case *ast.SendStmt:
			chanName := extractChannelName(c.Chan)
			usage := ChannelUsage{
				Name:     chanName,
				Type:     "select",
				Position: newPosition(fset.Position(c.Pos())),
				Context:  "select send",
			}
			if info, ok := channelVars[chanName]; ok {
				usage.ChannelType = info.chanType
			}
			analysis.Channels = append(analysis.Channels, usage)
		
		case *ast.AssignStmt:
			// Receive in select
			if len(c.Rhs) > 0 {
				if unary, ok := c.Rhs[0].(*ast.UnaryExpr); ok && unary.Op == token.ARROW {
					chanName := extractChannelName(unary.X)
					usage := ChannelUsage{
						Name:     chanName,
						Type:     "select",
						Position: newPosition(fset.Position(c.Pos())),
						Context:  "select receive",
					}
					if info, ok := channelVars[chanName]; ok {
						usage.ChannelType = info.chanType
					}
					analysis.Channels = append(analysis.Channels, usage)
				}
			}
		}
	}
	
	if !hasDefault && len(sel.Body.List) == 1 {
		issue := ChannelIssue{
			Type:        "single_case_select",
			Description: "Select with single case and no default - consider using simple channel operation",
			Position:    newPosition(pos),
		}
		analysis.Issues = append(analysis.Issues, issue)
	}
}

func isInMainGoroutine(file *ast.File, target ast.Node) bool {
	// Check if node is not inside a goroutine
	var inGoroutine bool
	ast.Inspect(file, func(n ast.Node) bool {
		if _, ok := n.(*ast.GoStmt); ok {
			if containsNode(n, target) {
				inGoroutine = true
				return false
			}
		}
		return true
	})
	return !inGoroutine
}

func hasGoroutineNearby(file *ast.File, target ast.Node) bool {
	// Check if there's a goroutine in the same function
	var hasGo bool
	ast.Inspect(file, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok && containsNode(fn, target) {
			ast.Inspect(fn, func(inner ast.Node) bool {
				if _, ok := inner.(*ast.GoStmt); ok {
					hasGo = true
					return false
				}
				return true
			})
			return false
		}
		return true
	})
	return hasGo
}