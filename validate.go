package yabre

import (
	"fmt"
	"strings"
)

// Severity represents the level of a validation issue.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// ValidationIssue represents a single validation problem found in a rule set.
type ValidationIssue struct {
	Severity      Severity
	ConditionName string // which condition the issue relates to (empty for rule-set-level issues)
	Message       string
}

type ValidationIssues []ValidationIssue

func (v ValidationIssues) String() string {
	msgs := make([]string, len(v))
	for i, issue := range v {
		msgs[i] = issue.String()
	}
	return strings.Join(msgs, "\n")
}

// ValidationResult groups all issues found during a validation pass.
type ValidationResult struct {
	Errors   ValidationIssues
	Warnings ValidationIssues
}

func (e ValidationResult) String() string {
	msgs := []string{}
	warns := e.Warnings.String()
	if warns != "" {
		warns = "Warnings:\n" + strings.ReplaceAll(warns, "\n", "\n    ")
		msgs = append(msgs, warns)
	}
	errs := e.Errors.String()
	if errs != "" {
		errs = "Errors:\n" + strings.ReplaceAll(errs, "\n", "\n    ")
		msgs = append(msgs, errs)
	}
	return strings.Join(msgs, "\n")
}

func (v ValidationIssue) String() string {
	if v.ConditionName != "" {
		return fmt.Sprintf("[%s] %s: %s", v.Severity, v.ConditionName, v.Message)
	}
	return fmt.Sprintf("[%s] %s", v.Severity, v.Message)
}

// ValidateRules checks a Rules struct for circular dependencies in the condition graph.
// It traverses all possible paths from every condition and reports any cycles found.
// Returns a list of validation issues (errors for cycles, warnings for unreachable conditions).
func ValidateRules(rules *Rules) []ValidationIssue {
	var issues []ValidationIssue

	// Check for circular dependencies in condition chains
	cycles := detectConditionCycles(rules)
	issues = append(issues, cycles...)

	// Check for dangling next references
	issues = append(issues, detectDanglingReferences(rules)...)

	// Check for unreachable conditions
	issues = append(issues, detectUnreachableConditions(rules)...)

	return issues
}

// detectConditionCycles traverses all paths through the condition graph using DFS.
// For each condition, it follows both true.next and false.next edges.
// If any path visits the same condition twice, that's a cycle.
func detectConditionCycles(rules *Rules) []ValidationIssue {
	var issues []ValidationIssue

	// For each condition, try to find cycles starting from it
	for name := range rules.Conditions {
		path := make(map[string]bool)
		if cycle := walkCondition(rules, name, path); cycle != nil {
			issues = append(issues, *cycle)
		}
	}

	// Deduplicate: a cycle A->B->A will be reported from both A and B.
	// Keep only unique cycle messages.
	seen := make(map[string]bool)
	var deduplicated []ValidationIssue
	for _, issue := range issues {
		if !seen[issue.Message] {
			seen[issue.Message] = true
			deduplicated = append(deduplicated, issue)
		}
	}

	return deduplicated
}

// walkCondition performs DFS from a given condition, tracking the current path.
// Returns a ValidationIssue if a cycle is detected, nil otherwise.
func walkCondition(rules *Rules, conditionName string, path map[string]bool) *ValidationIssue {
	if path[conditionName] {
		// We've visited this condition already on the current path — cycle detected
		return &ValidationIssue{
			Severity:      SeverityError,
			ConditionName: conditionName,
			Message:       fmt.Sprintf("circular dependency detected: condition '%s' is reachable from itself", conditionName),
		}
	}

	condition, ok := rules.Conditions[conditionName]
	if !ok {
		// Dangling reference — handled separately
		return nil
	}

	path[conditionName] = true

	// Follow true branch
	if condition.True != nil && condition.True.Next != "" {
		if issue := walkCondition(rules, condition.True.Next, path); issue != nil {
			return issue
		}
	}

	// Follow false branch
	if condition.False != nil && condition.False.Next != "" {
		if issue := walkCondition(rules, condition.False.Next, path); issue != nil {
			return issue
		}
	}

	delete(path, conditionName)
	return nil
}

// detectDanglingReferences checks that all next references point to existing conditions.
func detectDanglingReferences(rules *Rules) []ValidationIssue {
	var issues []ValidationIssue

	for name, condition := range rules.Conditions {
		if condition.True != nil && condition.True.Next != "" {
			if _, ok := rules.Conditions[condition.True.Next]; !ok {
				issues = append(issues, ValidationIssue{
					Severity:      SeverityError,
					ConditionName: name,
					Message:       fmt.Sprintf("true branch references non-existent condition '%s'", condition.True.Next),
				})
			}
		}
		if condition.False != nil && condition.False.Next != "" {
			if _, ok := rules.Conditions[condition.False.Next]; !ok {
				issues = append(issues, ValidationIssue{
					Severity:      SeverityError,
					ConditionName: name,
					Message:       fmt.Sprintf("false branch references non-existent condition '%s'", condition.False.Next),
				})
			}
		}
	}

	return issues
}

// detectUnreachableConditions finds conditions that are never referenced by any other
// condition's next pointer and are not the default condition.
func detectUnreachableConditions(rules *Rules) []ValidationIssue {
	var issues []ValidationIssue

	referenced := make(map[string]bool)

	// The default condition is always reachable (it's the entry point)
	if rules.DefaultCondition != nil {
		referenced[rules.DefaultCondition.Name] = true
	}

	// Collect all conditions referenced via next pointers
	for _, condition := range rules.Conditions {
		if condition.True != nil && condition.True.Next != "" {
			referenced[condition.True.Next] = true
		}
		if condition.False != nil && condition.False.Next != "" {
			referenced[condition.False.Next] = true
		}
	}

	for name := range rules.Conditions {
		if !referenced[name] {
			issues = append(issues, ValidationIssue{
				Severity:      SeverityWarning,
				ConditionName: name,
				Message:       "condition is unreachable (not referenced by any other condition and is not the default)",
			})
		}
	}

	return issues
}

// ValidateLibraryDependencies checks for circular dependencies in the library's
// require chains (rule set A requires B which requires A).
func (rl *RulesLibrary) ValidateLibraryDependencies() []ValidationIssue {
	var issues []ValidationIssue

	for name := range rl.rulePaths {
		path := make(map[string]bool)
		chain := []string{}
		if cycle := rl.walkRequireDeps(name, path, chain); cycle != nil {
			issues = append(issues, *cycle)
		}
	}

	// Deduplicate
	seen := make(map[string]bool)
	var deduplicated []ValidationIssue
	for _, issue := range issues {
		if !seen[issue.Message] {
			seen[issue.Message] = true
			deduplicated = append(deduplicated, issue)
		}
	}

	return deduplicated
}

// walkRequireDeps performs DFS through the require dependency graph.
func (rl *RulesLibrary) walkRequireDeps(name string, path map[string]bool, chain []string) *ValidationIssue {
	if path[name] {
		// Build the cycle path for a clear error message
		cycleStart := 0
		for i, n := range chain {
			if n == name {
				cycleStart = i
				break
			}
		}
		cyclePath := append(chain[cycleStart:], name)
		return &ValidationIssue{
			Severity: SeverityError,
			Message:  fmt.Sprintf("circular require dependency: %s", formatCyclePath(cyclePath)),
		}
	}

	path[name] = true
	chain = append(chain, name)

	for _, dep := range rl.dependencies[name] {
		if issue := rl.walkRequireDeps(dep, path, chain); issue != nil {
			return issue
		}
	}

	delete(path, name)
	return nil
}

func formatCyclePath(path []string) string {
	var result strings.Builder
	for i, name := range path {
		if i > 0 {
			result.WriteString(" -> ")
		}
		result.WriteString(name)
	}
	return result.String()
}
