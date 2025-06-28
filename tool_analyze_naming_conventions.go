package main

import (
	"go/ast"
	"go/token"
	"strings"
	"unicode"
)

type NamingViolation struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"` // "function", "variable", "constant", "type", "package"
	Issue       string   `json:"issue"`
	Suggestion  string   `json:"suggestion,omitempty"`
	Position    Position `json:"position"`
}

type NamingAnalysis struct {
	Violations []NamingViolation `json:"violations"`
	Statistics NamingStats       `json:"statistics"`
}

type NamingStats struct {
	TotalSymbols       int `json:"total_symbols"`
	ExportedSymbols    int `json:"exported_symbols"`
	UnexportedSymbols  int `json:"unexported_symbols"`
	ViolationCount     int `json:"violation_count"`
}

func analyzeNamingConventions(dir string) (*NamingAnalysis, error) {
	analysis := &NamingAnalysis{
		Violations: []NamingViolation{},
		Statistics: NamingStats{},
	}

	err := walkGoFiles(dir, func(path string, src []byte, file *ast.File, fset *token.FileSet) error {
		// Check package name
		checkPackageName(file, fset, analysis)

		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.FuncDecl:
				analysis.Statistics.TotalSymbols++
				checkFunctionName(node, fset, analysis)

			case *ast.GenDecl:
				for _, spec := range node.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						analysis.Statistics.TotalSymbols++
						checkTypeName(s, node, fset, analysis)

					case *ast.ValueSpec:
						for _, name := range s.Names {
							analysis.Statistics.TotalSymbols++
							if node.Tok == token.CONST {
								checkConstantName(name, fset, analysis)
							} else {
								checkVariableName(name, fset, analysis)
							}
						}
					}
				}
			}
			return true
		})

		return nil
	})

	analysis.Statistics.ViolationCount = len(analysis.Violations)
	return analysis, err
}

func checkPackageName(file *ast.File, fset *token.FileSet, analysis *NamingAnalysis) {
	name := file.Name.Name
	pos := fset.Position(file.Name.Pos())

	// Package names should be lowercase
	if !isAllLowercase(name) {
		violation := NamingViolation{
			Name:       name,
			Type:       "package",
			Issue:      "Package name should be lowercase",
			Suggestion: strings.ToLower(name),
			Position:   newPosition(pos),
		}
		analysis.Violations = append(analysis.Violations, violation)
	}

	// Check for underscores
	if strings.Contains(name, "_") && name != "main" {
		violation := NamingViolation{
			Name:     name,
			Type:     "package",
			Issue:    "Package name should not contain underscores",
			Position: newPosition(pos),
		}
		analysis.Violations = append(analysis.Violations, violation)
	}
}

func checkFunctionName(fn *ast.FuncDecl, fset *token.FileSet, analysis *NamingAnalysis) {
	name := fn.Name.Name
	pos := fset.Position(fn.Name.Pos())
	isExported := ast.IsExported(name)

	if isExported {
		analysis.Statistics.ExportedSymbols++
	} else {
		analysis.Statistics.UnexportedSymbols++
	}

	// Check receiver naming
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		for _, recv := range fn.Recv.List {
			for _, recvName := range recv.Names {
				checkReceiverName(recvName, recv.Type, fset, analysis)
			}
		}
	}

	// Check CamelCase
	if !isCamelCase(name) && !isSpecialFunction(name) {
		violation := NamingViolation{
			Name:       name,
			Type:       "function",
			Issue:      "Function name should be in CamelCase",
			Suggestion: toCamelCase(name),
			Position:   newPosition(pos),
		}
		analysis.Violations = append(analysis.Violations, violation)
	}

	// Check exported function starts with capital
	if isExported && !unicode.IsUpper(rune(name[0])) {
		violation := NamingViolation{
			Name:     name,
			Type:     "function",
			Issue:    "Exported function should start with capital letter",
			Position: newPosition(pos),
		}
		analysis.Violations = append(analysis.Violations, violation)
	}

	// Check for Get prefix on getters
	if strings.HasPrefix(name, "Get") && fn.Recv != nil && !returnsError(fn) {
		violation := NamingViolation{
			Name:       name,
			Type:       "function",
			Issue:      "Getter methods should not use Get prefix",
			Suggestion: name[3:], // Remove "Get" prefix
			Position:   newPosition(pos),
		}
		analysis.Violations = append(analysis.Violations, violation)
	}
}

func checkTypeName(typeSpec *ast.TypeSpec, genDecl *ast.GenDecl, fset *token.FileSet, analysis *NamingAnalysis) {
	name := typeSpec.Name.Name
	pos := fset.Position(typeSpec.Name.Pos())
	isExported := ast.IsExported(name)

	if isExported {
		analysis.Statistics.ExportedSymbols++
	} else {
		analysis.Statistics.UnexportedSymbols++
	}

	// Check CamelCase
	if !isCamelCase(name) {
		violation := NamingViolation{
			Name:       name,
			Type:       "type",
			Issue:      "Type name should be in CamelCase",
			Suggestion: toCamelCase(name),
			Position:   newPosition(pos),
		}
		analysis.Violations = append(analysis.Violations, violation)
	}

	// Check interface naming
	if _, ok := typeSpec.Type.(*ast.InterfaceType); ok {
		if isExported && !strings.HasSuffix(name, "er") && !isWellKnownInterface(name) {
			// Only suggest for single-method interfaces
			if iface, ok := typeSpec.Type.(*ast.InterfaceType); ok && len(iface.Methods.List) == 1 {
				violation := NamingViolation{
					Name:     name,
					Type:     "type",
					Issue:    "Single-method interface should end with 'er'",
					Position: newPosition(pos),
				}
				analysis.Violations = append(analysis.Violations, violation)
			}
		}
	}
}

func checkConstantName(name *ast.Ident, fset *token.FileSet, analysis *NamingAnalysis) {
	pos := fset.Position(name.Pos())
	isExported := ast.IsExported(name.Name)

	if isExported {
		analysis.Statistics.ExportedSymbols++
	} else {
		analysis.Statistics.UnexportedSymbols++
	}

	// Constants can be CamelCase or ALL_CAPS
	if !isCamelCase(name.Name) && !isAllCaps(name.Name) {
		violation := NamingViolation{
			Name:       name.Name,
			Type:       "constant",
			Issue:      "Constant should be in CamelCase or ALL_CAPS",
			Suggestion: toCamelCase(name.Name),
			Position:   newPosition(pos),
		}
		analysis.Violations = append(analysis.Violations, violation)
	}
}

func checkVariableName(name *ast.Ident, fset *token.FileSet, analysis *NamingAnalysis) {
	pos := fset.Position(name.Pos())
	isExported := ast.IsExported(name.Name)

	if isExported {
		analysis.Statistics.ExportedSymbols++
	} else {
		analysis.Statistics.UnexportedSymbols++
	}

	// Skip blank identifier
	if name.Name == "_" {
		return
	}

	// Check for single letter names (except common ones)
	if len(name.Name) == 1 && !isCommonSingleLetter(name.Name) {
		violation := NamingViolation{
			Name:     name.Name,
			Type:     "variable",
			Issue:    "Single letter variable names should be avoided except for common cases (i, j, k for loops)",
			Position: newPosition(pos),
		}
		analysis.Violations = append(analysis.Violations, violation)
	}

	// Check CamelCase
	if !isCamelCase(name.Name) && len(name.Name) > 1 {
		violation := NamingViolation{
			Name:       name.Name,
			Type:       "variable",
			Issue:      "Variable name should be in camelCase",
			Suggestion: toCamelCase(name.Name),
			Position:   newPosition(pos),
		}
		analysis.Violations = append(analysis.Violations, violation)
	}
}

func checkReceiverName(name *ast.Ident, recvType ast.Expr, fset *token.FileSet, analysis *NamingAnalysis) {
	pos := fset.Position(name.Pos())
	
	// Receiver names should be short
	if len(name.Name) > 3 {
		typeName := extractReceiverTypeName(recvType)
		suggestion := ""
		if typeName != "" && len(typeName) > 0 {
			suggestion = strings.ToLower(string(typeName[0]))
		}
		
		violation := NamingViolation{
			Name:       name.Name,
			Type:       "receiver",
			Issue:      "Receiver name should be a short, typically one-letter abbreviation",
			Suggestion: suggestion,
			Position:   newPosition(pos),
		}
		analysis.Violations = append(analysis.Violations, violation)
	}

	// Check for "self" or "this"
	if name.Name == "self" || name.Name == "this" {
		violation := NamingViolation{
			Name:     name.Name,
			Type:     "receiver",
			Issue:    "Avoid 'self' or 'this' for receiver names",
			Position: newPosition(pos),
		}
		analysis.Violations = append(analysis.Violations, violation)
	}
}

func extractReceiverTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return extractReceiverTypeName(t.X)
	}
	return ""
}

func isCamelCase(name string) bool {
	if len(name) == 0 {
		return false
	}

	// Check for underscores
	if strings.Contains(name, "_") {
		return false
	}

	// Allow all lowercase for short names (like "i", "ok", "err")
	if len(name) <= 3 && isAllLowercase(name) {
		return true
	}

	// Check for proper camelCase/PascalCase
	hasUpper := false
	hasLower := false
	for _, r := range name {
		if unicode.IsUpper(r) {
			hasUpper = true
		} else if unicode.IsLower(r) {
			hasLower = true
		}
	}

	// Single case is ok for short names
	return len(name) <= 3 || (hasUpper && hasLower) || isAllLowercase(name)
}

func isAllLowercase(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) && !unicode.IsLower(r) {
			return false
		}
	}
	return true
}

func isAllCaps(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) && !unicode.IsUpper(r) {
			return false
		}
	}
	return true
}

func toCamelCase(s string) string {
	words := strings.FieldsFunc(s, func(r rune) bool {
		return r == '_' || r == '-'
	})
	
	if len(words) == 0 {
		return s
	}

	// First word stays lowercase for camelCase
	result := strings.ToLower(words[0])
	
	// Capitalize first letter of subsequent words
	for i := 1; i < len(words); i++ {
		if len(words[i]) > 0 {
			result += strings.ToUpper(words[i][:1]) + strings.ToLower(words[i][1:])
		}
	}
	
	return result
}

func isSpecialFunction(name string) bool {
	// Special functions that don't follow normal naming
	special := []string{"init", "main", "String", "Error", "MarshalJSON", "UnmarshalJSON"}
	for _, s := range special {
		if name == s {
			return true
		}
	}
	return false
}

func isWellKnownInterface(name string) bool {
	// Well-known interfaces that don't end in 'er'
	known := []string{"Interface", "Handler", "ResponseWriter", "Context", "Value"}
	for _, k := range known {
		if name == k {
			return true
		}
	}
	return false
}

func isCommonSingleLetter(name string) bool {
	// Common single letter variables that are acceptable
	common := []string{"i", "j", "k", "n", "m", "x", "y", "z", "s", "b", "r", "w", "t"}
	for _, c := range common {
		if name == c {
			return true
		}
	}
	return false
}

func returnsError(fn *ast.FuncDecl) bool {
	if fn.Type.Results == nil {
		return false
	}
	
	for _, result := range fn.Type.Results.List {
		if ident, ok := result.Type.(*ast.Ident); ok && ident.Name == "error" {
			return true
		}
	}
	return false
}