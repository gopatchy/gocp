package main

import (
	"fmt"
	"strings"
)

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