package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type SearchReplaceResult struct {
	Files       []FileSearchReplaceResult `json:"files"`
	TotalMatches int                      `json:"total_matches"`
	TotalReplaced int                     `json:"total_replaced,omitempty"`
}

type FileSearchReplaceResult struct {
	Path         string              `json:"path"`
	Matches      []SearchMatch       `json:"matches,omitempty"`
	Replaced     int                 `json:"replaced,omitempty"`
	Error        string              `json:"error,omitempty"`
}

type SearchMatch struct {
	Line       int    `json:"line"`
	Column     int    `json:"column"`
	Text       string `json:"text"`
	Context    string `json:"context,omitempty"`
	StartLine  int    `json:"start_line"`
	StartCol   int    `json:"start_col"`
	EndLine    int    `json:"end_line"`
	EndCol     int    `json:"end_col"`
	StartByte  int    `json:"start_byte"`
	EndByte    int    `json:"end_byte"`
}

func searchReplace(paths []string, pattern string, replacement *string, useRegex, caseInsensitive bool, includeContext bool, beforePattern, afterPattern string, replaceAll bool) (*SearchReplaceResult, error) {
	result := &SearchReplaceResult{
		Files: []FileSearchReplaceResult{},
	}

	// Prepare search/replace function
	var searchFunc func(string) [][]int
	var replaceFunc func(string) string
	
	// Handle context-aware replacement with before/after patterns
	if (beforePattern != "" || afterPattern != "") && replacement != nil {
		// Build a pattern that captures before, target, and after parts
		contextPattern := ""
		if beforePattern != "" {
			contextPattern += "(" + regexp.QuoteMeta(beforePattern) + ")"
		}
		if useRegex {
			contextPattern += "(" + pattern + ")"
		} else {
			contextPattern += "(" + regexp.QuoteMeta(pattern) + ")"
		}
		if afterPattern != "" {
			contextPattern += "(" + regexp.QuoteMeta(afterPattern) + ")"
		}
		
		flags := ""
		if caseInsensitive {
			flags = "(?i)"
		}
		
		contextRe, err := regexp.Compile(flags + contextPattern)
		if err != nil {
			return nil, fmt.Errorf("invalid context pattern: %w", err)
		}
		
		searchFunc = func(text string) [][]int {
			return contextRe.FindAllStringIndex(text, -1)
		}
		
		replaceFunc = func(text string) string {
			if !replaceAll {
				// Replace only first occurrence
				loc := contextRe.FindStringSubmatchIndex(text)
				if loc == nil {
					return text
				}
				
				submatches := contextRe.FindStringSubmatch(text)
				// Rebuild the match with the target replaced
				result := ""
				if beforePattern != "" && len(submatches) > 1 {
					result += submatches[1] // before part
				}
				result += *replacement // replacement for target
				if afterPattern != "" {
					// The after part is at index 3 if before exists, otherwise at index 2
					afterIndex := 2
					if beforePattern != "" {
						afterIndex = 3
					}
					if len(submatches) > afterIndex {
						result += submatches[afterIndex]
					}
				}
				
				return text[:loc[0]] + result + text[loc[1]:]
			}
			
			return contextRe.ReplaceAllStringFunc(text, func(match string) string {
				submatches := contextRe.FindStringSubmatch(match)
				
				// Rebuild the match with the target replaced
				result := ""
				if beforePattern != "" && len(submatches) > 1 {
					result += submatches[1] // before part
				}
				result += *replacement // replacement for target
				if afterPattern != "" {
					// The after part is at index 3 if before exists, otherwise at index 2
					afterIndex := 2
					if beforePattern != "" {
						afterIndex = 3
					}
					if len(submatches) > afterIndex {
						result += submatches[afterIndex]
					}
				}
				return result
			})
		}
	} else if useRegex {
		flags := ""
		if caseInsensitive {
			flags = "(?i)"
		}
		re, err := regexp.Compile(flags + pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid regex pattern: %w", err)
		}
		searchFunc = func(text string) [][]int {
			return re.FindAllStringIndex(text, -1)
		}
		if replacement != nil {
			replaceFunc = func(text string) string {
				if !replaceAll {
					// Replace only first occurrence
					loc := re.FindStringIndex(text)
					if loc == nil {
						return text
					}
					return text[:loc[0]] + string(re.ExpandString([]byte{}, *replacement, text, re.FindStringSubmatchIndex(text))) + text[loc[1]:]
				}
				return re.ReplaceAllString(text, *replacement)
			}
		}
	} else {
		searchPattern := pattern
		if caseInsensitive {
			searchPattern = strings.ToLower(pattern)
		}
		searchFunc = func(text string) [][]int {
			searchText := text
			if caseInsensitive {
				searchText = strings.ToLower(text)
			}
			var matches [][]int
			start := 0
			for {
				idx := strings.Index(searchText[start:], searchPattern)
				if idx < 0 {
					break
				}
				realIdx := start + idx
				matches = append(matches, []int{realIdx, realIdx + len(pattern)})
				start = realIdx + len(pattern)
			}
			return matches
		}
		if replacement != nil {
			replaceFunc = func(text string) string {
				if !replaceAll {
					// Replace only first occurrence
					if caseInsensitive {
						return caseInsensitiveReplaceFirst(text, pattern, *replacement)
					}
					return strings.Replace(text, pattern, *replacement, 1)
				}
				if caseInsensitive {
					// Case-insensitive string replacement
					return caseInsensitiveReplace(text, pattern, *replacement)
				}
				return strings.ReplaceAll(text, pattern, *replacement)
			}
		}
	}

	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			result.Files = append(result.Files, FileSearchReplaceResult{
				Path:  path,
				Error: fmt.Sprintf("stat error: %v", err),
			})
			continue
		}

		if info.IsDir() {
			// Process directory tree
			err := filepath.WalkDir(path, func(filePath string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return nil
				}
				
				// Skip non-text files
				if !isTextFile(filePath) {
					return nil
				}

				fileResult := processFile(filePath, searchFunc, replaceFunc, includeContext, replaceAll)
				if len(fileResult.Matches) > 0 || fileResult.Replaced > 0 || fileResult.Error != "" {
					result.Files = append(result.Files, fileResult)
					result.TotalMatches += len(fileResult.Matches)
					result.TotalReplaced += fileResult.Replaced
				}
				return nil
			})
			if err != nil {
				return nil, err
			}
		} else {
			// Process single file
			fileResult := processFile(path, searchFunc, replaceFunc, includeContext, replaceAll)
			if len(fileResult.Matches) > 0 || fileResult.Replaced > 0 || fileResult.Error != "" {
				result.Files = append(result.Files, fileResult)
				result.TotalMatches += len(fileResult.Matches)
				result.TotalReplaced += fileResult.Replaced
			}
		}
	}

	return result, nil
}


func processFile(path string, searchFunc func(string) [][]int, replaceFunc func(string) string, includeContext bool, replaceAll bool) FileSearchReplaceResult {
	result := FileSearchReplaceResult{
		Path: path,
	}

	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		result.Error = fmt.Sprintf("read error: %v", err)
		return result
	}

	content := string(data)
	
	// If replacement is requested, do it
	if replaceFunc != nil {
		matches := searchFunc(content)
		if replaceAll {
			result.Replaced = len(matches)
		} else {
			// Only replacing first occurrence
			if len(matches) > 0 {
				result.Replaced = 1
			}
		}
		
		if result.Replaced > 0 {
			newContent := replaceFunc(content)
			err = os.WriteFile(path, []byte(newContent), 0644)
			if err != nil {
				result.Error = fmt.Sprintf("write error: %v", err)
				result.Replaced = 0
			}
		}
		return result
	}

	// Otherwise, just search
	lines := strings.Split(content, "\n")
	lineStarts := make([]int, len(lines))
	pos := 0
	for i, line := range lines {
		lineStarts[i] = pos
		pos += len(line) + 1 // +1 for newline
	}

	matches := searchFunc(content)
	for _, match := range matches {
		// Find line number
		lineNum := 0
		for i, start := range lineStarts {
			if match[0] >= start && (i == len(lineStarts)-1 || match[0] < lineStarts[i+1]) {
				lineNum = i + 1
				break
			}
		}
		
		// Calculate column
		lineStart := 0
		if lineNum > 0 {
			lineStart = lineStarts[lineNum-1]
		}
		column := match[0] - lineStart + 1
		
		// Calculate end position
		endLineNum := lineNum
		endColumn := column + len(content[match[0]:match[1]])
		
		// Check if match spans multiple lines
		matchText := content[match[0]:match[1]]
		newlineCount := strings.Count(matchText, "\n")
		if newlineCount > 0 {
			// Find the end line
			for i := lineNum; i < len(lineStarts); i++ {
				if match[1] <= lineStarts[i] {
					endLineNum = i
					break
				} else if i == len(lineStarts)-1 {
					endLineNum = len(lines)
				}
			}
			
			// Calculate end column on the last line
			endLineStart := 0
			if endLineNum > 0 && endLineNum <= len(lineStarts) {
				endLineStart = lineStarts[endLineNum-1]
			}
			endColumn = match[1] - endLineStart + 1
		}
		
		searchMatch := SearchMatch{
			Line:      lineNum,
			Column:    column,
			Text:      matchText,
			StartLine: lineNum,
			StartCol:  column,
			EndLine:   endLineNum,
			EndCol:    endColumn,
			StartByte: match[0],
			EndByte:   match[1],
		}
		
		if includeContext && lineNum > 0 && lineNum <= len(lines) {
			searchMatch.Context = strings.TrimSpace(lines[lineNum-1])
		}
		
		result.Matches = append(result.Matches, searchMatch)
	}

	return result
}

func caseInsensitiveReplace(text, old, new string) string {
	// Simple case-insensitive replacement
	var result strings.Builder
	lowerText := strings.ToLower(text)
	lowerOld := strings.ToLower(old)
	
	start := 0
	for {
		idx := strings.Index(lowerText[start:], lowerOld)
		if idx < 0 {
			result.WriteString(text[start:])
			break
		}
		realIdx := start + idx
		result.WriteString(text[start:realIdx])
		result.WriteString(new)
		start = realIdx + len(old)
	}
	
	return result.String()
}

func caseInsensitiveReplaceFirst(text, old, new string) string {
	// Case-insensitive replacement for first occurrence only
	lowerText := strings.ToLower(text)
	lowerOld := strings.ToLower(old)
	
	idx := strings.Index(lowerText, lowerOld)
	if idx < 0 {
		return text
	}
	
	return text[:idx] + new + text[idx+len(old):]
}

func isTextFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	textExts := map[string]bool{
		".go":   true,
		".txt":  true,
		".md":   true,
		".json": true,
		".yaml": true,
		".yml":  true,
		".toml": true,
		".xml":  true,
		".html": true,
		".css":  true,
		".js":   true,
		".ts":   true,
		".py":   true,
		".rb":   true,
		".java": true,
		".c":    true,
		".cpp":  true,
		".h":    true,
		".hpp":  true,
		".rs":   true,
		".sh":   true,
		".bash": true,
		".zsh":  true,
		".fish": true,
		".sql":  true,
		".proto": true,
		".mod":  true,
		".sum":  true,
	}
	
	// Check extension
	if textExts[ext] {
		return true
	}
	
	// Check for files without extension that might be text
	base := filepath.Base(path)
	if base == "Makefile" || base == "Dockerfile" || base == "README" || 
	   base == "LICENSE" || base == "CHANGELOG" || base == "TODO" ||
	   strings.HasPrefix(base, ".") { // dotfiles are often text
		return true
	}
	
	return false
}