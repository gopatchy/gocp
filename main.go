package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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