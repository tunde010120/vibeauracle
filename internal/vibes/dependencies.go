package vibes

import (
	"fmt"
	"sort"
	"strings"
)

// Dependency represents a Vibe dependency.
type Dependency struct {
	Name     string `yaml:"name"`
	Version  string `yaml:"version,omitempty"` // Semver constraint (e.g., ">=1.0.0")
	Optional bool   `yaml:"optional,omitempty"`
}

// Conflict represents a conflicting Vibe.
type Conflict struct {
	Name   string `yaml:"name"`
	Reason string `yaml:"reason,omitempty"`
}

// DependencySpec extends the base Spec with dependency info.
type DependencySpec struct {
	Dependencies []Dependency `yaml:"dependencies,omitempty"`
	Conflicts    []Conflict   `yaml:"conflicts,omitempty"`
	Provides     []string     `yaml:"provides,omitempty"` // Virtual capabilities
}

// ResolutionResult holds the result of dependency resolution.
type ResolutionResult struct {
	Resolved  []*Vibe
	Missing   []string
	Conflicts []string
	LoadOrder []string
}

func (rr *ResolutionResult) IsValid() bool {
	return len(rr.Missing) == 0 && len(rr.Conflicts) == 0
}

// DependencyResolver handles Vibe dependency resolution.
type DependencyResolver struct {
	registry *Registry
}

// NewDependencyResolver creates a new resolver.
func NewDependencyResolver(registry *Registry) *DependencyResolver {
	return &DependencyResolver{registry: registry}
}

// Resolve determines the correct load order for a set of Vibes.
func (dr *DependencyResolver) Resolve(names []string) (*ResolutionResult, error) {
	result := &ResolutionResult{
		Resolved:  make([]*Vibe, 0),
		Missing:   make([]string, 0),
		Conflicts: make([]string, 0),
		LoadOrder: make([]string, 0),
	}

	// Build dependency graph
	graph := make(map[string][]string)
	inDegree := make(map[string]int)
	vibeMap := make(map[string]*Vibe)

	for _, name := range names {
		vibe, ok := dr.registry.Get(name)
		if !ok {
			result.Missing = append(result.Missing, name)
			continue
		}
		vibeMap[name] = vibe
		graph[name] = []string{}
		inDegree[name] = 0
	}

	// Add dependency edges
	for name, vibe := range vibeMap {
		deps := extractDependencies(vibe)
		for _, dep := range deps {
			if _, ok := vibeMap[dep.Name]; !ok {
				if !dep.Optional {
					result.Missing = append(result.Missing, fmt.Sprintf("%s (required by %s)", dep.Name, name))
				}
				continue
			}
			graph[dep.Name] = append(graph[dep.Name], name)
			inDegree[name]++
		}

		// Check for conflicts
		conflicts := extractConflicts(vibe)
		for _, conflict := range conflicts {
			if _, ok := vibeMap[conflict.Name]; ok {
				result.Conflicts = append(result.Conflicts,
					fmt.Sprintf("%s conflicts with %s: %s", name, conflict.Name, conflict.Reason))
			}
		}
	}

	// Topological sort (Kahn's algorithm)
	queue := make([]string, 0)
	for name, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, name)
		}
	}

	for len(queue) > 0 {
		// Sort for deterministic order
		sort.Strings(queue)
		current := queue[0]
		queue = queue[1:]

		result.LoadOrder = append(result.LoadOrder, current)
		if vibe, ok := vibeMap[current]; ok {
			result.Resolved = append(result.Resolved, vibe)
		}

		for _, dependent := range graph[current] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	// Check for cycles
	if len(result.LoadOrder) != len(vibeMap) {
		return nil, fmt.Errorf("circular dependency detected")
	}

	return result, nil
}

// CheckConflicts checks if enabling a Vibe would cause conflicts.
func (dr *DependencyResolver) CheckConflicts(vibeName string) []string {
	vibe, ok := dr.registry.Get(vibeName)
	if !ok {
		return nil
	}

	var conflicts []string
	vibeConflicts := extractConflicts(vibe)

	for _, conflict := range vibeConflicts {
		other, ok := dr.registry.Get(conflict.Name)
		if ok && other.Enabled {
			conflicts = append(conflicts, fmt.Sprintf("%s: %s", conflict.Name, conflict.Reason))
		}
	}

	// Check reverse conflicts (other vibes that conflict with this one)
	for _, other := range dr.registry.List() {
		if !other.Enabled || other.Spec.Name == vibeName {
			continue
		}
		for _, conflict := range extractConflicts(other) {
			if conflict.Name == vibeName {
				conflicts = append(conflicts, fmt.Sprintf("%s conflicts with enabling %s: %s",
					other.Spec.Name, vibeName, conflict.Reason))
			}
		}
	}

	return conflicts
}

// GetMissingDependencies returns unmet dependencies for a Vibe.
func (dr *DependencyResolver) GetMissingDependencies(vibeName string) []string {
	vibe, ok := dr.registry.Get(vibeName)
	if !ok {
		return nil
	}

	var missing []string
	deps := extractDependencies(vibe)

	for _, dep := range deps {
		other, ok := dr.registry.Get(dep.Name)
		if !ok {
			if !dep.Optional {
				missing = append(missing, dep.Name)
			}
			continue
		}
		if !other.Enabled && !dep.Optional {
			missing = append(missing, fmt.Sprintf("%s (disabled)", dep.Name))
		}
	}

	return missing
}

// extractDependencies parses dependencies from a Vibe's instructions.
// In a real implementation, this would be in the YAML spec.
func extractDependencies(vibe *Vibe) []Dependency {
	// Check if instructions contain dependency markers
	var deps []Dependency
	lines := strings.Split(vibe.Instructions, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "@depends:") {
			depName := strings.TrimSpace(strings.TrimPrefix(line, "@depends:"))
			deps = append(deps, Dependency{Name: depName})
		}
		if strings.HasPrefix(line, "@optional-depends:") {
			depName := strings.TrimSpace(strings.TrimPrefix(line, "@optional-depends:"))
			deps = append(deps, Dependency{Name: depName, Optional: true})
		}
	}
	return deps
}

// extractConflicts parses conflicts from a Vibe's instructions.
func extractConflicts(vibe *Vibe) []Conflict {
	var conflicts []Conflict
	lines := strings.Split(vibe.Instructions, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "@conflicts:") {
			parts := strings.SplitN(strings.TrimPrefix(line, "@conflicts:"), ":", 2)
			conflict := Conflict{Name: strings.TrimSpace(parts[0])}
			if len(parts) > 1 {
				conflict.Reason = strings.TrimSpace(parts[1])
			}
			conflicts = append(conflicts, conflict)
		}
	}
	return conflicts
}
