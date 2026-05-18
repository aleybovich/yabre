package yabre

import (
	"errors"
	"testing"

	"github.com/dop251/goja"
	"gopkg.in/yaml.v2"
)

func TestDecision_UnmarshalYAML_ValidNext(t *testing.T) {
	yamlData := `
description: "Test decision"
action: "doSomething()"
next: "nextCondition"
`
	var decision Decision
	err := yaml.Unmarshal([]byte(yamlData), &decision)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if decision.Description != "Test decision" {
		t.Errorf("Expected description 'Test decision', got: %s", decision.Description)
	}
	if decision.Action != "doSomething()" {
		t.Errorf("Expected action 'doSomething()', got: %s", decision.Action)
	}
	if decision.Next != "nextCondition" {
		t.Errorf("Expected next 'nextCondition', got: %s", decision.Next)
	}
	if decision.Terminate {
		t.Error("Expected terminate to be false")
	}
}

func TestDecision_UnmarshalYAML_ValidTerminate(t *testing.T) {
	yamlData := `
description: "Test decision"
action: "doSomething()"
terminate: true
`
	var decision Decision
	err := yaml.Unmarshal([]byte(yamlData), &decision)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !decision.Terminate {
		t.Error("Expected terminate to be true")
	}
	if decision.Next != "" {
		t.Errorf("Expected next to be empty, got: %s", decision.Next)
	}
}

func TestDecision_UnmarshalYAML_ErrorBothNextAndTerminate(t *testing.T) {
	yamlData := `
description: "Test decision"
action: "doSomething()"
next: "nextCondition"
terminate: true
`
	var decision Decision
	err := yaml.Unmarshal([]byte(yamlData), &decision)

	if err == nil {
		t.Fatal("Expected error when both next and terminate are set")
	}
	if err.Error() != "next and terminate cannot be used together" {
		t.Errorf("Expected specific error message, got: %v", err)
	}
}

func TestDecision_UnmarshalYAML_InvalidYAML(t *testing.T) {
	yamlData := `
description: "Test decision
action: "doSomething()"
next: "nextCondition"
`
	var decision Decision
	err := yaml.Unmarshal([]byte(yamlData), &decision)

	if err == nil {
		t.Fatal("Expected error for invalid YAML")
	}
}

func TestRunCondition_CheckFunctionNotFound(t *testing.T) {
	vm := goja.New()
	runner := &RulesRunner[any]{
		decisionCallback: func(format string, args ...any) {},
	}

	rules := &Rules{
		Conditions: map[string]Condition{
			"test": {
				Name:        "test",
				Description: "Test condition",
				Check:       "function() { return true; }",
			},
		},
	}

	condition := rules.Conditions["test"]
	// Don't inject the function — it should fail to find it
	err := runner.runCondition(vm, rules, &condition)

	if err == nil {
		t.Fatal("Expected error for missing check function")
	}
	if err.Error() != "check function not found: "+conditionCheckFuncName("test") {
		t.Errorf("Expected specific error message, got: %v", err)
	}
}

func TestRunCondition_CheckFunctionExecutionError(t *testing.T) {
	vm := goja.New()
	runner := &RulesRunner[any]{
		decisionCallback: func(format string, args ...any) {},
	}

	// Register a function under the condition name that throws
	vm.Set(conditionCheckFuncName("test"), func() error {
		return errors.New("check error")
	})

	rules := &Rules{
		Conditions: map[string]Condition{
			"test": {
				Name:        "test",
				Description: "Test condition",
				Check:       "function() { throw 'check error'; }",
			},
		},
	}

	condition := rules.Conditions["test"]
	err := runner.runCondition(vm, rules, &condition)

	if err == nil {
		t.Fatal("Expected error from check function")
	}
	if !contains(err.Error(), "error evaluating check function") {
		t.Errorf("Expected error evaluating check function, got: %v", err)
	}
}

func TestRunCondition_TrueBranchExecution(t *testing.T) {
	vm := goja.New()
	actionExecuted := false

	runner := &RulesRunner[any]{
		decisionCallback: func(format string, args ...any) {},
	}

	// Register functions under the deterministic names
	vm.Set(conditionCheckFuncName("test"), func() bool { return true })
	vm.Set(decisionActionFuncName("test", true), func() { actionExecuted = true })

	rules := &Rules{
		Conditions: map[string]Condition{
			"test": {
				Name:        "test",
				Description: "Test condition",
				Check:       "function() { return true; }",
				True: &Decision{
					Name:        "test_true",
					Description: "True action",
					Action:      "function() { }",
					Terminate:   true,
				},
			},
		},
	}

	condition := rules.Conditions["test"]
	err := runner.runCondition(vm, rules, &condition)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !actionExecuted {
		t.Error("Expected true branch action to be executed")
	}
}

func TestRunCondition_FalseBranchExecution(t *testing.T) {
	vm := goja.New()
	actionExecuted := false

	runner := &RulesRunner[any]{
		decisionCallback: func(format string, args ...any) {},
	}

	// Register functions under the deterministic names
	vm.Set(conditionCheckFuncName("test"), func() bool { return false })
	vm.Set(decisionActionFuncName("test", false), func() { actionExecuted = true })

	rules := &Rules{
		Conditions: map[string]Condition{
			"test": {
				Name:        "test",
				Description: "Test condition",
				Check:       "function() { return false; }",
				False: &Decision{
					Name:        "test_false",
					Description: "False action",
					Action:      "function() { }",
					Terminate:   true,
				},
			},
		},
	}

	condition := rules.Conditions["test"]
	err := runner.runCondition(vm, rules, &condition)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !actionExecuted {
		t.Error("Expected false branch action to be executed")
	}
}

func TestRunCondition_NullDecisionHandling(t *testing.T) {
	vm := goja.New()
	runner := &RulesRunner[any]{
		decisionCallback: func(format string, args ...any) {},
	}

	// Register function under condition name
	vm.Set(conditionCheckFuncName("test"), func() bool { return true })

	rules := &Rules{
		Conditions: map[string]Condition{
			"test": {
				Name:        "test",
				Description: "Test condition",
				Check:       "function() { return true; }",
				// No True or False decision
			},
		},
	}

	condition := rules.Conditions["test"]
	err := runner.runCondition(vm, rules, &condition)

	if err != nil {
		t.Fatalf("Expected no error for null decision, got: %v", err)
	}
}

func TestRunAction_ActionFunctionNotFound(t *testing.T) {
	vm := goja.New()
	runner := &RulesRunner[any]{
		decisionCallback: func(format string, args ...any) {},
	}

	rules := &Rules{}
	decision := &Decision{
		Name:   "test_true",
		Action: "function() { }",
	}

	err := runner.runAction(vm, rules, decision)

	if err == nil {
		t.Fatal("Expected error for missing action function")
	}
	if err.Error() != "action function not found: test_true" {
		t.Errorf("Expected specific error message, got: %v", err)
	}
}

func TestRunAction_ActionFunctionExecutionError(t *testing.T) {
	vm := goja.New()
	runner := &RulesRunner[any]{
		decisionCallback: func(format string, args ...any) {},
	}

	// Register a function that throws under the decision name
	vm.Set("test_true", func() error {
		return errors.New("action error")
	})

	rules := &Rules{}
	decision := &Decision{
		Name:   "test_true",
		Action: "function() { throw 'action error'; }",
	}

	err := runner.runAction(vm, rules, decision)

	if err == nil {
		t.Fatal("Expected error from action function")
	}
	if !contains(err.Error(), "error running action") {
		t.Errorf("Expected error running action, got: %v", err)
	}
}

func TestRunAction_NextConditionNotFound(t *testing.T) {
	vm := goja.New()
	runner := &RulesRunner[any]{
		decisionCallback: func(format string, args ...any) {},
	}

	rules := &Rules{
		Conditions: make(map[string]Condition),
	}
	decision := &Decision{
		Name: "test_true",
		Next: "nonExistentCondition",
	}

	err := runner.runAction(vm, rules, decision)

	if err == nil {
		t.Fatal("Expected error for missing next condition")
	}
	if !contains(err.Error(), "condition 'nonExistentCondition' not found") {
		t.Errorf("Expected condition not found error, got: %v", err)
	}
}

func TestRunAction_TerminateFlagBehavior(t *testing.T) {
	vm := goja.New()
	terminated := false

	runner := &RulesRunner[any]{
		decisionCallback: func(format string, args ...any) {
			if format == "Terminating" {
				terminated = true
			}
		},
	}

	rules := &Rules{}
	decision := &Decision{
		Name:      "test_true",
		Terminate: true,
	}

	err := runner.runAction(vm, rules, decision)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !terminated {
		t.Error("Expected termination callback to be called")
	}
}

func TestFindConditionByName_ExistingCondition(t *testing.T) {
	rules := &Rules{
		Conditions: map[string]Condition{
			"test": {
				Name:        "test",
				Description: "Test condition",
			},
		},
	}

	condition, err := findConditionByName(rules, "test")

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if condition == nil {
		t.Fatal("Expected condition to be found")
	}
	if condition.Name != "test" {
		t.Errorf("Expected condition name 'test', got: %s", condition.Name)
	}
}

func TestFindConditionByName_ConditionNotFound(t *testing.T) {
	rules := &Rules{
		Conditions: make(map[string]Condition),
	}

	condition, err := findConditionByName(rules, "nonExistent")

	if err == nil {
		t.Fatal("Expected error for non-existent condition")
	}
	if condition != nil {
		t.Error("Expected nil condition")
	}
	if err.Error() != "Condition not found: nonExistent" {
		t.Errorf("Expected specific error message, got: %v", err)
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && len(substr) > 0 && findInString(s, substr))
}

func findInString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
