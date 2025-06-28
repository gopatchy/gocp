package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type GoRunResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
	Error    string `json:"error,omitempty"`
	Command  string `json:"command"`
	WorkDir  string `json:"work_dir"`
}

func goRun(path string, flags []string, timeout time.Duration) (*GoRunResult, error) {
	// Resolve absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return &GoRunResult{
			Error:   err.Error(),
			Command: "go run " + path,
		}, nil
	}

	// Determine working directory and target file/package
	var workDir string
	var target string

	// Check if path is a file or directory
	info, err := os.Stat(absPath)
	if err != nil {
		return &GoRunResult{
			Error:   err.Error(),
			Command: "go run " + path,
		}, nil
	}

	if info.IsDir() {
		// Running a package
		workDir = absPath
		target = "."
	} else {
		// Running a specific file
		workDir = filepath.Dir(absPath)
		target = filepath.Base(absPath)
	}

	// Build command arguments
	args := []string{"run"}
	args = append(args, flags...)
	args = append(args, target)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = workDir

	stdout, stderr, exitCode, cmdErr := runCommand(cmd)

	result := &GoRunResult{
		Stdout:   stdout,
		Stderr:   stderr,
		ExitCode: exitCode,
		Command:  "go " + strings.Join(args, " "),
		WorkDir:  workDir,
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