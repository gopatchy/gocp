package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type GoTestResult struct {
	Stdout    string `json:"stdout"`
	Stderr    string `json:"stderr"`
	ExitCode  int    `json:"exit_code"`
	Error     string `json:"error,omitempty"`
	Command   string `json:"command"`
	WorkDir   string `json:"work_dir"`
	Passed    bool   `json:"passed"`
	TestCount int    `json:"test_count,omitempty"`
}

func goTest(path string, flags []string, timeout time.Duration) (*GoTestResult, error) {
	// Resolve absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return &GoTestResult{
			Error:   err.Error(),
			Command: "go test " + path,
		}, nil
	}

	// Determine working directory and target
	var workDir string
	var target string

	// Check if path is a file or directory
	info, err := os.Stat(absPath)
	if err != nil {
		return &GoTestResult{
			Error:   err.Error(),
			Command: "go test " + path,
		}, nil
	}

	if info.IsDir() {
		// Testing a package
		workDir = absPath
		target = "."
	} else {
		// Testing a specific file (though go test typically works with packages)
		workDir = filepath.Dir(absPath)
		target = "."
	}

	// Build command arguments
	args := []string{"test"}
	args = append(args, flags...)
	args = append(args, target)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = workDir

	stdout, stderr, exitCode, cmdErr := runCommand(cmd)

	result := &GoTestResult{
		Stdout:   stdout,
		Stderr:   stderr,
		ExitCode: exitCode,
		Command:  "go " + strings.Join(args, " "),
		WorkDir:  workDir,
		Passed:   exitCode == 0,
	}

	// Try to extract test count from output
	if strings.Contains(stdout, "PASS") || strings.Contains(stdout, "FAIL") {
		result.TestCount = countTests(stdout)
	}

	if cmdErr != nil {
		if ctx.Err() == context.DeadlineExceeded {
			result.Error = "execution timeout exceeded"
		} else {
			result.Error = cmdErr.Error()
		}
	}

	return result, nil
}

// Helper function to count tests from go test output
func countTests(output string) int {
	count := 0
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "--- PASS:") ||
			strings.HasPrefix(strings.TrimSpace(line), "--- FAIL:") ||
			strings.HasPrefix(strings.TrimSpace(line), "--- SKIP:") {
			count++
		}
	}
	return count
}