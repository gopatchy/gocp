package main

// Coupling analysis types
type CouplingInfo struct {
	Package        string            `json:"package"`
	Afferent       int               `json:"afferent"`
	Efferent       int               `json:"efferent"`
	Instability    float64           `json:"instability"`
	Dependencies   []string          `json:"dependencies"`
	Dependents     []string          `json:"dependents"`
	Suggestions    []string          `json:"suggestions,omitempty"`
}

func analyzeCoupling(dir string) ([]CouplingInfo, error) {
	var coupling []CouplingInfo

	// This is a simplified implementation
	packages, err := listPackages(dir, false)
	if err != nil {
		return nil, err
	}

	for _, pkg := range packages {
		info := CouplingInfo{
			Package:      pkg.Name,
			Dependencies: pkg.Imports,
			Efferent:     len(pkg.Imports),
		}

		// Calculate instability (Ce / (Ca + Ce))
		if info.Afferent+info.Efferent > 0 {
			info.Instability = float64(info.Efferent) / float64(info.Afferent+info.Efferent)
		}

		coupling = append(coupling, info)
	}

	return coupling, nil
}