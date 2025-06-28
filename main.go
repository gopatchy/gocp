package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type RunResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
	Error    string `json:"error,omitempty"`
}

func main() {
	mcpServer := server.NewMCPServer(
		"gocp",
		"1.0.0",
		server.WithToolCapabilities(false),
	)

	// Define the build_and_run_go tool
	buildAndRunTool := mcp.NewTool("build_and_run_go",
		mcp.WithDescription("Build and execute Go code"),
		mcp.WithString("code",
			mcp.Required(),
			mcp.Description("The Go source code to build and run"),
		),
		mcp.WithNumber("timeout",
			mcp.Description("Timeout in seconds (default: 30)"),
		),
	)
	mcpServer.AddTool(buildAndRunTool, buildAndRunHandler)

	// Define the find_symbols tool
	findSymbolsTool := mcp.NewTool("find_symbols",
		mcp.WithDescription("Find all functions, types, interfaces, constants, and variables by name/pattern"),
		mcp.WithString("dir",
			mcp.Description("Directory to search (default: current directory)"),
		),
		mcp.WithString("pattern",
			mcp.Description("Symbol name pattern to search for (case-insensitive substring match)"),
		),
	)
	mcpServer.AddTool(findSymbolsTool, findSymbolsHandler)

	// Define the get_type_info tool
	getTypeInfoTool := mcp.NewTool("get_type_info",
		mcp.WithDescription("Get detailed information about a type including fields, methods, and embedded types"),
		mcp.WithString("dir",
			mcp.Description("Directory to search (default: current directory)"),
		),
		mcp.WithString("type",
			mcp.Required(),
			mcp.Description("Type name to get information for"),
		),
	)
	mcpServer.AddTool(getTypeInfoTool, getTypeInfoHandler)

	// Define the find_references tool
	findReferencesTool := mcp.NewTool("find_references",
		mcp.WithDescription("Find all references to a symbol (function calls, type usage, etc.)"),
		mcp.WithString("dir",
			mcp.Description("Directory to search (default: current directory)"),
		),
		mcp.WithString("symbol",
			mcp.Required(),
			mcp.Description("Symbol name to find references for"),
		),
	)
	mcpServer.AddTool(findReferencesTool, findReferencesHandler)

	// Define the list_packages tool
	listPackagesTool := mcp.NewTool("list_packages",
		mcp.WithDescription("List all Go packages in directory tree"),
		mcp.WithString("dir",
			mcp.Description("Directory to search (default: current directory)"),
		),
		mcp.WithBoolean("include_tests",
			mcp.Description("Include test files in package listings (default: false)"),
		),
	)
	mcpServer.AddTool(listPackagesTool, listPackagesHandler)

	// Define the find_imports tool
	findImportsTool := mcp.NewTool("find_imports",
		mcp.WithDescription("Analyze import usage and find unused imports"),
		mcp.WithString("dir",
			mcp.Description("Directory to search (default: current directory)"),
		),
	)
	mcpServer.AddTool(findImportsTool, findImportsHandler)

	// Define the find_function_calls tool
	findFunctionCallsTool := mcp.NewTool("find_function_calls",
		mcp.WithDescription("Find all calls to a specific function"),
		mcp.WithString("dir",
			mcp.Description("Directory to search (default: current directory)"),
		),
		mcp.WithString("function",
			mcp.Required(),
			mcp.Description("Function name to find calls for"),
		),
	)
	mcpServer.AddTool(findFunctionCallsTool, findFunctionCallsHandler)

	// Define the find_struct_usage tool
	findStructUsageTool := mcp.NewTool("find_struct_usage",
		mcp.WithDescription("Find struct instantiations and field access patterns"),
		mcp.WithString("dir",
			mcp.Description("Directory to search (default: current directory)"),
		),
		mcp.WithString("struct",
			mcp.Required(),
			mcp.Description("Struct name to analyze usage for"),
		),
	)
	mcpServer.AddTool(findStructUsageTool, findStructUsageHandler)

	// Define the extract_interfaces tool
	extractInterfacesTool := mcp.NewTool("extract_interfaces",
		mcp.WithDescription("Find types implementing an interface or suggest interfaces"),
		mcp.WithString("dir",
			mcp.Description("Directory to search (default: current directory)"),
		),
		mcp.WithString("interface",
			mcp.Description("Interface name to find implementations for (if empty, lists all interfaces)"),
		),
	)
	mcpServer.AddTool(extractInterfacesTool, extractInterfacesHandler)

	// Define the find_errors tool
	findErrorsTool := mcp.NewTool("find_errors",
		mcp.WithDescription("Find error handling patterns and unhandled errors"),
		mcp.WithString("dir",
			mcp.Description("Directory to search (default: current directory)"),
		),
	)
	mcpServer.AddTool(findErrorsTool, findErrorsHandler)

	// Define the analyze_tests tool
	analyzeTestsTool := mcp.NewTool("analyze_tests",
		mcp.WithDescription("Analyze test coverage and find untested exported functions"),
		mcp.WithString("dir",
			mcp.Description("Directory to search (default: current directory)"),
		),
	)
	mcpServer.AddTool(analyzeTestsTool, analyzeTestsHandler)

	// Define the find_comments tool
	findCommentsTool := mcp.NewTool("find_comments",
		mcp.WithDescription("Find all comments in Go files, with optional filtering for TODOs or undocumented exports"),
		mcp.WithString("dir",
			mcp.Description("Directory to search (default: current directory)"),
		),
		mcp.WithString("type",
			mcp.Description("Comment type: 'todo' (filters by TODO keywords), 'undocumented' (finds undocumented exports), or 'all' (returns all comments, default)"),
		),
		mcp.WithString("filter",
			mcp.Description("Optional regex to filter comments (applies to 'todo' and 'all' types)"),
		),
	)
	mcpServer.AddTool(findCommentsTool, findCommentsHandler)

	// Define the analyze_dependencies tool
	analyzeDependenciesTool := mcp.NewTool("analyze_dependencies",
		mcp.WithDescription("Analyze package dependencies and find cycles"),
		mcp.WithString("dir",
			mcp.Description("Directory to search (default: current directory)"),
		),
	)
	mcpServer.AddTool(analyzeDependenciesTool, analyzeDependenciesHandler)

	// Define the find_generics tool
	findGenericsTool := mcp.NewTool("find_generics",
		mcp.WithDescription("Find generic types, functions and their instantiations"),
		mcp.WithString("dir",
			mcp.Description("Directory to search (default: current directory)"),
		),
	)
	mcpServer.AddTool(findGenericsTool, findGenericsHandler)

	// Define the find_dead_code tool
	findDeadCodeTool := mcp.NewTool("find_dead_code",
		mcp.WithDescription("Find unreachable code, unused variables, and dead branches"),
		mcp.WithString("dir",
			mcp.Description("Directory to search (default: current directory)"),
		),
	)
	mcpServer.AddTool(findDeadCodeTool, findDeadCodeHandler)

	// Define the find_duplicates tool
	findDuplicatesTool := mcp.NewTool("find_duplicates",
		mcp.WithDescription("Detect code duplication and similar function patterns"),
		mcp.WithString("dir",
			mcp.Description("Directory to search (default: current directory)"),
		),
		mcp.WithNumber("threshold",
			mcp.Description("Similarity threshold (0.0-1.0, default: 0.8)"),
		),
	)
	mcpServer.AddTool(findDuplicatesTool, findDuplicatesHandler)

	// Define the find_inefficiencies tool
	findInefficienciesTool := mcp.NewTool("find_inefficiencies",
		mcp.WithDescription("Find performance inefficiencies like string concatenation in loops"),
		mcp.WithString("dir",
			mcp.Description("Directory to search (default: current directory)"),
		),
	)
	mcpServer.AddTool(findInefficienciesTool, findInefficienciesHandler)

	// Define the extract_api tool
	extractApiTool := mcp.NewTool("extract_api",
		mcp.WithDescription("Extract public API surface and detect breaking changes"),
		mcp.WithString("dir",
			mcp.Description("Directory to search (default: current directory)"),
		),
	)
	mcpServer.AddTool(extractApiTool, extractApiHandler)

	// Define the generate_docs tool
	generateDocsTool := mcp.NewTool("generate_docs",
		mcp.WithDescription("Auto-generate documentation from code structure"),
		mcp.WithString("dir",
			mcp.Description("Directory to search (default: current directory)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: 'markdown' or 'json' (default: 'markdown')"),
		),
	)
	mcpServer.AddTool(generateDocsTool, generateDocsHandler)

	// Define the find_deprecated tool
	findDeprecatedTool := mcp.NewTool("find_deprecated",
		mcp.WithDescription("Find deprecated usage and suggest alternatives"),
		mcp.WithString("dir",
			mcp.Description("Directory to search (default: current directory)"),
		),
	)
	mcpServer.AddTool(findDeprecatedTool, findDeprecatedHandler)

	// Define the analyze_coupling tool
	analyzeCouplingTool := mcp.NewTool("analyze_coupling",
		mcp.WithDescription("Analyze package/module coupling and suggest refactoring"),
		mcp.WithString("dir",
			mcp.Description("Directory to search (default: current directory)"),
		),
	)
	mcpServer.AddTool(analyzeCouplingTool, analyzeCouplingHandler)

	// Define the find_patterns tool
	findPatternsTool := mcp.NewTool("find_patterns",
		mcp.WithDescription("Find design patterns usage (singleton, factory, etc.)"),
		mcp.WithString("dir",
			mcp.Description("Directory to search (default: current directory)"),
		),
	)
	mcpServer.AddTool(findPatternsTool, findPatternsHandler)

	// Define the analyze_architecture tool
	analyzeArchitectureTool := mcp.NewTool("analyze_architecture",
		mcp.WithDescription("Analyze layer violations and dependency direction"),
		mcp.WithString("dir",
			mcp.Description("Directory to search (default: current directory)"),
		),
	)
	mcpServer.AddTool(analyzeArchitectureTool, analyzeArchitectureHandler)

	// Define the analyze_go_idioms tool
	analyzeGoIdiomsTool := mcp.NewTool("analyze_go_idioms",
		mcp.WithDescription("Check for idiomatic Go patterns and suggest improvements"),
		mcp.WithString("dir",
			mcp.Description("Directory to search (default: current directory)"),
		),
	)
	mcpServer.AddTool(analyzeGoIdiomsTool, analyzeGoIdiomsHandler)

	// Define the find_context_usage tool
	findContextUsageTool := mcp.NewTool("find_context_usage",
		mcp.WithDescription("Analyze Context.Context usage patterns and find missing contexts"),
		mcp.WithString("dir",
			mcp.Description("Directory to search (default: current directory)"),
		),
	)
	mcpServer.AddTool(findContextUsageTool, findContextUsageHandler)

	// Define the analyze_embedding tool
	analyzeEmbeddingTool := mcp.NewTool("analyze_embedding",
		mcp.WithDescription("Analyze interface and struct embedding patterns"),
		mcp.WithString("dir",
			mcp.Description("Directory to search (default: current directory)"),
		),
	)
	mcpServer.AddTool(analyzeEmbeddingTool, analyzeEmbeddingHandler)

	// Define the analyze_test_quality tool
	analyzeTestQualityTool := mcp.NewTool("analyze_test_quality",
		mcp.WithDescription("Analyze test patterns, assertion quality, and mock usage"),
		mcp.WithString("dir",
			mcp.Description("Directory to search (default: current directory)"),
		),
	)
	mcpServer.AddTool(analyzeTestQualityTool, analyzeTestQualityHandler)

	// Define the find_missing_tests tool
	findMissingTestsTool := mcp.NewTool("find_missing_tests",
		mcp.WithDescription("Find missing tests based on complexity and criticality"),
		mcp.WithString("dir",
			mcp.Description("Directory to search (default: current directory)"),
		),
	)
	mcpServer.AddTool(findMissingTestsTool, findMissingTestsHandler)

	// Define the read_range tool
	readRangeTool := mcp.NewTool("read_range",
		mcp.WithDescription("Read file content by line/column or byte range"),
		mcp.WithString("file",
			mcp.Required(),
			mcp.Description("File path to read from"),
		),
		mcp.WithNumber("start_line",
			mcp.Description("Start line (1-based, use with end_line)"),
		),
		mcp.WithNumber("end_line",
			mcp.Description("End line (1-based, inclusive)"),
		),
		mcp.WithNumber("start_col",
			mcp.Description("Start column (1-based, optional)"),
		),
		mcp.WithNumber("end_col",
			mcp.Description("End column (1-based, optional)"),
		),
		mcp.WithNumber("start_byte",
			mcp.Description("Start byte offset (0-based, use with end_byte)"),
		),
		mcp.WithNumber("end_byte",
			mcp.Description("End byte offset (0-based, exclusive)"),
		),
	)
	mcpServer.AddTool(readRangeTool, readRangeHandler)

	// Define the write_range tool
	writeRangeTool := mcp.NewTool("write_range",
		mcp.WithDescription("Write content to file at specific line/column or byte range"),
		mcp.WithString("file",
			mcp.Required(),
			mcp.Description("File path to write to"),
		),
		mcp.WithString("content",
			mcp.Required(),
			mcp.Description("Content to write"),
		),
		mcp.WithNumber("start_line",
			mcp.Description("Start line (1-based, use with end_line)"),
		),
		mcp.WithNumber("end_line",
			mcp.Description("End line (1-based, inclusive)"),
		),
		mcp.WithNumber("start_col",
			mcp.Description("Start column (1-based, optional)"),
		),
		mcp.WithNumber("end_col",
			mcp.Description("End column (1-based, optional)"),
		),
		mcp.WithNumber("start_byte",
			mcp.Description("Start byte offset (0-based, use with end_byte)"),
		),
		mcp.WithNumber("end_byte",
			mcp.Description("End byte offset (0-based, exclusive)"),
		),
		mcp.WithString("confirm_old",
			mcp.Description("Expected old content for confirmation before replacing"),
		),
	)
	mcpServer.AddTool(writeRangeTool, writeRangeHandler)

	// Define the search_replace tool
	searchReplaceTool := mcp.NewTool("search_replace",
		mcp.WithDescription("Search and optionally replace text in files. Supports context-aware replacements with capture groups."),
		mcp.WithString("paths",
			mcp.Required(),
			mcp.Description("File/directory path or comma-separated paths to search"),
		),
		mcp.WithString("pattern",
			mcp.Required(),
			mcp.Description("Search pattern (string or regex). Used for simple search unless context_pattern is provided."),
		),
		mcp.WithString("replacement",
			mcp.Description("Replacement text (omit for search-only)"),
		),
		mcp.WithBoolean("regex",
			mcp.Description("Use regex pattern matching (default: false)"),
		),
		mcp.WithBoolean("case_insensitive",
			mcp.Description("Case-insensitive matching (default: false)"),
		),
		mcp.WithBoolean("include_context",
			mcp.Description("Include line context in search results (default: false)"),
		),
		mcp.WithString("context_pattern",
			mcp.Description("Context pattern with capture groups, e.g., '(prefix)(target)(suffix)' to replace only the target"),
		),
		mcp.WithNumber("target_group",
			mcp.Description("Which capture group to replace when using context_pattern (1-based, default: 2 for 3 groups, otherwise last)"),
		),
	)
	mcpServer.AddTool(searchReplaceTool, searchReplaceHandler)

	// Start the server
	if err := server.ServeStdio(mcpServer); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}

func buildAndRunHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	code, err := request.RequireString("code")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	timeout := request.GetFloat("timeout", 30.0)

	stdout, stderr, exitCode, runErr := buildAndRunGo(code, time.Duration(timeout)*time.Second)
	
	result := RunResult{
		Stdout:   stdout,
		Stderr:   stderr,
		ExitCode: exitCode,
	}
	
	if runErr != nil {
		result.Error = runErr.Error()
	}

	jsonData, err := json.Marshal(result)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal result: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

func buildAndRunGo(code string, timeout time.Duration) (stdout, stderr string, exitCode int, err error) {
	tmpDir, err := os.MkdirTemp("", "gocp-*")
	if err != nil {
		return "", "", -1, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	tmpFile := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(tmpFile, []byte(code), 0644); err != nil {
		return "", "", -1, fmt.Errorf("failed to write code: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	modCmd := exec.CommandContext(ctx, "go", "mod", "init", "temp")
	modCmd.Dir = tmpDir
	if err := modCmd.Run(); err != nil {
		return "", "", -1, fmt.Errorf("failed to initialize go.mod: %w", err)
	}

	runCmd := exec.CommandContext(ctx, "go", "run", tmpFile)
	runCmd.Dir = tmpDir
	
	var stdoutBuf, stderrBuf bytes.Buffer
	runCmd.Stdout = &stdoutBuf
	runCmd.Stderr = &stderrBuf
	
	err = runCmd.Run()
	
	exitCode = 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
			err = nil
		} else if ctx.Err() == context.DeadlineExceeded {
			return stdoutBuf.String(), stderrBuf.String(), -1, fmt.Errorf("execution timeout exceeded")
		} else {
			return stdoutBuf.String(), stderrBuf.String(), -1, err
		}
	}

	return stdoutBuf.String(), stderrBuf.String(), exitCode, nil
}

func findSymbolsHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	dir := request.GetString("dir", "./")
	pattern := request.GetString("pattern", "")

	symbols, err := findSymbols(dir, pattern)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to find symbols: %v", err)), nil
	}

	jsonData, err := json.Marshal(symbols)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal symbols: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

func getTypeInfoHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	dir := request.GetString("dir", "./")

	typeName, err := request.RequireString("type")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	info, err := getTypeInfo(dir, typeName)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get type info: %v", err)), nil
	}

	jsonData, err := json.Marshal(info)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal type info: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

func findReferencesHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	dir := request.GetString("dir", "./")

	symbol, err := request.RequireString("symbol")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	refs, err := findReferences(dir, symbol)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to find references: %v", err)), nil
	}

	jsonData, err := json.Marshal(refs)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal references: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

func listPackagesHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	dir := request.GetString("dir", "./")
	includeTests := request.GetBool("include_tests", false)

	packages, err := listPackages(dir, includeTests)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list packages: %v", err)), nil
	}

	jsonData, err := json.Marshal(packages)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal packages: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

func findImportsHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	dir := request.GetString("dir", "./")

	imports, err := findImports(dir)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to analyze imports: %v", err)), nil
	}

	jsonData, err := json.Marshal(imports)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal imports: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

func findFunctionCallsHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	dir := request.GetString("dir", "./")
	function, err := request.RequireString("function")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	calls, err := findFunctionCalls(dir, function)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to find function calls: %v", err)), nil
	}

	jsonData, err := json.Marshal(calls)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal calls: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

func findStructUsageHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	dir := request.GetString("dir", "./")
	structName, err := request.RequireString("struct")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	usage, err := findStructUsage(dir, structName)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to find struct usage: %v", err)), nil
	}

	jsonData, err := json.Marshal(usage)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal usage: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

func extractInterfacesHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	dir := request.GetString("dir", "./")
	interfaceName := request.GetString("interface", "")

	interfaces, err := extractInterfaces(dir, interfaceName)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to extract interfaces: %v", err)), nil
	}

	jsonData, err := json.Marshal(interfaces)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal interfaces: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

func findErrorsHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	dir := request.GetString("dir", "./")

	errors, err := findErrors(dir)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to find errors: %v", err)), nil
	}

	jsonData, err := json.Marshal(errors)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal errors: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

func analyzeTestsHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	dir := request.GetString("dir", "./")

	analysis, err := analyzeTests(dir)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to analyze tests: %v", err)), nil
	}

	jsonData, err := json.Marshal(analysis)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal analysis: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

func findCommentsHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	dir := request.GetString("dir", "./")
	commentType := request.GetString("type", "all")
	filter := request.GetString("filter", "")

	comments, err := findComments(dir, commentType, filter)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to find comments: %v", err)), nil
	}

	jsonData, err := json.Marshal(comments)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal comments: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

func analyzeDependenciesHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	dir := request.GetString("dir", "./")

	deps, err := analyzeDependencies(dir)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to analyze dependencies: %v", err)), nil
	}

	jsonData, err := json.Marshal(deps)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal dependencies: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

func findGenericsHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	dir := request.GetString("dir", "./")

	generics, err := findGenerics(dir)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to find generics: %v", err)), nil
	}

	jsonData, err := json.Marshal(generics)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal generics: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

func findDeadCodeHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	dir := request.GetString("dir", "./")

	deadCode, err := findDeadCode(dir)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to find dead code: %v", err)), nil
	}

	jsonData, err := json.Marshal(deadCode)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal dead code: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

func findDuplicatesHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	dir := request.GetString("dir", "./")
	threshold := request.GetFloat("threshold", 0.8)

	duplicates, err := findDuplicates(dir, threshold)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to find duplicates: %v", err)), nil
	}

	jsonData, err := json.Marshal(duplicates)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal duplicates: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

func findInefficienciesHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	dir := request.GetString("dir", "./")

	inefficiencies, err := findInefficiencies(dir)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to find inefficiencies: %v", err)), nil
	}

	jsonData, err := json.Marshal(inefficiencies)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal inefficiencies: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

func extractApiHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	dir := request.GetString("dir", "./")

	api, err := extractApi(dir)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to extract API: %v", err)), nil
	}

	jsonData, err := json.Marshal(api)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal API: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

func generateDocsHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	dir := request.GetString("dir", "./")
	format := request.GetString("format", "markdown")

	docs, err := generateDocs(dir, format)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to generate docs: %v", err)), nil
	}

	if format == "markdown" {
		return mcp.NewToolResultText(docs.(string)), nil
	}

	jsonData, err := json.Marshal(docs)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal docs: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

func findDeprecatedHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	dir := request.GetString("dir", "./")

	deprecated, err := findDeprecated(dir)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to find deprecated: %v", err)), nil
	}

	jsonData, err := json.Marshal(deprecated)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal deprecated: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

func analyzeCouplingHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	dir := request.GetString("dir", "./")

	coupling, err := analyzeCoupling(dir)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to analyze coupling: %v", err)), nil
	}

	jsonData, err := json.Marshal(coupling)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal coupling: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

func findPatternsHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	dir := request.GetString("dir", "./")

	patterns, err := findPatterns(dir)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to find patterns: %v", err)), nil
	}

	jsonData, err := json.Marshal(patterns)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal patterns: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

func analyzeArchitectureHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	dir := request.GetString("dir", "./")

	architecture, err := analyzeArchitecture(dir)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to analyze architecture: %v", err)), nil
	}

	jsonData, err := json.Marshal(architecture)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal architecture: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

func analyzeGoIdiomsHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	dir := request.GetString("dir", "./")

	idioms, err := analyzeGoIdioms(dir)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to analyze Go idioms: %v", err)), nil
	}

	jsonData, err := json.Marshal(idioms)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal idioms: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

func findContextUsageHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	dir := request.GetString("dir", "./")

	contextUsage, err := findContextUsage(dir)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to find context usage: %v", err)), nil
	}

	jsonData, err := json.Marshal(contextUsage)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal context usage: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

func analyzeEmbeddingHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	dir := request.GetString("dir", "./")

	embedding, err := analyzeEmbedding(dir)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to analyze embedding: %v", err)), nil
	}

	jsonData, err := json.Marshal(embedding)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal embedding: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

func analyzeTestQualityHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	dir := request.GetString("dir", "./")

	testQuality, err := analyzeTestQuality(dir)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to analyze test quality: %v", err)), nil
	}

	jsonData, err := json.Marshal(testQuality)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal test quality: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

func findMissingTestsHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	dir := request.GetString("dir", "./")

	missingTests, err := findMissingTests(dir)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to find missing tests: %v", err)), nil
	}

	jsonData, err := json.Marshal(missingTests)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal missing tests: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

func readRangeHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	file, err := request.RequireString("file")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	startLine := int(request.GetFloat("start_line", -1))
	endLine := int(request.GetFloat("end_line", -1))
	startCol := int(request.GetFloat("start_col", -1))
	endCol := int(request.GetFloat("end_col", -1))
	startByte := int(request.GetFloat("start_byte", -1))
	endByte := int(request.GetFloat("end_byte", -1))

	result, err := readRange(file, startLine, endLine, startCol, endCol, startByte, endByte)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to read range: %v", err)), nil
	}

	jsonData, err := json.Marshal(result)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal result: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

func writeRangeHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	file, err := request.RequireString("file")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	content, err := request.RequireString("content")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	startLine := int(request.GetFloat("start_line", -1))
	endLine := int(request.GetFloat("end_line", -1))
	startCol := int(request.GetFloat("start_col", -1))
	endCol := int(request.GetFloat("end_col", -1))
	startByte := int(request.GetFloat("start_byte", -1))
	endByte := int(request.GetFloat("end_byte", -1))
	confirmOld := request.GetString("confirm_old", "")

	result, err := writeRange(file, content, startLine, endLine, startCol, endCol, startByte, endByte, confirmOld)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to write range: %v", err)), nil
	}

	jsonData, err := json.Marshal(result)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal result: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

func searchReplaceHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// For now, paths will be a single string
	pathStr, err := request.RequireString("paths")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	
	// Split paths by comma if multiple
	paths := strings.Split(pathStr, ",")
	for i := range paths {
		paths[i] = strings.TrimSpace(paths[i])
	}

	pattern, err := request.RequireString("pattern")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	var replacement *string
	if r := request.GetString("replacement", ""); r != "" {
		replacement = &r
	}

	useRegex := request.GetBool("regex", false)
	caseInsensitive := request.GetBool("case_insensitive", false)
	includeContext := request.GetBool("include_context", false)
	contextPattern := request.GetString("context_pattern", "")
	targetGroup := int(request.GetFloat("target_group", 0))

	// If context pattern is provided, use the context-aware replacement
	if contextPattern != "" && replacement != nil && targetGroup > 0 {
		result, err := searchReplaceWithGroups(paths, contextPattern, *replacement, targetGroup, caseInsensitive)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("search/replace failed: %v", err)), nil
		}
		jsonData, err := json.Marshal(result)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to marshal result: %v", err)), nil
		}
		return mcp.NewToolResultText(string(jsonData)), nil
	}

	result, err := searchReplace(paths, pattern, replacement, useRegex, caseInsensitive, includeContext, contextPattern)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("search/replace failed: %v", err)), nil
	}

	jsonData, err := json.Marshal(result)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal result: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}