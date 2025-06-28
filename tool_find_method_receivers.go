package main

import (
	"go/ast"
	"go/token"
)

type MethodReceiver struct {
	TypeName       string   `json:"type_name"`
	MethodName     string   `json:"method_name"`
	ReceiverType   string   `json:"receiver_type"` // "pointer" or "value"
	ReceiverName   string   `json:"receiver_name"`
	Position       Position `json:"position"`
}

type ReceiverAnalysis struct {
	Methods []MethodReceiver `json:"methods"`
	Issues  []ReceiverIssue  `json:"issues"`
}

type ReceiverIssue struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Position    Position `json:"position"`
}

func findMethodReceivers(dir string) (*ReceiverAnalysis, error) {
	analysis := &ReceiverAnalysis{
		Methods: []MethodReceiver{},
		Issues:  []ReceiverIssue{},
	}

	typeReceivers := make(map[string]map[string]bool) // type -> receiver type -> exists

	err := walkGoFiles(dir, func(path string, src []byte, file *ast.File, fset *token.FileSet) error {
		ast.Inspect(file, func(n ast.Node) bool {
			if funcDecl, ok := n.(*ast.FuncDecl); ok && funcDecl.Recv != nil {
				if len(funcDecl.Recv.List) > 0 {
					recv := funcDecl.Recv.List[0]
					var typeName string
					var receiverType string
					var receiverName string

					if len(recv.Names) > 0 {
						receiverName = recv.Names[0].Name
					}

					switch t := recv.Type.(type) {
					case *ast.Ident:
						typeName = t.Name
						receiverType = "value"
					case *ast.StarExpr:
						if ident, ok := t.X.(*ast.Ident); ok {
							typeName = ident.Name
							receiverType = "pointer"
						}
					}

					if typeName != "" {
						pos := fset.Position(funcDecl.Pos())
						method := MethodReceiver{
							TypeName:     typeName,
							MethodName:   funcDecl.Name.Name,
							ReceiverType: receiverType,
							ReceiverName: receiverName,
							Position:     newPosition(pos),
						}
						analysis.Methods = append(analysis.Methods, method)

						// Track receiver types per type
						if typeReceivers[typeName] == nil {
							typeReceivers[typeName] = make(map[string]bool)
						}
						typeReceivers[typeName][receiverType] = true
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

	// Analyze for inconsistencies
	for typeName, receivers := range typeReceivers {
		if receivers["pointer"] && receivers["value"] {
			// Find all methods with this issue
			for _, method := range analysis.Methods {
				if method.TypeName == typeName {
					issue := ReceiverIssue{
						Type:        "mixed_receivers",
						Description: "Type " + typeName + " has methods with both pointer and value receivers",
						Position:    method.Position,
					}
					analysis.Issues = append(analysis.Issues, issue)
					break // Only report once per type
				}
			}
		}
	}

	// Check for methods that should use pointer receivers
	for _, method := range analysis.Methods {
		if method.ReceiverType == "value" && shouldUsePointerReceiver(method.MethodName) {
			issue := ReceiverIssue{
				Type:        "should_use_pointer",
				Description: "Method " + method.MethodName + " on " + method.TypeName + " should probably use a pointer receiver",
				Position:    method.Position,
			}
			analysis.Issues = append(analysis.Issues, issue)
		}
	}

	return analysis, nil
}

func shouldUsePointerReceiver(methodName string) bool {
	// Methods that typically modify state should use pointer receivers
	prefixes := []string{"Set", "Add", "Remove", "Delete", "Update", "Append", "Clear", "Reset"}
	for _, prefix := range prefixes {
		if len(methodName) > len(prefix) && methodName[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}