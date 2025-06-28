package main

import (
	"go/ast"
	"go/token"
	"strings"
)

type EmptyBlock struct {
	Type        string   `json:"type"` // "if", "else", "for", "switch_case", "function", etc.
	Description string   `json:"description"`
	Position    Position `json:"position"`
	Context     string   `json:"context"`
}

type EmptyBlockAnalysis struct {
	EmptyBlocks []EmptyBlock       `json:"empty_blocks"`
	Issues      []EmptyBlockIssue  `json:"issues"`
}

type EmptyBlockIssue struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Position    Position `json:"position"`
}

func findEmptyBlocks(dir string) (*EmptyBlockAnalysis, error) {
	analysis := &EmptyBlockAnalysis{
		EmptyBlocks: []EmptyBlock{},
		Issues:      []EmptyBlockIssue{},
	}

	err := walkGoFiles(dir, func(path string, src []byte, file *ast.File, fset *token.FileSet) error {
		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.IfStmt:
				analyzeIfStatement(node, fset, src, analysis)

			case *ast.ForStmt:
				if isEmptyBlock(node.Body) {
					pos := fset.Position(node.Pos())
					empty := EmptyBlock{
						Type:        "for",
						Description: "Empty for loop",
						Position:    newPosition(pos),
						Context:     extractContext(src, pos),
					}
					analysis.EmptyBlocks = append(analysis.EmptyBlocks, empty)

					// Check if it's an infinite loop
					if node.Cond == nil && node.Init == nil && node.Post == nil {
						issue := EmptyBlockIssue{
							Type:        "empty_infinite_loop",
							Description: "Empty infinite loop - possible bug or incomplete implementation",
							Position:    newPosition(pos),
						}
						analysis.Issues = append(analysis.Issues, issue)
					}
				}

			case *ast.RangeStmt:
				if isEmptyBlock(node.Body) {
					pos := fset.Position(node.Pos())
					empty := EmptyBlock{
						Type:        "range",
						Description: "Empty range loop",
						Position:    newPosition(pos),
						Context:     extractContext(src, pos),
					}
					analysis.EmptyBlocks = append(analysis.EmptyBlocks, empty)
				}

			case *ast.SwitchStmt:
				analyzeSwitchStatement(node, fset, src, analysis)

			case *ast.TypeSwitchStmt:
				analyzeTypeSwitchStatement(node, fset, src, analysis)

			case *ast.FuncDecl:
				if node.Body != nil && isEmptyBlock(node.Body) {
					pos := fset.Position(node.Pos())
					empty := EmptyBlock{
						Type:        "function",
						Description: "Empty function: " + node.Name.Name,
						Position:    newPosition(pos),
						Context:     extractContext(src, pos),
					}
					analysis.EmptyBlocks = append(analysis.EmptyBlocks, empty)

					// Check if it's an interface stub
					if !isInterfaceStub(node) && !isTestHelper(node.Name.Name) {
						issue := EmptyBlockIssue{
							Type:        "empty_function",
							Description: "Function '" + node.Name.Name + "' has empty body",
							Position:    newPosition(pos),
						}
						analysis.Issues = append(analysis.Issues, issue)
					}
				}

			case *ast.BlockStmt:
				// Check for standalone empty blocks
				if isEmptyBlock(node) && !isPartOfControlStructure(file, node) {
					pos := fset.Position(node.Pos())
					empty := EmptyBlock{
						Type:        "block",
						Description: "Empty code block",
						Position:    newPosition(pos),
						Context:     extractContext(src, pos),
					}
					analysis.EmptyBlocks = append(analysis.EmptyBlocks, empty)
				}
			}
			return true
		})

		return nil
	})

	return analysis, err
}

func isEmptyBlock(block *ast.BlockStmt) bool {
	if block == nil {
		return true
	}
	
	// Check if block has no statements
	if len(block.List) == 0 {
		return true
	}
	
	// Check if all statements are empty
	for _, stmt := range block.List {
		if !isEmptyStatement(stmt) {
			return false
		}
	}
	
	return true
}

func isEmptyStatement(stmt ast.Stmt) bool {
	switch s := stmt.(type) {
	case *ast.EmptyStmt:
		return true
	case *ast.BlockStmt:
		return isEmptyBlock(s)
	default:
		return false
	}
}

func analyzeIfStatement(ifStmt *ast.IfStmt, fset *token.FileSet, src []byte, analysis *EmptyBlockAnalysis) {
	// Check if body
	if isEmptyBlock(ifStmt.Body) {
		pos := fset.Position(ifStmt.Pos())
		empty := EmptyBlock{
			Type:        "if",
			Description: "Empty if block",
			Position:    newPosition(pos),
			Context:     extractContext(src, pos),
		}
		analysis.EmptyBlocks = append(analysis.EmptyBlocks, empty)

		// Check if there's an else block
		if ifStmt.Else == nil {
			issue := EmptyBlockIssue{
				Type:        "empty_if_no_else",
				Description: "Empty if block with no else - condition may be unnecessary",
				Position:    newPosition(pos),
			}
			analysis.Issues = append(analysis.Issues, issue)
		}
	}

	// Check else block
	if ifStmt.Else != nil {
		switch elseNode := ifStmt.Else.(type) {
		case *ast.BlockStmt:
			if isEmptyBlock(elseNode) {
				pos := fset.Position(elseNode.Pos())
				empty := EmptyBlock{
					Type:        "else",
					Description: "Empty else block",
					Position:    newPosition(pos),
					Context:     extractContext(src, pos),
				}
				analysis.EmptyBlocks = append(analysis.EmptyBlocks, empty)

				issue := EmptyBlockIssue{
					Type:        "empty_else",
					Description: "Empty else block - can be removed",
					Position:    newPosition(pos),
				}
				analysis.Issues = append(analysis.Issues, issue)
			}
		case *ast.IfStmt:
			// Recursively analyze else if
			analyzeIfStatement(elseNode, fset, src, analysis)
		}
	}
}

func analyzeSwitchStatement(switchStmt *ast.SwitchStmt, fset *token.FileSet, src []byte, analysis *EmptyBlockAnalysis) {
	for _, stmt := range switchStmt.Body.List {
		if caseClause, ok := stmt.(*ast.CaseClause); ok {
			if len(caseClause.Body) == 0 {
				pos := fset.Position(caseClause.Pos())
				caseDesc := "default"
				if len(caseClause.List) > 0 {
					caseDesc = "case"
				}
				
				empty := EmptyBlock{
					Type:        "switch_case",
					Description: "Empty " + caseDesc + " clause",
					Position:    newPosition(pos),
					Context:     extractContext(src, pos),
				}
				analysis.EmptyBlocks = append(analysis.EmptyBlocks, empty)

				// Check if it's not a fallthrough case
				if !hasFallthrough(switchStmt, caseClause) {
					issue := EmptyBlockIssue{
						Type:        "empty_switch_case",
						Description: "Empty " + caseDesc + " clause with no fallthrough",
						Position:    newPosition(pos),
					}
					analysis.Issues = append(analysis.Issues, issue)
				}
			}
		}
	}
}

func analyzeTypeSwitchStatement(typeSwitch *ast.TypeSwitchStmt, fset *token.FileSet, src []byte, analysis *EmptyBlockAnalysis) {
	for _, stmt := range typeSwitch.Body.List {
		if caseClause, ok := stmt.(*ast.CaseClause); ok {
			if len(caseClause.Body) == 0 {
				pos := fset.Position(caseClause.Pos())
				caseDesc := "default"
				if len(caseClause.List) > 0 {
					caseDesc = "type case"
				}
				
				empty := EmptyBlock{
					Type:        "type_switch_case",
					Description: "Empty " + caseDesc + " clause",
					Position:    newPosition(pos),
					Context:     extractContext(src, pos),
				}
				analysis.EmptyBlocks = append(analysis.EmptyBlocks, empty)
			}
		}
	}
}

func isPartOfControlStructure(file *ast.File, block *ast.BlockStmt) bool {
	var isControl bool
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.IfStmt:
			if node.Body == block || node.Else == block {
				isControl = true
				return false
			}
		case *ast.ForStmt:
			if node.Body == block {
				isControl = true
				return false
			}
		case *ast.RangeStmt:
			if node.Body == block {
				isControl = true
				return false
			}
		case *ast.SwitchStmt:
			if node.Body == block {
				isControl = true
				return false
			}
		case *ast.TypeSwitchStmt:
			if node.Body == block {
				isControl = true
				return false
			}
		case *ast.FuncDecl:
			if node.Body == block {
				isControl = true
				return false
			}
		case *ast.FuncLit:
			if node.Body == block {
				isControl = true
				return false
			}
		}
		return true
	})
	return isControl
}

func hasFallthrough(switchStmt *ast.SwitchStmt, caseClause *ast.CaseClause) bool {
	// Check if the previous case has a fallthrough
	var prevCase *ast.CaseClause
	for _, stmt := range switchStmt.Body.List {
		if cc, ok := stmt.(*ast.CaseClause); ok {
			if cc == caseClause && prevCase != nil {
				// Check if previous case ends with fallthrough
				if len(prevCase.Body) > 0 {
					if _, ok := prevCase.Body[len(prevCase.Body)-1].(*ast.BranchStmt); ok {
						return true
					}
				}
			}
			prevCase = cc
		}
	}
	return false
}

func isInterfaceStub(fn *ast.FuncDecl) bool {
	// Check if function has a receiver (method)
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return false
	}

	// Check for common stub patterns in name
	name := fn.Name.Name
	stubPatterns := []string{"Stub", "Mock", "Fake", "Dummy", "NoOp", "Noop"}
	for _, pattern := range stubPatterns {
		if strings.Contains(name, pattern) {
			return true
		}
	}

	// Check if receiver type contains stub patterns
	if len(fn.Recv.List) > 0 {
		recvType := exprToString(fn.Recv.List[0].Type)
		for _, pattern := range stubPatterns {
			if strings.Contains(recvType, pattern) {
				return true
			}
		}
	}

	return false
}

func isTestHelper(name string) bool {
	// Common test helper patterns
	helpers := []string{"setUp", "tearDown", "beforeEach", "afterEach", "beforeAll", "afterAll"}
	nameLower := strings.ToLower(name)
	
	for _, helper := range helpers {
		if strings.ToLower(helper) == nameLower {
			return true
		}
	}
	
	// Check for test-related prefixes
	return strings.HasPrefix(name, "Test") || 
	       strings.HasPrefix(name, "Benchmark") ||
	       strings.HasPrefix(name, "Example")
}