package main

import (
	"go/ast"
	"go/token"
	"path/filepath"
	"regexp"
	"strings"
)

// Import analysis types
type ImportInfo struct {
	Package      string            `json:"package"`
	File         string            `json:"file"`
	Imports      []ImportDetail    `json:"imports"`
	UnusedImports []string         `json:"unused_imports,omitempty"`
}

type ImportDetail struct {
	Path     string   `json:"path"`
	Alias    string   `json:"alias,omitempty"`
	Used     []string `json:"used_symbols,omitempty"`
	Position Position `json:"position"`
}

// Function call types
type FunctionCall struct {
	Caller   string   `json:"caller"`
	Context  string   `json:"context"`
	Position Position `json:"position"`
}

// Struct usage types
type StructUsage struct {
	File         string            `json:"file"`
	Literals     []StructLiteral   `json:"literals,omitempty"`
	FieldAccess  []FieldAccess     `json:"field_access,omitempty"`
	TypeUsage    []TypeUsage       `json:"type_usage,omitempty"`
}

type StructLiteral struct {
	Fields       []string `json:"fields_initialized"`
	IsComposite  bool     `json:"is_composite"`
	Position     Position `json:"position"`
}

type FieldAccess struct {
	Field    string   `json:"field"`
	Context  string   `json:"context"`
	Position Position `json:"position"`
}

type TypeUsage struct {
	Usage    string   `json:"usage"`
	Position Position `json:"position"`
}

// Interface analysis types
type InterfaceInfo struct {
	Name           string               `json:"name"`
	Package        string               `json:"package"`
	Position       Position             `json:"position"`
	Methods        []MethodInfo         `json:"methods"`
	Implementations []ImplementationType `json:"implementations,omitempty"`
}

type ImplementationType struct {
	Type     string   `json:"type"`
	Package  string   `json:"package"`
	Position Position `json:"position"`
}

// Error handling types
type ErrorInfo struct {
	File           string         `json:"file"`
	UnhandledErrors []ErrorContext `json:"unhandled_errors,omitempty"`
	ErrorChecks    []ErrorContext `json:"error_checks,omitempty"`
	ErrorReturns   []ErrorContext `json:"error_returns,omitempty"`
}

type ErrorContext struct {
	Context  string   `json:"context"`
	Type     string   `json:"type"`
	Position Position `json:"position"`
}

// Test analysis types
type TestAnalysis struct {
	TestFiles         []TestFile      `json:"test_files"`
	ExportedFunctions []ExportedFunc  `json:"exported_functions"`
	TestCoverage      TestCoverage    `json:"coverage_summary"`
}

type TestFile struct {
	File      string   `json:"file"`
	Package   string   `json:"package"`
	Tests     []string `json:"tests"`
	Benchmarks []string `json:"benchmarks,omitempty"`
	Examples  []string `json:"examples,omitempty"`
}

type ExportedFunc struct {
	Name     string   `json:"name"`
	Package  string   `json:"package"`
	Tested   bool     `json:"tested"`
	Position Position `json:"position"`
}

type TestCoverage struct {
	TotalExported int     `json:"total_exported"`
	TotalTested   int     `json:"total_tested"`
	Percentage    float64 `json:"percentage"`
}

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

// Dependency analysis types
type DependencyInfo struct {
	Package      string         `json:"package"`
	Dir          string         `json:"dir"`
	Dependencies []string       `json:"dependencies"`
	Dependents   []string       `json:"dependents,omitempty"`
	Cycles       [][]string     `json:"cycles,omitempty"`
}

// Generic types
type GenericInfo struct {
	Name        string       `json:"name"`
	Kind        string       `json:"kind"`
	Package     string       `json:"package"`
	Position    Position     `json:"position"`
	TypeParams  []TypeParam  `json:"type_params"`
	Instances   []Instance   `json:"instances,omitempty"`
}

type TypeParam struct {
	Name       string   `json:"name"`
	Constraint string   `json:"constraint"`
	Position   Position `json:"position"`
}

type Instance struct {
	Types    []string `json:"types"`
	Position Position `json:"position"`
}

func findImports(dir string) ([]ImportInfo, error) {
	var imports []ImportInfo

	err := walkGoFiles(dir, func(path string, src []byte, file *ast.File, fset *token.FileSet) error {
		info := ImportInfo{
			Package: file.Name.Name,
			File:    path,
			Imports: []ImportDetail{},
		}

		// Collect all imports
		importMap := make(map[string]*ImportDetail)
		for _, imp := range file.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)
			pos := fset.Position(imp.Pos())
			detail := &ImportDetail{
				Path:     importPath,
				Position: newPosition(pos),
			}
			if imp.Name != nil {
				detail.Alias = imp.Name.Name
			}
			importMap[importPath] = detail
			info.Imports = append(info.Imports, *detail)
		}

		// Track which imports are used
		usedImports := make(map[string]map[string]bool)
		ast.Inspect(file, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.SelectorExpr:
				if ident, ok := x.X.(*ast.Ident); ok {
					pkgName := ident.Name
					symbol := x.Sel.Name
					
					// Find matching import
					for importPath, detail := range importMap {
						importName := filepath.Base(importPath)
						if detail.Alias != "" && detail.Alias == pkgName {
							if usedImports[importPath] == nil {
								usedImports[importPath] = make(map[string]bool)
							}
							usedImports[importPath][symbol] = true
						} else if importName == pkgName {
							if usedImports[importPath] == nil {
								usedImports[importPath] = make(map[string]bool)
							}
							usedImports[importPath][symbol] = true
						}
					}
				}
			}
			return true
		})

		// Update import details with used symbols
		for i, imp := range info.Imports {
			if used, ok := usedImports[imp.Path]; ok {
				for symbol := range used {
					info.Imports[i].Used = append(info.Imports[i].Used, symbol)
				}
			} else if !strings.HasSuffix(imp.Path, "_test") && imp.Alias != "_" {
				info.UnusedImports = append(info.UnusedImports, imp.Path)
			}
		}

		if len(info.Imports) > 0 {
			imports = append(imports, info)
		}
		return nil
	})

	return imports, err
}

func findFunctionCalls(dir string, functionName string) ([]FunctionCall, error) {
	var calls []FunctionCall

	err := walkGoFiles(dir, func(path string, src []byte, file *ast.File, fset *token.FileSet) error {
		currentFunc := ""
		
		ast.Inspect(file, func(n ast.Node) bool {
			// Track current function context
			if fn, ok := n.(*ast.FuncDecl); ok {
				currentFunc = fn.Name.Name
				return true
			}

			// Find function calls
			switch x := n.(type) {
			case *ast.CallExpr:
				var calledName string
				switch fun := x.Fun.(type) {
				case *ast.Ident:
					calledName = fun.Name
				case *ast.SelectorExpr:
					calledName = fun.Sel.Name
				}

				if calledName == functionName {
					pos := fset.Position(x.Pos())
					context := extractContext(src, pos)
					
					calls = append(calls, FunctionCall{
						Caller:   currentFunc,
						Context:  context,
						Position: newPosition(pos),
					})
				}
			}
			return true
		})

		return nil
	})

	return calls, err
}

func findStructUsage(dir string, structName string) ([]StructUsage, error) {
	var usages []StructUsage

	err := walkGoFiles(dir, func(path string, src []byte, file *ast.File, fset *token.FileSet) error {
		usage := StructUsage{
			File: path,
		}

		ast.Inspect(file, func(n ast.Node) bool {
			switch x := n.(type) {
			// Find struct literals
			case *ast.CompositeLit:
				if typeName := getTypeName(x.Type); typeName == structName {
					pos := fset.Position(x.Pos())
					lit := StructLiteral{
						IsComposite: len(x.Elts) > 0,
						Position:    newPosition(pos),
					}
					
					// Extract initialized fields
					for _, elt := range x.Elts {
						if kv, ok := elt.(*ast.KeyValueExpr); ok {
							if ident, ok := kv.Key.(*ast.Ident); ok {
								lit.Fields = append(lit.Fields, ident.Name)
							}
						}
					}
					
					usage.Literals = append(usage.Literals, lit)
				}

			// Find field access
			case *ast.SelectorExpr:
				if typeName := getTypeName(x.X); strings.Contains(typeName, structName) {
					pos := fset.Position(x.Sel.Pos())
					context := extractContext(src, pos)
					
					usage.FieldAccess = append(usage.FieldAccess, FieldAccess{
						Field:    x.Sel.Name,
						Context:  context,
						Position: newPosition(pos),
					})
				}

			// Find type usage in declarations
			case *ast.Field:
				if typeName := getTypeName(x.Type); typeName == structName {
					pos := fset.Position(x.Pos())
					usage.TypeUsage = append(usage.TypeUsage, TypeUsage{
						Usage:    "field",
						Position: newPosition(pos),
					})
				}
			}
			return true
		})

		if len(usage.Literals) > 0 || len(usage.FieldAccess) > 0 || len(usage.TypeUsage) > 0 {
			usages = append(usages, usage)
		}
		return nil
	})

	return usages, err
}

func extractInterfaces(dir string, interfaceName string) ([]InterfaceInfo, error) {
	var interfaces []InterfaceInfo
	interfaceMap := make(map[string]*InterfaceInfo)

	// First pass: collect all interfaces
	err := walkGoFiles(dir, func(path string, src []byte, file *ast.File, fset *token.FileSet) error {
		ast.Inspect(file, func(n ast.Node) bool {
			if genDecl, ok := n.(*ast.GenDecl); ok {
				for _, spec := range genDecl.Specs {
					if typeSpec, ok := spec.(*ast.TypeSpec); ok {
						if iface, ok := typeSpec.Type.(*ast.InterfaceType); ok {
							name := typeSpec.Name.Name
							if interfaceName == "" || name == interfaceName {
								pos := fset.Position(typeSpec.Pos())
								info := &InterfaceInfo{
									Name:     name,
									Package:  file.Name.Name,
									Position: newPosition(pos),
									Methods:  extractInterfaceMethods(iface, fset),
								}
								interfaceMap[name] = info
							}
						}
					}
				}
			}
			return true
		})
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Second pass: find implementations
	if interfaceName != "" {
		iface, exists := interfaceMap[interfaceName]
		if exists {
			err = walkGoFiles(dir, func(path string, src []byte, file *ast.File, fset *token.FileSet) error {
				// Collect all types with methods
				types := make(map[string][]string)
				
				for _, decl := range file.Decls {
					if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv != nil {
						for _, recv := range fn.Recv.List {
							typeName := getTypeName(recv.Type)
							types[typeName] = append(types[typeName], fn.Name.Name)
						}
					}
				}

				// Check if any type implements the interface
				for typeName, methods := range types {
					if implementsInterface(methods, iface.Methods) {
						// Find type declaration
						ast.Inspect(file, func(n ast.Node) bool {
							if genDecl, ok := n.(*ast.GenDecl); ok {
								for _, spec := range genDecl.Specs {
									if typeSpec, ok := spec.(*ast.TypeSpec); ok && typeSpec.Name.Name == typeName {
										pos := fset.Position(typeSpec.Pos())
										iface.Implementations = append(iface.Implementations, ImplementationType{
											Type:     typeName,
											Package:  file.Name.Name,
											Position: newPosition(pos),
										})
									}
								}
							}
							return true
						})
					}
				}
				return nil
			})
		}
	}

	// Convert map to slice
	for _, iface := range interfaceMap {
		interfaces = append(interfaces, *iface)
	}

	return interfaces, err
}

func findErrors(dir string) ([]ErrorInfo, error) {
	var errors []ErrorInfo

	err := walkGoFiles(dir, func(path string, src []byte, file *ast.File, fset *token.FileSet) error {
		info := ErrorInfo{
			File: path,
		}

		ast.Inspect(file, func(n ast.Node) bool {
			switch x := n.(type) {
			// Find function calls that return errors but aren't checked
			case *ast.ExprStmt:
				if call, ok := x.X.(*ast.CallExpr); ok {
					// Check if this function likely returns an error
					if returnsError(call, file) {
						pos := fset.Position(call.Pos())
						context := extractContext(src, pos)
						info.UnhandledErrors = append(info.UnhandledErrors, ErrorContext{
							Context:  context,
							Type:     "unchecked_call",
							Position: newPosition(pos),
						})
					}
				}

			// Find error checks
			case *ast.IfStmt:
				if isErrorCheck(x) {
					pos := fset.Position(x.Pos())
					context := extractContext(src, pos)
					info.ErrorChecks = append(info.ErrorChecks, ErrorContext{
						Context:  context,
						Type:     "error_check",
						Position: newPosition(pos),
					})
				}

			// Find error returns
			case *ast.ReturnStmt:
				for _, result := range x.Results {
					if ident, ok := result.(*ast.Ident); ok && (ident.Name == "err" || strings.Contains(ident.Name, "error")) {
						pos := fset.Position(x.Pos())
						context := extractContext(src, pos)
						info.ErrorReturns = append(info.ErrorReturns, ErrorContext{
							Context:  context,
							Type:     "error_return",
							Position: newPosition(pos),
						})
						break
					}
				}
			}
			return true
		})

		if len(info.UnhandledErrors) > 0 || len(info.ErrorChecks) > 0 || len(info.ErrorReturns) > 0 {
			errors = append(errors, info)
		}
		return nil
	})

	return errors, err
}

func analyzeTests(dir string) (*TestAnalysis, error) {
	analysis := &TestAnalysis{
		TestFiles:         []TestFile{},
		ExportedFunctions: []ExportedFunc{},
	}

	// Collect all exported functions
	exportedFuncs := make(map[string]*ExportedFunc)
	
	err := walkGoFiles(dir, func(path string, src []byte, file *ast.File, fset *token.FileSet) error {
		if strings.HasSuffix(path, "_test.go") {
			// Process test files
			testFile := TestFile{
				File:    path,
				Package: file.Name.Name,
			}

			for _, decl := range file.Decls {
				if fn, ok := decl.(*ast.FuncDecl); ok {
					name := fn.Name.Name
					if strings.HasPrefix(name, "Test") {
						testFile.Tests = append(testFile.Tests, name)
					} else if strings.HasPrefix(name, "Benchmark") {
						testFile.Benchmarks = append(testFile.Benchmarks, name)
					} else if strings.HasPrefix(name, "Example") {
						testFile.Examples = append(testFile.Examples, name)
					}
				}
			}

			if len(testFile.Tests) > 0 || len(testFile.Benchmarks) > 0 || len(testFile.Examples) > 0 {
				analysis.TestFiles = append(analysis.TestFiles, testFile)
			}
		} else {
			// Collect exported functions
			for _, decl := range file.Decls {
				if fn, ok := decl.(*ast.FuncDecl); ok && ast.IsExported(fn.Name.Name) {
					key := file.Name.Name + "." + fn.Name.Name
					pos := fset.Position(fn.Pos())
					exportedFuncs[key] = &ExportedFunc{
						Name:     fn.Name.Name,
						Package:  file.Name.Name,
						Tested:   false,
						Position: newPosition(pos),
					}
				}
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Check which functions are tested
	for _, testFile := range analysis.TestFiles {
		for _, testName := range testFile.Tests {
			// Simple heuristic: TestFunctionName tests FunctionName
			funcName := strings.TrimPrefix(testName, "Test")
			key := testFile.Package + "." + funcName
			if fn, exists := exportedFuncs[key]; exists {
				fn.Tested = true
			}
		}
	}

	// Convert map to slice and calculate coverage
	tested := 0
	for _, fn := range exportedFuncs {
		analysis.ExportedFunctions = append(analysis.ExportedFunctions, *fn)
		if fn.Tested {
			tested++
		}
	}

	analysis.TestCoverage = TestCoverage{
		TotalExported: len(exportedFuncs),
		TotalTested:   tested,
	}
	if len(exportedFuncs) > 0 {
		analysis.TestCoverage.Percentage = float64(tested) / float64(len(exportedFuncs)) * 100
	}

	return analysis, nil
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

func analyzeDependencies(dir string) ([]DependencyInfo, error) {
	depMap := make(map[string]*DependencyInfo)

	// First pass: collect all packages and their imports
	err := walkGoFiles(dir, func(path string, src []byte, file *ast.File, fset *token.FileSet) error {
		pkgDir := filepath.Dir(path)
		if _, exists := depMap[pkgDir]; !exists {
			depMap[pkgDir] = &DependencyInfo{
				Package:      file.Name.Name,
				Dir:          pkgDir,
				Dependencies: []string{},
			}
		}

		// Add imports
		for _, imp := range file.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)
			if !contains(depMap[pkgDir].Dependencies, importPath) {
				depMap[pkgDir].Dependencies = append(depMap[pkgDir].Dependencies, importPath)
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Build dependency graph and find cycles
	var deps []DependencyInfo
	for _, dep := range depMap {
		// Find internal dependencies
		for _, imp := range dep.Dependencies {
			// Check if this is an internal package
			for otherDir, otherDep := range depMap {
				if strings.HasSuffix(imp, otherDep.Package) && otherDir != dep.Dir {
					otherDep.Dependents = append(otherDep.Dependents, dep.Package)
				}
			}
		}
		deps = append(deps, *dep)
	}

	// Simple cycle detection (could be enhanced)
	for i := range deps {
		deps[i].Cycles = findCycles(&deps[i], depMap)
	}

	return deps, nil
}

func findGenerics(dir string) ([]GenericInfo, error) {
	var generics []GenericInfo

	err := walkGoFiles(dir, func(path string, src []byte, file *ast.File, fset *token.FileSet) error {
		ast.Inspect(file, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.GenDecl:
				for _, spec := range x.Specs {
					if ts, ok := spec.(*ast.TypeSpec); ok && ts.TypeParams != nil {
						pos := fset.Position(ts.Pos())
						info := GenericInfo{
							Name:     ts.Name.Name,
							Kind:     "type",
							Package:  file.Name.Name,
							Position: newPosition(pos),
						}

						// Extract type parameters
						for _, param := range ts.TypeParams.List {
							for _, name := range param.Names {
								namePos := fset.Position(name.Pos())
								tp := TypeParam{
									Name:     name.Name,
									Position: newPosition(namePos),
								}
								if param.Type != nil {
									tp.Constraint = exprToString(param.Type)
								}
								info.TypeParams = append(info.TypeParams, tp)
							}
						}

						generics = append(generics, info)
					}
				}

			case *ast.FuncDecl:
				if x.Type.TypeParams != nil {
					pos := fset.Position(x.Pos())
					info := GenericInfo{
						Name:     x.Name.Name,
						Kind:     "function",
						Package:  file.Name.Name,
						Position: newPosition(pos),
					}

					// Extract type parameters
					for _, param := range x.Type.TypeParams.List {
						for _, name := range param.Names {
							namePos := fset.Position(name.Pos())
							tp := TypeParam{
								Name:     name.Name,
								Position: newPosition(namePos),
							}
							if param.Type != nil {
								tp.Constraint = exprToString(param.Type)
							}
							info.TypeParams = append(info.TypeParams, tp)
						}
					}

					generics = append(generics, info)
				}
			}
			return true
		})
		return nil
	})

	return generics, err
}

// Helper functions

func getTypeName(expr ast.Expr) string {
	switch x := expr.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.StarExpr:
		return getTypeName(x.X)
	case *ast.SelectorExpr:
		return exprToString(x)
	}
	return ""
}

func implementsInterface(methods []string, interfaceMethods []MethodInfo) bool {
	for _, im := range interfaceMethods {
		found := false
		for _, m := range methods {
			if m == im.Name {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func returnsError(call *ast.CallExpr, file *ast.File) bool {
	// Simple heuristic: check if the function name suggests it returns an error
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		name := fun.Name
		return strings.HasPrefix(name, "New") || strings.HasPrefix(name, "Create") ||
			strings.HasPrefix(name, "Open") || strings.HasPrefix(name, "Read") ||
			strings.HasPrefix(name, "Write") || strings.HasPrefix(name, "Parse")
	case *ast.SelectorExpr:
		name := fun.Sel.Name
		return strings.HasPrefix(name, "New") || strings.HasPrefix(name, "Create") ||
			strings.HasPrefix(name, "Open") || strings.HasPrefix(name, "Read") ||
			strings.HasPrefix(name, "Write") || strings.HasPrefix(name, "Parse")
	}
	return false
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

func contains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

func findCycles(dep *DependencyInfo, depMap map[string]*DependencyInfo) [][]string {
	// Simple DFS-based cycle detection
	var cycles [][]string
	visited := make(map[string]bool)
	recStack := make(map[string]bool)
	path := []string{}

	var dfs func(pkg string) bool
	dfs = func(pkg string) bool {
		visited[pkg] = true
		recStack[pkg] = true
		path = append(path, pkg)

		// Find dependencies for this package
		for _, d := range depMap {
			if d.Package == pkg {
				for _, imp := range d.Dependencies {
					for _, otherDep := range depMap {
						if strings.HasSuffix(imp, otherDep.Package) {
							if !visited[otherDep.Package] {
								if dfs(otherDep.Package) {
									return true
								}
							} else if recStack[otherDep.Package] {
								// Found a cycle
								cycleStart := -1
								for i, p := range path {
									if p == otherDep.Package {
										cycleStart = i
										break
									}
								}
								if cycleStart >= 0 {
									cycle := append([]string{}, path[cycleStart:]...)
									cycles = append(cycles, cycle)
								}
								return true
							}
						}
					}
				}
				break
			}
		}

		path = path[:len(path)-1]
		recStack[pkg] = false
		return false
	}

	dfs(dep.Package)
	return cycles
}