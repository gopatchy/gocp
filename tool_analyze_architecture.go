package main

// Architecture analysis types
type ArchitectureInfo struct {
	Layers       []LayerInfo       `json:"layers"`
	Violations   []LayerViolation  `json:"violations,omitempty"`
	Suggestions  []string          `json:"suggestions,omitempty"`
}

type LayerInfo struct {
	Name         string   `json:"name"`
	Packages     []string `json:"packages"`
	Dependencies []string `json:"dependencies"`
}

type LayerViolation struct {
	From        string   `json:"from"`
	To          string   `json:"to"`
	Violation   string   `json:"violation"`
	Position    Position `json:"position"`
}

func analyzeArchitecture(dir string) (*ArchitectureInfo, error) {
	// Simplified architecture analysis
	packages, err := listPackages(dir, false)
	if err != nil {
		return nil, err
	}

	arch := &ArchitectureInfo{
		Layers: []LayerInfo{},
	}

	// Detect common Go project structure
	for _, pkg := range packages {
		layer := LayerInfo{
			Name:         pkg.Name,
			Packages:     []string{pkg.ImportPath},
			Dependencies: pkg.Imports,
		}
		arch.Layers = append(arch.Layers, layer)
	}

	return arch, nil
}