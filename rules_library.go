package yabre

import (
	"embed"
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
	// cached parsed rule sets (loaded once at init, no further FS access)
	rules map[string]*Rules
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

func NewRulesLibrary(s RulesLibrarySettings) (*RulesLibrary, ValidationResult, error) {
	if s.BasePath == "" {
		s.BasePath = "."
	}

	rl := &RulesLibrary{
		rulePaths:    make(map[string]string),
		dependencies: make(map[string][]string),
		rules:        make(map[string]*Rules),
		fileSystem:   s.FileSystem,
		basePath:     s.BasePath,
	}

	if rl.fileSystem == nil {
		rl.fileSystem = os.DirFS(rl.basePath)
		// BasePath becomes relative to fileSystem root
		rl.basePath = "."
	} else if _, ok := rl.fileSystem.(embed.FS); ok {
		// For embedded fs, directory walkthru fs.WalkDir doesn't like paths starting with ./ so we trim it
		rl.basePath = strings.TrimPrefix(rl.basePath, "./")
	}

	// Scan all yaml files and map dependencies
	if err := rl.scanFiles(); err != nil {
		return nil, ValidationResult{}, fmt.Errorf("failed to scan files: %w", err)
	}

	// validateAll enables validation of all rule sets during library initialization.
	// Circular dependencies are fatal (library init fails). Other issues (e.g., unreachable
	// conditions, dangling references) are collected in the library's Warnings field.
	result := rl.validateAll()

	return rl, result, nil
}

// GetRuleNamesAndPaths retrieves rule names and their associated file paths.
// Useful for clients that need to reference the underlying YAML files,
// e.g. for visualization, editor integration, or tooling that operates on the source files.
func (rl *RulesLibrary) GetRuleNamesAndPaths() map[string]string {
	return rl.rulePaths
}

func (rl *RulesLibrary) LoadRules(name string) (*Rules, error) {
	cached, exists := rl.rules[name]
	if !exists {
		return nil, fmt.Errorf("rule set %s not found", name)
	}

	// Get ordered list of dependencies
	deps, err := rl.resolveDependencies(name)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve dependencies: %w", err)
	}

	// Start with a copy of the main rule set (avoid mutating the cache)
	main := cached.copy()

	// Merge all dependencies into the main rule set
	for _, depName := range deps {
		dep, exists := rl.rules[depName]
		if !exists {
			return nil, fmt.Errorf("dependency %s not found", depName)
		}

		if err := mergeRules(main, dep); err != nil {
			return nil, fmt.Errorf("failed to merge dependency %s: %w", depName, err)
		}
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

func mergeRules(target *Rules, source *Rules) error {
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

// validateAll runs validation on all rule sets (with merged dependencies).
// It collects all issues across every rule set and returns them grouped into
// errors and warnings — it never stops early on the first problem.
func (rl *RulesLibrary) validateAll() ValidationResult {
	var result ValidationResult

	// Check library-level require cycles
	for _, issue := range rl.ValidateLibraryDependencies() {
		if issue.Severity == SeverityError {
			result.Errors = append(result.Errors, issue)
		} else {
			result.Warnings = append(result.Warnings, issue)
		}
	}

	// Validate each rule set with its merged dependencies
	for name := range rl.rules {
		merged, err := rl.LoadRules(name)
		if err != nil {
			result.Errors = append(result.Errors, ValidationIssue{
				Severity: SeverityError,
				Message:  fmt.Sprintf("rule set %s: %s", name, err.Error()),
			})
			continue
		}

		for _, issue := range ValidateRules(merged) {
			issue.ConditionName = name + "." + issue.ConditionName
			if issue.Severity == SeverityError {
				result.Errors = append(result.Errors, issue)
			} else {
				result.Warnings = append(result.Warnings, issue)
			}
		}
	}

	return result
}

func (rl *RulesLibrary) scanFiles() error {
	return fs.WalkDir(rl.fileSystem, rl.basePath, func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && (strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml")) {
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

			if _, exists := rl.rules[rules.Name]; exists {
				return fmt.Errorf("duplicate rule set name %s", rules.Name)
			}

			rl.rulePaths[rules.Name] = path
			rl.dependencies[rules.Name] = rules.Require
			rl.rules[rules.Name] = &rules
		}
		return nil
	})
}
