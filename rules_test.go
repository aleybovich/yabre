package yabre

import (
	"strings"
	"testing"

	"github.com/dop251/goja"
	"gopkg.in/yaml.v2"
)

func TestRules_UnmarshalYAML_MultipleDefaultConditions(t *testing.T) {
	yamlData := `
name: "test-rules"
conditions:
  condition1:
    description: "First condition"
    default: true
    check: "return true"
  condition2:
    description: "Second condition" 
    default: true
    check: "return false"
`
	var rules Rules
	err := yaml.Unmarshal([]byte(yamlData), &rules)
	
	if err == nil {
		t.Fatal("Expected error for multiple default conditions")
	}
	if err.Error() != "multiple default conditions found" {
		t.Errorf("Expected specific error message, got: %v", err)
	}
}

func TestRules_UnmarshalYAML_MissingRequiredFields(t *testing.T) {
	yamlData := `
conditions:
  condition1:
    description: "First condition"
    check: "return true"
`
	var rules Rules
	err := yaml.Unmarshal([]byte(yamlData), &rules)
	
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	
	// Name is optional, so this should work
	if rules.Name != "" {
		t.Errorf("Expected empty name, got: %s", rules.Name)
	}
	
	// Check that condition names are set
	cond := rules.Conditions["condition1"]
	if cond.Name != "condition1" {
		t.Errorf("Expected condition name to be set to 'condition1', got: %s", cond.Name)
	}
}

func TestRules_UnmarshalYAML_ComplexNestedConditions(t *testing.T) {
	yamlData := `
name: "complex-rules"
scripts: |
  function check1() { return true; }
  function action1() { console.log("action1"); }
conditions:
  condition1:
    description: "First condition"
    default: true
    check: "check1()"
    true:
      description: "True path"
      action: "action1()"
      next: "condition2"
    false:
      description: "False path"
      terminate: true
  condition2:
    description: "Second condition"
    check: "return false"
`
	var rules Rules
	err := yaml.Unmarshal([]byte(yamlData), &rules)
	
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	
	// Verify structure
	if rules.DefaultCondition == nil {
		t.Fatal("Expected default condition to be set")
	}
	if rules.DefaultCondition.Name != "condition1" {
		t.Errorf("Expected default condition name 'condition1', got: %s", rules.DefaultCondition.Name)
	}
	
	// Check true/false decision names
	cond1 := rules.Conditions["condition1"]
	if cond1.True.Name != "condition1_true" {
		t.Errorf("Expected true decision name 'condition1_true', got: %s", cond1.True.Name)
	}
	if cond1.False.Name != "condition1_false" {
		t.Errorf("Expected false decision name 'condition1_false', got: %s", cond1.False.Name)
	}
	
	// Check values
	if !cond1.True.Value {
		t.Error("Expected true decision value to be true")
	}
	if cond1.False.Value {
		t.Error("Expected false decision value to be false")
	}
}


func TestAddJsFunctions_ScriptInjectionErrors(t *testing.T) {
	vm := goja.New()
	runner := &RulesRunner[any]{
		Rules: &Rules{
			Scripts: "invalid javascript {{{",
		},
	}

	err := runner.addJsFunctions(vm)

	if err == nil {
		t.Fatal("Expected error for invalid JavaScript")
	}
	if !strings.Contains(err.Error(), "error injecting scripts into vm") {
		t.Errorf("Expected script injection error, got: %v", err)
	}
}

func TestAddJsFunctions_InvalidJavaScriptSyntax(t *testing.T) {
	vm := goja.New()
	runner := &RulesRunner[any]{
		Rules: &Rules{
			Conditions: map[string]Condition{
				"test": {
					Name:  "test",
					Check: "function check() { return", // Invalid JS
				},
			},
		},
	}

	err := runner.addJsFunctions(vm)

	if err == nil {
		t.Fatal("Expected error for invalid JavaScript function")
	}
	if !strings.Contains(err.Error(), "error injecting condition function into vm") {
		t.Errorf("Expected condition function injection error, got: %v", err)
	}
}

func TestAddJsFunctions_MissingCheckFunctions(t *testing.T) {
	vm := goja.New()
	runner := &RulesRunner[any]{
		Rules: &Rules{
			Conditions: map[string]Condition{
				"test": {
					Name: "test",
					// No check function
					True: &Decision{
						Name:   "test_true",
						Action: "function() { return true; }",
					},
				},
			},
		},
	}

	err := runner.addJsFunctions(vm)

	// Should not error on missing check, only when trying to inject non-empty functions
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify check function was NOT injected (empty check)
	checkVal := vm.Get(conditionCheckFuncName("test"))
	if checkVal != nil && checkVal != goja.Undefined() {
		t.Error("Expected no function registered for empty check")
	}
}

func TestInjectJSFunction_NamedFunction(t *testing.T) {
	vm := goja.New()

	// Named functions get bound to our generated var name, not their internal name
	err := injectJSFunction(vm, "my_check", "function userDefinedName() { return true; }")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should be callable via our generated name
	funcVal := vm.Get("my_check")
	if funcVal == nil || funcVal == goja.Undefined() {
		t.Error("Expected function 'my_check' to exist in VM")
	}
}

func TestInjectJSFunction_ArrowFunction(t *testing.T) {
	vm := goja.New()

	err := injectJSFunction(vm, "arrow_check", "() => true")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	funcVal := vm.Get("arrow_check")
	if funcVal == nil || funcVal == goja.Undefined() {
		t.Error("Expected function 'arrow_check' to exist in VM")
	}
}

func TestInjectJSFunction_AnonymousFunction(t *testing.T) {
	vm := goja.New()

	err := injectJSFunction(vm, "anon_check", "function() { return 42; }")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	funcVal := vm.Get("anon_check")
	if funcVal == nil || funcVal == goja.Undefined() {
		t.Error("Expected function 'anon_check' to exist in VM")
	}
}

func TestInjectJSFunction_InvalidCode(t *testing.T) {
	vm := goja.New()

	err := injectJSFunction(vm, "broken", "function broken() { return")
	if err == nil {
		t.Fatal("Expected error for invalid function code")
	}
	if !strings.Contains(err.Error(), "error injecting function") {
		t.Errorf("Expected function injection error, got: %v", err)
	}
}

func TestInjectJSFunction_DuplicateNamedFunctions(t *testing.T) {
	// Two conditions can define functions with the same internal name
	// because we bind them to different generated var names
	vm := goja.New()

	err := injectJSFunction(vm, "cond_a", "function isEligible() { return true; }")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	err = injectJSFunction(vm, "cond_b", "function isEligible() { return false; }")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Both should be independently callable
	funcA, ok := goja.AssertFunction(vm.Get("cond_a"))
	if !ok {
		t.Fatal("Expected cond_a to be a function")
	}
	funcB, ok := goja.AssertFunction(vm.Get("cond_b"))
	if !ok {
		t.Fatal("Expected cond_b to be a function")
	}

	resultA, _ := funcA(goja.Undefined())
	resultB, _ := funcB(goja.Undefined())

	if !resultA.ToBoolean() {
		t.Error("Expected cond_a to return true")
	}
	if resultB.ToBoolean() {
		t.Error("Expected cond_b to return false")
	}
}