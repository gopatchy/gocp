package main

import (
	"bytes"
	"os/exec"
)

// runCommand executes a command and returns stdout, stderr, exit code, and error
func runCommand(cmd *exec.Cmd) (stdout, stderr string, exitCode int, err error) {
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err = cmd.Run()
	
	exitCode = 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
			err = nil // Don't treat non-zero exit as error for go commands
		}
	}

	return stdoutBuf.String(), stderrBuf.String(), exitCode, err
}