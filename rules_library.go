package yabre

import (
	"fmt"
	"io/fs"
	"os"
	"strings"

	"gopkg.in/yaml.v2"
)

type RulesLibrary struct {
	// maps rule name to its file path
	rulePaths map[string]string
	// maps rule name to its dependencies
	dependencies map[string][]string
	// File system for os or embedded file systems
	fileSystem fs.FS
	// base path for all rules/libraries
	basePath string
}

type RulesLibrarySettings struct {
	// BasePath is only required if no FileSystem is provided.
	// If FileSystem is nil, BasePath will be used to create an OS-based file system using the specified path.
	BasePath string
	// FileSystem specifies the file system to be used, either for the OS file system or an embedded file system.
	FileSystem fs.FS
}

func NewRulesLibrary(s RulesLibrarySettings) (*RulesLibrary, error) {
	rl := &RulesLibrary{
		rulePaths:    make(map[string]string),
		dependencies: make(map[string][]string),
		fileSystem:   s.FileSystem,
		basePath:     s.BasePath,
	}

	if rl.fileSystem == nil {
		rl.fileSystem = os.DirFS(rl.basePath)
	}

	// Scan all yaml files and map dependencies
	if err := rl.scanFiles(); err != nil {
		return nil, fmt.Errorf("failed to scan files: %w", err)
	}

	return rl, nil
}

// GetRuleNamesAndPaths retrieves rule names and their associated paths for visualization purposes.
func (rl *RulesLibrary) GetRuleNamesAndPaths() map[string]string {
	return rl.rulePaths
}

func (rl *RulesLibrary) LoadRules(name string) (*Rules, error) {
	// Get ordered list of dependencies
	deps, err := rl.resolveDependencies(name)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve dependencies: %w", err)
	}

	// Start with empty Rules to merge into
	merged := &Rules{
		Conditions: make(map[string]Condition),
	}

	// Load and merge all dependencies first
	for _, depName := range deps {
		dep, err := rl.loadFile(rl.rulePaths[depName])
		if err != nil {
			return nil, fmt.Errorf("failed to load dependency %s: %w", depName, err)
		}

		if err := rl.mergeRules(merged, dep); err != nil {
			return nil, fmt.Errorf("failed to merge dependency %s: %w", depName, err)
		}
	}

	// Finally load and merge the requested rule set
	main, err := rl.loadFile(rl.rulePaths[name])
	if err != nil {
		return nil, fmt.Errorf("failed to load rule set %s: %w", name, err)
	}

	if err := rl.mergeRules(main, merged); err != nil {
		return nil, fmt.Errorf("failed to merge rule set %s: %w", name, err)
	}

	return main, nil
}

func (rl *RulesLibrary) resolveDependencies(name string) ([]string, error) {
	visited := make(map[string]bool)
	ordered := make([]string, 0)

	var visit func(string) error
	visit = func(n string) error {
		if visited[n] {
			return nil
		}

		if _, exists := rl.rulePaths[n]; !exists {
			return fmt.Errorf("rule set %s not found", n)
		}

		visited[n] = true

		// Visit all dependencies first
		for _, dep := range rl.dependencies[n] {
			if err := visit(dep); err != nil {
				return err
			}
		}

		ordered = append(ordered, n)
		return nil
	}

	if err := visit(name); err != nil {
		return nil, err
	}

	// Remove the last element as it's the main rule set
	return ordered[:len(ordered)-1], nil
}

func (rl *RulesLibrary) mergeRules(target *Rules, source *Rules) error {
	// Merge scripts
	if source.Scripts != "" {
		if target.Scripts == "" {
			target.Scripts = source.Scripts
		} else {
			target.Scripts += "\n" + source.Scripts
		}
	}

	// Merge conditions
	for name, cond := range source.Conditions {
		if _, exists := target.Conditions[name]; exists {
			return fmt.Errorf("duplicate condition %s", name)
		}
		target.Conditions[name] = cond
	}

	return nil
}

func (rl *RulesLibrary) loadFile(path string) (*Rules, error) {
	data, err := fs.ReadFile(rl.fileSystem, path)
	if err != nil {
		return nil, err
	}

	var rules Rules
	if err := yaml.Unmarshal(data, &rules); err != nil {
		return nil, err
	}

	return &rules, nil
}

func (rl *RulesLibrary) scanFiles() error {
	return fs.WalkDir(rl.fileSystem, ".", func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && (strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml")) {
			// Read file to get name and dependencies
			data, err := fs.ReadFile(rl.fileSystem, path)
			if err != nil {
				return fmt.Errorf("failed to read file %s: %w", path, err)
			}

			var rules Rules
			if err := yaml.Unmarshal(data, &rules); err != nil {
				return fmt.Errorf("failed to parse yaml %s: %w", path, err)
			}

			if rules.Name == "" {
				return fmt.Errorf("file %s has no name", path)
			}

			if _, exists := rl.rulePaths[rules.Name]; exists {
				return fmt.Errorf("duplicate rule set name %s", rules.Name)
			}

			rl.rulePaths[rules.Name] = path
			rl.dependencies[rules.Name] = rules.Require
		}
		return nil
	})
}
