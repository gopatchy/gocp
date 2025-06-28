package main

import (
	"go/ast"
	"go/token"
	"crypto/md5"
	"fmt"
	"strings"
)

// Dead code analysis types
type DeadCodeInfo struct {
	File           string         `json:"file"`
	UnusedVars     []UnusedItem   `json:"unused_vars,omitempty"`
	UnreachableCode []CodeLocation `json:"unreachable_code,omitempty"`
	DeadBranches   []CodeLocation `json:"dead_branches,omitempty"`
}

type UnusedItem struct {
	Name     string   `json:"name"`
	Type     string   `json:"type"`
	Position Position `json:"position"`
}

type CodeLocation struct {
	Description string   `json:"description"`
	Position    Position `json:"position"`
}

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

// Performance inefficiency types
type InefficiencyInfo struct {
	File           string               `json:"file"`
	StringConcat   []InefficiencyItem   `json:"string_concat,omitempty"`
	Conversions    []InefficiencyItem   `json:"unnecessary_conversions,omitempty"`
	Allocations    []InefficiencyItem   `json:"potential_allocations,omitempty"`
}

type InefficiencyItem struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Suggestion  string   `json:"suggestion"`
	Position    Position `json:"position"`
}

// API analysis types
type ApiInfo struct {
	Package   string        `json:"package"`
	Functions []ApiFunction `json:"functions"`
	Types     []ApiType     `json:"types"`
	Constants []ApiConstant `json:"constants"`
	Variables []ApiVariable `json:"variables"`
}

type ApiFunction struct {
	Name      string   `json:"name"`
	Signature string   `json:"signature"`
	Doc       string   `json:"doc,omitempty"`
	Position  Position `json:"position"`
}

type ApiType struct {
	Name     string   `json:"name"`
	Kind     string   `json:"kind"`
	Doc      string   `json:"doc,omitempty"`
	Position Position `json:"position"`
}

type ApiConstant struct {
	Name     string   `json:"name"`
	Type     string   `json:"type"`
	Value    string   `json:"value,omitempty"`
	Doc      string   `json:"doc,omitempty"`
	Position Position `json:"position"`
}

type ApiVariable struct {
	Name     string   `json:"name"`
	Type     string   `json:"type"`
	Doc      string   `json:"doc,omitempty"`
	Position Position `json:"position"`
}

// Documentation types
type DocInfo struct {
	Package   string        `json:"package"`
	Overview  string        `json:"overview"`
	Functions []DocFunction `json:"functions"`
	Types     []DocType     `json:"types"`
}

type DocFunction struct {
	Name        string   `json:"name"`
	Signature   string   `json:"signature"`
	Description string   `json:"description"`
	Parameters  []string `json:"parameters,omitempty"`
	Returns     []string `json:"returns,omitempty"`
	Examples    []string `json:"examples,omitempty"`
	Position    Position `json:"position"`
}

type DocType struct {
	Name        string      `json:"name"`
	Kind        string      `json:"kind"`
	Description string      `json:"description"`
	Fields      []DocField  `json:"fields,omitempty"`
	Methods     []DocMethod `json:"methods,omitempty"`
	Position    Position    `json:"position"`
}

type DocField struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

type DocMethod struct {
	Name        string `json:"name"`
	Signature   string `json:"signature"`
	Description string `json:"description"`
}

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

// Coupling analysis types
type CouplingInfo struct {
	Package        string            `json:"package"`
	Afferent       int               `json:"afferent"`
	Efferent       int               `json:"efferent"`
	Instability    float64           `json:"instability"`
	Dependencies   []string          `json:"dependencies"`
	Dependents     []string          `json:"dependents"`
	Suggestions    []string          `json:"suggestions,omitempty"`
}

// Design pattern types
type PatternInfo struct {
	Pattern     string             `json:"pattern"`
	Occurrences []PatternOccurrence `json:"occurrences"`
}

type PatternOccurrence struct {
	File        string   `json:"file"`
	Description string   `json:"description"`
	Quality     string   `json:"quality"`
	Position    Position `json:"position"`
}

// Architecture analysis types
type ArchitectureInfo struct {
	Layers       []LayerInfo       `json:"layers"`
	Violations   []LayerViolation  `json:"violations,omitempty"`
	Suggestions  []string          `json:"suggestions,omitempty"`
}

type LayerInfo struct {
	Name         string   `json:"name"`
	Packages     []string `json:"packages"`
	Dependencies []string `json:"dependencies"`
}

type LayerViolation struct {
	From        string   `json:"from"`
	To          string   `json:"to"`
	Violation   string   `json:"violation"`
	Position    Position `json:"position"`
}

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

// Embedding analysis types
type EmbeddingInfo struct {
	File         string              `json:"file"`
	Structs      []StructEmbedding   `json:"structs,omitempty"`
	Interfaces   []InterfaceEmbedding `json:"interfaces,omitempty"`
}

type StructEmbedding struct {
	Name        string   `json:"name"`
	Embedded    []string `json:"embedded"`
	Methods     []string `json:"promoted_methods"`
	Position    Position `json:"position"`
}

type InterfaceEmbedding struct {
	Name        string   `json:"name"`
	Embedded    []string `json:"embedded"`
	Methods     []string `json:"methods"`
	Position    Position `json:"position"`
}

// Test quality types
type TestQualityInfo struct {
	File         string           `json:"file"`
	TestMetrics  TestMetrics      `json:"metrics"`
	Issues       []TestIssue      `json:"issues,omitempty"`
	Suggestions  []string         `json:"suggestions,omitempty"`
}

type TestMetrics struct {
	TotalTests    int     `json:"total_tests"`
	TableDriven   int     `json:"table_driven"`
	Benchmarks    int     `json:"benchmarks"`
	Examples      int     `json:"examples"`
	Coverage      float64 `json:"estimated_coverage"`
}

type TestIssue struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Severity    string   `json:"severity"`
	Position    Position `json:"position"`
}

// Missing tests types
type MissingTestInfo struct {
	Function    string   `json:"function"`
	Package     string   `json:"package"`
	Complexity  int      `json:"complexity"`
	Criticality string   `json:"criticality"`
	Reason      string   `json:"reason"`
	Position    Position `json:"position"`
}

func findDeadCode(dir string) ([]DeadCodeInfo, error) {
	var deadCode []DeadCodeInfo

	err := walkGoFiles(dir, func(path string, src []byte, file *ast.File, fset *token.FileSet) error {
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}

		info := DeadCodeInfo{
			File: path,
		}

		// Track variable usage
		declaredVars := make(map[string]*ast.ValueSpec)
		usedVars := make(map[string]bool)

		// First pass: collect declared variables
		ast.Inspect(file, func(n ast.Node) bool {
			if genDecl, ok := n.(*ast.GenDecl); ok && genDecl.Tok == token.VAR {
				for _, spec := range genDecl.Specs {
					if valueSpec, ok := spec.(*ast.ValueSpec); ok {
						for _, name := range valueSpec.Names {
							if name.Name != "_" && !ast.IsExported(name.Name) {
								declaredVars[name.Name] = valueSpec
							}
						}
					}
				}
			}
			return true
		})

		// Second pass: track usage
		ast.Inspect(file, func(n ast.Node) bool {
			if ident, ok := n.(*ast.Ident); ok {
				usedVars[ident.Name] = true
			}
			return true
		})

		// Find unused variables
		for varName, valueSpec := range declaredVars {
			if !usedVars[varName] {
				for _, name := range valueSpec.Names {
					if name.Name == varName {
						pos := fset.Position(name.Pos())
						info.UnusedVars = append(info.UnusedVars, UnusedItem{
							Name:     varName,
							Type:     "variable",
							Position: newPosition(pos),
						})
					}
				}
			}
		}

		// Find unreachable code (simplified detection)
		ast.Inspect(file, func(n ast.Node) bool {
			if blockStmt, ok := n.(*ast.BlockStmt); ok {
				for i, stmt := range blockStmt.List {
					if _, ok := stmt.(*ast.ReturnStmt); ok && i < len(blockStmt.List)-1 {
						pos := fset.Position(blockStmt.List[i+1].Pos())
						info.UnreachableCode = append(info.UnreachableCode, CodeLocation{
							Description: "Code after return statement",
							Position:    newPosition(pos),
						})
					}
				}
			}
			return true
		})

		if len(info.UnusedVars) > 0 || len(info.UnreachableCode) > 0 || len(info.DeadBranches) > 0 {
			deadCode = append(deadCode, info)
		}
		return nil
	})

	return deadCode, err
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

func findInefficiencies(dir string) ([]InefficiencyInfo, error) {
	var inefficiencies []InefficiencyInfo

	err := walkGoFiles(dir, func(path string, src []byte, file *ast.File, fset *token.FileSet) error {
		info := InefficiencyInfo{
			File: path,
		}

		ast.Inspect(file, func(n ast.Node) bool {
			// Find string concatenation in loops
			if forStmt, ok := n.(*ast.ForStmt); ok {
				ast.Inspect(forStmt.Body, func(inner ast.Node) bool {
					if binExpr, ok := inner.(*ast.BinaryExpr); ok && binExpr.Op == token.ADD {
						if isStringType(binExpr.X) || isStringType(binExpr.Y) {
							pos := fset.Position(binExpr.Pos())
							info.StringConcat = append(info.StringConcat, InefficiencyItem{
								Type:        "string_concatenation_in_loop",
								Description: "String concatenation in loop can be inefficient",
								Suggestion:  "Consider using strings.Builder",
								Position:    newPosition(pos),
							})
						}
					}
					return true
				})
			}

			// Find unnecessary type conversions
			if callExpr, ok := n.(*ast.CallExpr); ok {
				if len(callExpr.Args) == 1 {
					if ident, ok := callExpr.Fun.(*ast.Ident); ok {
						argType := getExprType(callExpr.Args[0])
						if ident.Name == argType {
							pos := fset.Position(callExpr.Pos())
							info.Conversions = append(info.Conversions, InefficiencyItem{
								Type:        "unnecessary_conversion",
								Description: fmt.Sprintf("Unnecessary conversion to %s", ident.Name),
								Suggestion:  "Remove unnecessary type conversion",
								Position:    newPosition(pos),
							})
						}
					}
				}
			}

			return true
		})

		if len(info.StringConcat) > 0 || len(info.Conversions) > 0 || len(info.Allocations) > 0 {
			inefficiencies = append(inefficiencies, info)
		}
		return nil
	})

	return inefficiencies, err
}

func extractApi(dir string) ([]ApiInfo, error) {
	var apis []ApiInfo

	err := walkGoFiles(dir, func(path string, src []byte, file *ast.File, fset *token.FileSet) error {
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}

		api := ApiInfo{
			Package: file.Name.Name,
		}

		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if ast.IsExported(d.Name.Name) {
					pos := fset.Position(d.Pos())
					api.Functions = append(api.Functions, ApiFunction{
						Name:      d.Name.Name,
						Signature: funcSignature(d.Type),
						Doc:       extractDocString(d.Doc),
						Position:  newPosition(pos),
					})
				}

			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						if ast.IsExported(s.Name.Name) {
							pos := fset.Position(s.Pos())
							kind := "type"
							switch s.Type.(type) {
							case *ast.StructType:
								kind = "struct"
							case *ast.InterfaceType:
								kind = "interface"
							}
							api.Types = append(api.Types, ApiType{
								Name:     s.Name.Name,
								Kind:     kind,
								Doc:      extractDocString(d.Doc),
								Position: newPosition(pos),
							})
						}

					case *ast.ValueSpec:
						for _, name := range s.Names {
							if ast.IsExported(name.Name) {
								pos := fset.Position(name.Pos())
								if d.Tok == token.CONST {
									value := ""
									if len(s.Values) > 0 {
										value = exprToString(s.Values[0])
									}
									api.Constants = append(api.Constants, ApiConstant{
										Name:     name.Name,
										Type:     exprToString(s.Type),
										Value:    value,
										Doc:      extractDocString(d.Doc),
										Position: newPosition(pos),
									})
								} else {
									api.Variables = append(api.Variables, ApiVariable{
										Name:     name.Name,
										Type:     exprToString(s.Type),
										Doc:      extractDocString(d.Doc),
										Position: newPosition(pos),
									})
								}
							}
						}
					}
				}
			}
		}

		if len(api.Functions) > 0 || len(api.Types) > 0 || len(api.Constants) > 0 || len(api.Variables) > 0 {
			apis = append(apis, api)
		}
		return nil
	})

	return apis, err
}

func generateDocs(dir string, format string) (interface{}, error) {
	if format == "markdown" {
		return generateMarkdownDocs(dir)
	}
	return generateJsonDocs(dir)
}

func generateMarkdownDocs(dir string) (string, error) {
	apis, err := extractApi(dir)
	if err != nil {
		return "", err
	}

	var markdown strings.Builder
	for _, api := range apis {
		markdown.WriteString(fmt.Sprintf("# Package %s\n\n", api.Package))

		if len(api.Functions) > 0 {
			markdown.WriteString("## Functions\n\n")
			for _, fn := range api.Functions {
				markdown.WriteString(fmt.Sprintf("### %s\n\n", fn.Name))
				markdown.WriteString(fmt.Sprintf("```go\n%s\n```\n\n", fn.Signature))
				if fn.Doc != "" {
					markdown.WriteString(fmt.Sprintf("%s\n\n", fn.Doc))
				}
			}
		}

		if len(api.Types) > 0 {
			markdown.WriteString("## Types\n\n")
			for _, typ := range api.Types {
				markdown.WriteString(fmt.Sprintf("### %s\n\n", typ.Name))
				if typ.Doc != "" {
					markdown.WriteString(fmt.Sprintf("%s\n\n", typ.Doc))
				}
			}
		}
	}

	return markdown.String(), nil
}

func generateJsonDocs(dir string) ([]DocInfo, error) {
	apis, err := extractApi(dir)
	if err != nil {
		return nil, err
	}

	var docs []DocInfo
	for _, api := range apis {
		doc := DocInfo{
			Package: api.Package,
		}

		for _, fn := range api.Functions {
			doc.Functions = append(doc.Functions, DocFunction{
				Name:        fn.Name,
				Signature:   fn.Signature,
				Description: fn.Doc,
				Position:    fn.Position,
			})
		}

		for _, typ := range api.Types {
			doc.Types = append(doc.Types, DocType{
				Name:        typ.Name,
				Kind:        typ.Kind,
				Description: typ.Doc,
				Position:    typ.Position,
			})
		}

		docs = append(docs, doc)
	}

	return docs, nil
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

func analyzeCoupling(dir string) ([]CouplingInfo, error) {
	var coupling []CouplingInfo

	// This is a simplified implementation
	packages, err := listPackages(dir, false)
	if err != nil {
		return nil, err
	}

	for _, pkg := range packages {
		info := CouplingInfo{
			Package:      pkg.Name,
			Dependencies: pkg.Imports,
			Efferent:     len(pkg.Imports),
		}

		// Calculate instability (Ce / (Ca + Ce))
		if info.Afferent+info.Efferent > 0 {
			info.Instability = float64(info.Efferent) / float64(info.Afferent+info.Efferent)
		}

		coupling = append(coupling, info)
	}

	return coupling, nil
}

func findPatterns(dir string) ([]PatternInfo, error) {
	var patterns []PatternInfo

	err := walkGoFiles(dir, func(path string, src []byte, file *ast.File, fset *token.FileSet) error {
		// Look for singleton pattern
		singletonPattern := PatternInfo{Pattern: "singleton"}
		
		// Look for factory pattern
		factoryPattern := PatternInfo{Pattern: "factory"}

		ast.Inspect(file, func(n ast.Node) bool {
			if fn, ok := n.(*ast.FuncDecl); ok {
				name := strings.ToLower(fn.Name.Name)
				
				// Detect factory pattern
				if strings.HasPrefix(name, "new") || strings.HasPrefix(name, "create") {
					pos := fset.Position(fn.Pos())
					factoryPattern.Occurrences = append(factoryPattern.Occurrences, PatternOccurrence{
						File:        path,
						Description: fmt.Sprintf("Factory function: %s", fn.Name.Name),
						Quality:     "good",
						Position:    newPosition(pos),
					})
				}

				// Detect singleton pattern (simplified)
				if strings.Contains(name, "instance") && fn.Type.Results != nil {
					pos := fset.Position(fn.Pos())
					singletonPattern.Occurrences = append(singletonPattern.Occurrences, PatternOccurrence{
						File:        path,
						Description: fmt.Sprintf("Potential singleton: %s", fn.Name.Name),
						Quality:     "review",
						Position:    newPosition(pos),
					})
				}
			}
			return true
		})

		if len(singletonPattern.Occurrences) > 0 {
			patterns = append(patterns, singletonPattern)
		}
		if len(factoryPattern.Occurrences) > 0 {
			patterns = append(patterns, factoryPattern)
		}

		return nil
	})

	return patterns, err
}

func analyzeArchitecture(dir string) (*ArchitectureInfo, error) {
	// Simplified architecture analysis
	packages, err := listPackages(dir, false)
	if err != nil {
		return nil, err
	}

	arch := &ArchitectureInfo{
		Layers: []LayerInfo{},
	}

	// Detect common Go project structure
	for _, pkg := range packages {
		layer := LayerInfo{
			Name:         pkg.Name,
			Packages:     []string{pkg.ImportPath},
			Dependencies: pkg.Imports,
		}
		arch.Layers = append(arch.Layers, layer)
	}

	return arch, nil
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

func analyzeEmbedding(dir string) ([]EmbeddingInfo, error) {
	var embedding []EmbeddingInfo

	err := walkGoFiles(dir, func(path string, src []byte, file *ast.File, fset *token.FileSet) error {
		info := EmbeddingInfo{
			File: path,
		}

		ast.Inspect(file, func(n ast.Node) bool {
			switch decl := n.(type) {
			case *ast.GenDecl:
				for _, spec := range decl.Specs {
					if ts, ok := spec.(*ast.TypeSpec); ok {
						if st, ok := ts.Type.(*ast.StructType); ok {
							pos := fset.Position(ts.Pos())
							structEmb := StructEmbedding{
								Name:     ts.Name.Name,
								Position: newPosition(pos),
							}

							for _, field := range st.Fields.List {
								if len(field.Names) == 0 {
									structEmb.Embedded = append(structEmb.Embedded, exprToString(field.Type))
								}
							}

							if len(structEmb.Embedded) > 0 {
								info.Structs = append(info.Structs, structEmb)
							}
						}

						if it, ok := ts.Type.(*ast.InterfaceType); ok {
							pos := fset.Position(ts.Pos())
							ifaceEmb := InterfaceEmbedding{
								Name:     ts.Name.Name,
								Position: newPosition(pos),
							}

							for _, method := range it.Methods.List {
								if len(method.Names) == 0 {
									ifaceEmb.Embedded = append(ifaceEmb.Embedded, exprToString(method.Type))
								} else {
									for _, name := range method.Names {
										ifaceEmb.Methods = append(ifaceEmb.Methods, name.Name)
									}
								}
							}

							if len(ifaceEmb.Embedded) > 0 || len(ifaceEmb.Methods) > 0 {
								info.Interfaces = append(info.Interfaces, ifaceEmb)
							}
						}
					}
				}
			}
			return true
		})

		if len(info.Structs) > 0 || len(info.Interfaces) > 0 {
			embedding = append(embedding, info)
		}
		return nil
	})

	return embedding, err
}

func analyzeTestQuality(dir string) ([]TestQualityInfo, error) {
	var testQuality []TestQualityInfo

	err := walkGoFiles(dir, func(path string, src []byte, file *ast.File, fset *token.FileSet) error {
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}

		info := TestQualityInfo{
			File: path,
		}

		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok {
				name := fn.Name.Name
				if strings.HasPrefix(name, "Test") {
					info.TestMetrics.TotalTests++

					// Check for table-driven tests
					if hasTableDrivenPattern(fn) {
						info.TestMetrics.TableDriven++
					}

					// Check for proper assertions
					if !hasProperAssertions(fn) {
						pos := fset.Position(fn.Pos())
						info.Issues = append(info.Issues, TestIssue{
							Type:        "weak_assertions",
							Description: "Test lacks proper assertions",
							Severity:    "medium",
							Position:    newPosition(pos),
						})
					}
				} else if strings.HasPrefix(name, "Benchmark") {
					info.TestMetrics.Benchmarks++
				} else if strings.HasPrefix(name, "Example") {
					info.TestMetrics.Examples++
				}
			}
		}

		if info.TestMetrics.TotalTests > 0 {
			testQuality = append(testQuality, info)
		}
		return nil
	})

	return testQuality, err
}

func findMissingTests(dir string) ([]MissingTestInfo, error) {
	var missingTests []MissingTestInfo

	// Get all exported functions
	exportedFuncs := make(map[string]*ExportedFunc)
	testedFuncs := make(map[string]bool)

	// Collect exported functions
	err := walkGoFiles(dir, func(path string, src []byte, file *ast.File, fset *token.FileSet) error {
		if strings.HasSuffix(path, "_test.go") {
			// Track tested functions
			for _, decl := range file.Decls {
				if fn, ok := decl.(*ast.FuncDecl); ok && strings.HasPrefix(fn.Name.Name, "Test") {
					testedFunc := strings.TrimPrefix(fn.Name.Name, "Test")
					testedFuncs[file.Name.Name+"."+testedFunc] = true
				}
			}
			return nil
		}

		// Collect exported functions
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && ast.IsExported(fn.Name.Name) {
				pos := fset.Position(fn.Pos())
				key := file.Name.Name + "." + fn.Name.Name
				exportedFuncs[key] = &ExportedFunc{
					Name:     fn.Name.Name,
					Package:  file.Name.Name,
					Position: newPosition(pos),
				}
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Find missing tests
	for key, fn := range exportedFuncs {
		if !testedFuncs[key] {
			complexity := calculateComplexity(fn.Name)
			criticality := determineCriticality(fn.Name)
			
			missingTests = append(missingTests, MissingTestInfo{
				Function:    fn.Name,
				Package:     fn.Package,
				Complexity:  complexity,
				Criticality: criticality,
				Reason:      "No test found for exported function",
				Position:    fn.Position,
			})
		}
	}

	return missingTests, nil
}

// Helper functions

func extractFunctionBody(body *ast.BlockStmt, fset *token.FileSet) string {
	start := fset.Position(body.Pos())
	end := fset.Position(body.End())
	return fmt.Sprintf("%d-%d", start.Line, end.Line)
}

func isStringType(expr ast.Expr) bool {
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name == "string"
	}
	return false
}

func getExprType(expr ast.Expr) string {
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name
	}
	return "unknown"
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

func isValidReceiverName(name string) bool {
	return len(name) <= 2 && strings.ToLower(name) == name
}

func shouldHaveContext(fn *ast.FuncDecl) bool {
	// Simple heuristic: functions that might do I/O
	name := strings.ToLower(fn.Name.Name)
	return strings.Contains(name, "get") || strings.Contains(name, "fetch") || 
		   strings.Contains(name, "load") || strings.Contains(name, "save")
}

func hasTableDrivenPattern(fn *ast.FuncDecl) bool {
	// Look for table-driven test patterns
	found := false
	ast.Inspect(fn, func(n ast.Node) bool {
		if genDecl, ok := n.(*ast.GenDecl); ok {
			for _, spec := range genDecl.Specs {
				if valueSpec, ok := spec.(*ast.ValueSpec); ok {
					for _, name := range valueSpec.Names {
						if strings.Contains(name.Name, "test") || strings.Contains(name.Name, "case") {
							found = true
						}
					}
				}
			}
		}
		return true
	})
	return found
}

func hasProperAssertions(fn *ast.FuncDecl) bool {
	// Look for testing.T calls
	found := false
	ast.Inspect(fn, func(n ast.Node) bool {
		if callExpr, ok := n.(*ast.CallExpr); ok {
			if selExpr, ok := callExpr.Fun.(*ast.SelectorExpr); ok {
				if ident, ok := selExpr.X.(*ast.Ident); ok && ident.Name == "t" {
					if selExpr.Sel.Name == "Error" || selExpr.Sel.Name == "Fatal" || 
					   selExpr.Sel.Name == "Fail" {
						found = true
					}
				}
			}
		}
		return true
	})
	return found
}

func calculateComplexity(funcName string) int {
	// Simplified complexity calculation
	return len(funcName) % 10 + 1
}

func determineCriticality(funcName string) string {
	name := strings.ToLower(funcName)
	if strings.Contains(name, "delete") || strings.Contains(name, "remove") {
		return "high"
	}
	if strings.Contains(name, "create") || strings.Contains(name, "update") {
		return "medium"
	}
	return "low"
}