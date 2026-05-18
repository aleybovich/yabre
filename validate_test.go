package yabre

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateRules_NoCircularDependencies(t *testing.T) {
	rules := &Rules{
		Conditions: map[string]Condition{
			"start": {
				Name:    "start",
				Default: true,
				Check:   "function() { return true; }",
				True: &Decision{
					Next: "step2",
				},
				False: &Decision{
					Terminate: true,
				},
			},
			"step2": {
				Name:  "step2",
				Check: "function() { return true; }",
				True: &Decision{
					Terminate: true,
				},
				False: &Decision{
					Terminate: true,
				},
			},
		},
		DefaultCondition: &Condition{Name: "start"},
	}

	issues := ValidateRules(rules)

	// No errors expected
	for _, issue := range issues {
		if issue.Severity == SeverityError {
			t.Errorf("unexpected error: %s", issue.String())
		}
	}
}

func TestValidateRules_DirectCircularDependency(t *testing.T) {
	rules := &Rules{
		Conditions: map[string]Condition{
			"condition1": {
				Name:  "condition1",
				Check: "function() { return true; }",
				True: &Decision{
					Next: "condition2",
				},
				False: &Decision{
					Terminate: true,
				},
			},
			"condition2": {
				Name:  "condition2",
				Check: "function() { return true; }",
				True: &Decision{
					Next: "condition1", // circular!
				},
				False: &Decision{
					Terminate: true,
				},
			},
		},
	}

	issues := ValidateRules(rules)

	var errors []ValidationIssue
	for _, issue := range issues {
		if issue.Severity == SeverityError {
			errors = append(errors, issue)
		}
	}

	require.NotEmpty(t, errors, "expected at least one circular dependency error")
	assert.Contains(t, errors[0].Message, "circular dependency detected")
}

func TestValidateRules_SelfReference(t *testing.T) {
	rules := &Rules{
		Conditions: map[string]Condition{
			"loop": {
				Name:  "loop",
				Check: "function() { return true; }",
				True: &Decision{
					Next: "loop", // self-referencing
				},
				False: &Decision{
					Terminate: true,
				},
			},
		},
	}

	issues := ValidateRules(rules)

	var errors []ValidationIssue
	for _, issue := range issues {
		if issue.Severity == SeverityError {
			errors = append(errors, issue)
		}
	}

	require.NotEmpty(t, errors)
	assert.Contains(t, errors[0].Message, "circular dependency detected")
	assert.Contains(t, errors[0].Message, "loop")
}

func TestValidateRules_LongChainCircularDependency(t *testing.T) {
	// A -> B -> C -> D -> B (cycle of length 3)
	rules := &Rules{
		Conditions: map[string]Condition{
			"A": {
				Name:  "A",
				Check: "function() { return true; }",
				True:  &Decision{Next: "B"},
				False: &Decision{Terminate: true},
			},
			"B": {
				Name:  "B",
				Check: "function() { return true; }",
				True:  &Decision{Next: "C"},
				False: &Decision{Terminate: true},
			},
			"C": {
				Name:  "C",
				Check: "function() { return true; }",
				True:  &Decision{Next: "D"},
				False: &Decision{Terminate: true},
			},
			"D": {
				Name:  "D",
				Check: "function() { return true; }",
				True:  &Decision{Next: "B"}, // back to B
				False: &Decision{Terminate: true},
			},
		},
		DefaultCondition: &Condition{Name: "A"},
	}

	issues := ValidateRules(rules)

	var errors []ValidationIssue
	for _, issue := range issues {
		if issue.Severity == SeverityError {
			errors = append(errors, issue)
		}
	}

	require.NotEmpty(t, errors, "expected circular dependency error for B->C->D->B")
	assert.Contains(t, errors[0].Message, "circular dependency detected")
}

func TestValidateRules_CircularOnFalseBranch(t *testing.T) {
	rules := &Rules{
		Conditions: map[string]Condition{
			"check1": {
				Name:  "check1",
				Check: "function() { return true; }",
				True:  &Decision{Terminate: true},
				False: &Decision{Next: "check2"},
			},
			"check2": {
				Name:  "check2",
				Check: "function() { return true; }",
				True:  &Decision{Terminate: true},
				False: &Decision{Next: "check1"}, // cycle via false branches
			},
		},
	}

	issues := ValidateRules(rules)

	var errors []ValidationIssue
	for _, issue := range issues {
		if issue.Severity == SeverityError {
			errors = append(errors, issue)
		}
	}

	require.NotEmpty(t, errors)
	assert.Contains(t, errors[0].Message, "circular dependency detected")
}

func TestValidateRules_DanglingReference(t *testing.T) {
	rules := &Rules{
		Conditions: map[string]Condition{
			"start": {
				Name:    "start",
				Default: true,
				Check:   "function() { return true; }",
				True: &Decision{
					Next: "nonexistent",
				},
				False: &Decision{
					Terminate: true,
				},
			},
		},
		DefaultCondition: &Condition{Name: "start"},
	}

	issues := ValidateRules(rules)

	var errors []ValidationIssue
	for _, issue := range issues {
		if issue.Severity == SeverityError {
			errors = append(errors, issue)
		}
	}

	require.NotEmpty(t, errors)
	assert.Contains(t, errors[0].Message, "non-existent condition 'nonexistent'")
}

func TestValidateRules_UnreachableCondition(t *testing.T) {
	rules := &Rules{
		Conditions: map[string]Condition{
			"start": {
				Name:    "start",
				Default: true,
				Check:   "function() { return true; }",
				True:    &Decision{Terminate: true},
				False:   &Decision{Terminate: true},
			},
			"orphan": {
				Name:  "orphan",
				Check: "function() { return true; }",
				True:  &Decision{Terminate: true},
				False: &Decision{Terminate: true},
			},
		},
		DefaultCondition: &Condition{Name: "start"},
	}

	issues := ValidateRules(rules)

	var warnings []ValidationIssue
	for _, issue := range issues {
		if issue.Severity == SeverityWarning {
			warnings = append(warnings, issue)
		}
	}

	require.NotEmpty(t, warnings)
	assert.Equal(t, "orphan", warnings[0].ConditionName)
	assert.Contains(t, warnings[0].Message, "unreachable")
}

func TestValidateRules_ComplexGraphNoCycle(t *testing.T) {
	// Diamond pattern: A -> B, A -> C, B -> D, C -> D (no cycle)
	rules := &Rules{
		Conditions: map[string]Condition{
			"A": {
				Name:    "A",
				Default: true,
				Check:   "function() { return true; }",
				True:    &Decision{Next: "B"},
				False:   &Decision{Next: "C"},
			},
			"B": {
				Name:  "B",
				Check: "function() { return true; }",
				True:  &Decision{Next: "D"},
				False: &Decision{Terminate: true},
			},
			"C": {
				Name:  "C",
				Check: "function() { return true; }",
				True:  &Decision{Next: "D"},
				False: &Decision{Terminate: true},
			},
			"D": {
				Name:  "D",
				Check: "function() { return true; }",
				True:  &Decision{Terminate: true},
				False: &Decision{Terminate: true},
			},
		},
		DefaultCondition: &Condition{Name: "A"},
	}

	issues := ValidateRules(rules)

	for _, issue := range issues {
		if issue.Severity == SeverityError {
			t.Errorf("unexpected error in diamond graph: %s", issue.String())
		}
	}
}

func TestValidateLibraryDependencies_NoCycle(t *testing.T) {
	rl := &RulesLibrary{
		rulePaths: map[string]string{
			"a": "a.yaml",
			"b": "b.yaml",
			"c": "c.yaml",
		},
		dependencies: map[string][]string{
			"a": {"b"},
			"b": {"c"},
			"c": {},
		},
	}

	issues := rl.ValidateLibraryDependencies()
	for _, issue := range issues {
		if issue.Severity == SeverityError {
			t.Errorf("unexpected error: %s", issue.String())
		}
	}
}

func TestValidateLibraryDependencies_DirectCycle(t *testing.T) {
	rl := &RulesLibrary{
		rulePaths: map[string]string{
			"a": "a.yaml",
			"b": "b.yaml",
		},
		dependencies: map[string][]string{
			"a": {"b"},
			"b": {"a"}, // cycle: a -> b -> a
		},
	}

	issues := rl.ValidateLibraryDependencies()

	var errors []ValidationIssue
	for _, issue := range issues {
		if issue.Severity == SeverityError {
			errors = append(errors, issue)
		}
	}

	require.NotEmpty(t, errors)
	assert.Contains(t, errors[0].Message, "circular require dependency")
}

func TestValidateLibraryDependencies_IndirectCycle(t *testing.T) {
	rl := &RulesLibrary{
		rulePaths: map[string]string{
			"a": "a.yaml",
			"b": "b.yaml",
			"c": "c.yaml",
		},
		dependencies: map[string][]string{
			"a": {"b"},
			"b": {"c"},
			"c": {"a"}, // cycle: a -> b -> c -> a
		},
	}

	issues := rl.ValidateLibraryDependencies()

	var errors []ValidationIssue
	for _, issue := range issues {
		if issue.Severity == SeverityError {
			errors = append(errors, issue)
		}
	}

	require.NotEmpty(t, errors)
	assert.Contains(t, errors[0].Message, "circular require dependency")
	// The cycle may start at any node depending on map iteration order
	msg := errors[0].Message
	assert.True(t,
		strings.Contains(msg, "a -> b -> c -> a") ||
			strings.Contains(msg, "b -> c -> a -> b") ||
			strings.Contains(msg, "c -> a -> b -> c"),
		"expected cycle path in message, got: %s", msg,
	)
}
