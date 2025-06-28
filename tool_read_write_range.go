package main

import (
	"fmt"
	"os"
	"strings"
)

type ReadRangeResult struct {
	Content string `json:"content"`
	Lines   int    `json:"lines"`
	Bytes   int    `json:"bytes"`
}

type WriteRangeResult struct {
	Success      bool   `json:"success"`
	LinesWritten int    `json:"lines_written"`
	BytesWritten int    `json:"bytes_written"`
	Message      string `json:"message,omitempty"`
}

// Helper function to convert line/column positions to byte offsets
func lineColToByteRange(data []byte, startLine, endLine, startCol, endCol int) (startByte, endByte int, err error) {
	if len(data) == 0 {
		return 0, 0, nil
	}

	// Convert to 0-based indexing
	if startLine > 0 {
		startLine--
	}
	if endLine > 0 {
		endLine--
	}
	if startCol > 0 {
		startCol--
	}
	if endCol > 0 {
		endCol--
	}

	currentLine := 0
	currentCol := 0
	startByte = -1
	endByte = -1

	for i := 0; i < len(data); i++ {
		// Check if we're at the start position
		if currentLine == startLine && currentCol == startCol && startByte == -1 {
			startByte = i
		}

		// Check if we're at the end position
		if currentLine == endLine {
			if endCol < 0 {
				// No end column specified, go to end of line
				for j := i; j < len(data) && data[j] != '\n'; j++ {
					i = j
				}
				endByte = i + 1
				if endByte > len(data) {
					endByte = len(data)
				}
				break
			} else if currentCol == endCol {
				endByte = i
				break
			}
		}

		// Move to next character
		if data[i] == '\n' {
			// End of line reached
			if currentLine == endLine && endByte == -1 {
				endByte = i
				break
			}
			currentLine++
			currentCol = 0
		} else {
			currentCol++
		}

		// If we've passed the end line, set end byte
		if currentLine > endLine && endByte == -1 {
			endByte = i
			break
		}
	}

	// Handle end of file cases
	if startByte == -1 {
		return 0, 0, fmt.Errorf("start position (line %d, col %d) not found", startLine+1, startCol+1)
	}
	if endByte == -1 {
		endByte = len(data)
	}

	return startByte, endByte, nil
}

func readRange(file string, startLine, endLine, startCol, endCol, startByte, endByte int) (*ReadRangeResult, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Convert line/column to byte range if needed
	if startByte < 0 || endByte < 0 {
		startByte, endByte, err = lineColToByteRange(data, startLine, endLine, startCol, endCol)
		if err != nil {
			return nil, err
		}
	}

	// Validate byte range
	if startByte < 0 || startByte > len(data) {
		return nil, fmt.Errorf("start byte %d out of range (file size: %d)", startByte, len(data))
	}
	if endByte < startByte {
		return nil, fmt.Errorf("end byte %d is before start byte %d", endByte, startByte)
	}
	if endByte > len(data) {
		endByte = len(data)
	}

	// Extract content
	content := string(data[startByte:endByte])
	
	return &ReadRangeResult{
		Content: content,
		Lines:   strings.Count(content, "\n") + 1,
		Bytes:   len(content),
	}, nil
}

func writeRange(file string, content string, startLine, endLine, startCol, endCol, startByte, endByte int, confirmOld string) (*WriteRangeResult, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Convert line/column to byte range if needed
	if startByte < 0 || endByte < 0 {
		startByte, endByte, err = lineColToByteRange(data, startLine, endLine, startCol, endCol)
		if err != nil {
			return nil, err
		}
	}

	// Validate byte range
	if startByte < 0 || startByte > len(data) {
		return nil, fmt.Errorf("start byte %d out of range (file size: %d)", startByte, len(data))
	}
	if endByte < startByte {
		return nil, fmt.Errorf("end byte %d is before start byte %d", endByte, startByte)
	}
	if endByte > len(data) {
		endByte = len(data)
	}

	// Extract old content
	oldContent := string(data[startByte:endByte])

	// Check confirmation if provided
	if confirmOld != "" && oldContent != confirmOld {
		return &WriteRangeResult{
			Success: false,
			Message: fmt.Sprintf("content mismatch: expected %q but found %q", confirmOld, oldContent),
		}, nil
	}

	// Build new content
	newData := make([]byte, 0, len(data)-len(oldContent)+len(content))
	newData = append(newData, data[:startByte]...)
	newData = append(newData, []byte(content)...)
	newData = append(newData, data[endByte:]...)

	// Write the file
	err = os.WriteFile(file, newData, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	return &WriteRangeResult{
		Success:      true,
		LinesWritten: strings.Count(content, "\n") + 1,
		BytesWritten: len(content),
		Message:      "Successfully written",
	}, nil
}