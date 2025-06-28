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
	// Create MCP server
	s := server.NewMCPServer(
		"go-executor",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	// Define the build_and_run_go tool
	buildAndRunTool := mcp.NewTool(
		"build_and_run_go",
		mcp.WithDescription("Build and execute Go code"),
		mcp.WithString("code", mcp.Required(), mcp.Description("The Go source code to build and run")),
		mcp.WithNumber("timeout", mcp.Description("Timeout in seconds (default: 30)")),
	)

	// Add tool handler
	s.AddTool(buildAndRunTool, buildAndRunHandler)

	// Start the server
	if err := s.Serve(); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}

func buildAndRunHandler(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	// Extract code parameter
	code, ok := args["code"].(string)
	if !ok {
		return nil, fmt.Errorf("code parameter is required and must be a string")
	}

	// Extract timeout parameter (optional)
	timeout := 30.0
	if t, ok := args["timeout"].(float64); ok {
		timeout = t
	}

	// Build and run the code
	stdout, stderr, exitCode, err := buildAndRunGo(code, time.Duration(timeout)*time.Second)
	
	// Create structured result
	result := RunResult{
		Stdout:   stdout,
		Stderr:   stderr,
		ExitCode: exitCode,
	}
	
	if err != nil {
		result.Error = err.Error()
	}

	// Convert to JSON
	jsonData, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return mcp.NewCallToolResult(
		mcp.NewTextContent(string(jsonData)),
	), nil
}

func buildAndRunGo(code string, timeout time.Duration) (stdout, stderr string, exitCode int, err error) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "gocp-*")
	if err != nil {
		return "", "", -1, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Write code to temporary file
	tmpFile := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(tmpFile, []byte(code), 0644); err != nil {
		return "", "", -1, fmt.Errorf("failed to write code: %w", err)
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Initialize go.mod in temp directory
	modCmd := exec.CommandContext(ctx, "go", "mod", "init", "temp")
	modCmd.Dir = tmpDir
	if err := modCmd.Run(); err != nil {
		return "", "", -1, fmt.Errorf("failed to initialize go.mod: %w", err)
	}

	// Run the code directly with go run
	runCmd := exec.CommandContext(ctx, "go", "run", tmpFile)
	runCmd.Dir = tmpDir
	
	// Capture stdout and stderr separately
	var stdoutBuf, stderrBuf bytes.Buffer
	runCmd.Stdout = &stdoutBuf
	runCmd.Stderr = &stderrBuf
	
	// Run the command
	err = runCmd.Run()
	
	// Get exit code
	exitCode = 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
			err = nil // Clear error since we got the exit code
		} else if ctx.Err() == context.DeadlineExceeded {
			return stdoutBuf.String(), stderrBuf.String(), -1, fmt.Errorf("execution timeout exceeded")
		} else {
			// Some other error occurred
			return stdoutBuf.String(), stderrBuf.String(), -1, err
		}
	}

	return stdoutBuf.String(), stderrBuf.String(), exitCode, nil
}