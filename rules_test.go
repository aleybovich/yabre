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

func TestLoadRulesFromYaml_InvalidYAMLError(t *testing.T) {
	runner := &RulesRunner[any]{}
	
	invalidYAML := []byte(`
name: "invalid
conditions:
  condition1:
    description: "Missing quote
`)
	
	rules, err := runner.loadRulesFromYaml(invalidYAML)
	
	if err == nil {
		t.Fatal("Expected error for invalid YAML")
	}
	if rules != nil {
		t.Error("Expected nil rules for invalid YAML")
	}
	if !strings.Contains(err.Error(), "error parsing YAML") {
		t.Errorf("Expected YAML parsing error, got: %v", err)
	}
}

func TestLoadRulesFromYaml_EmptyYAML(t *testing.T) {
	runner := &RulesRunner[any]{}
	
	emptyYAML := []byte("")
	
	rules, err := runner.loadRulesFromYaml(emptyYAML)
	
	if err != nil {
		t.Fatalf("Unexpected error for empty YAML: %v", err)
	}
	if rules == nil {
		t.Fatal("Expected non-nil rules for empty YAML")
	}
	if len(rules.Conditions) != 0 {
		t.Errorf("Expected no conditions, got: %d", len(rules.Conditions))
	}
}

func TestAddJsFunctions_ScriptInjectionErrors(t *testing.T) {
	vm := goja.New()
	runner := &RulesRunner[any]{
		functionNames: make(map[string]string),
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
		functionNames: make(map[string]string),
		Rules: &Rules{
			Conditions: map[string]Condition{
				"test": {
					Name:  "test",
					Check: "function check() { return",  // Invalid JS
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
		functionNames: make(map[string]string),
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
	
	// Verify no function was registered for empty check
	if _, exists := runner.functionNames["test"]; exists {
		t.Error("Expected no function name mapping for empty check")
	}
}

func TestInjectJSFunction_VariousFunctionNamePatterns(t *testing.T) {
	tests := []struct {
		name         string
		funcCode     string
		defaultName  string
		expectedName string
	}{
		{
			name:         "Named function",
			funcCode:     "function myFunc() { return true; }",
			defaultName:  "default",
			expectedName: "myFunc",
		},
		{
			name:         "Named function with spaces",
			funcCode:     "function    spacedFunc   () { return true; }",
			defaultName:  "default",
			expectedName: "spacedFunc",
		},
		{
			name:         "Function with parameters",
			funcCode:     "function withParams(a, b) { return a + b; }",
			defaultName:  "default",
			expectedName: "withParams",
		},
		{
			name:         "No function keyword",
			funcCode:     "() => { return true; }",
			defaultName:  "arrowDefault",
			expectedName: "arrowDefault",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vm := goja.New()
			runner := &RulesRunner[any]{
				functionNames: make(map[string]string),
			}
			
			err := runner.injectJSFunction(vm, tt.defaultName, tt.funcCode)
			
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			
			if runner.functionNames[tt.defaultName] != tt.expectedName {
				t.Errorf("Expected function name '%s', got: '%s'", tt.expectedName, runner.functionNames[tt.defaultName])
			}
			
			// Verify function exists in VM
			funcVal := vm.Get(tt.expectedName)
			if funcVal == nil || funcVal == goja.Undefined() {
				t.Errorf("Expected function '%s' to exist in VM", tt.expectedName)
			}
		})
	}
}

func TestInjectJSFunction_ArrowFunctions(t *testing.T) {
	vm := goja.New()
	runner := &RulesRunner[any]{
		functionNames: make(map[string]string),
	}
	
	arrowFunc := "() => true"
	err := runner.injectJSFunction(vm, "arrowTest", arrowFunc)
	
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	
	// Should use default name for arrow functions
	if runner.functionNames["arrowTest"] != "arrowTest" {
		t.Errorf("Expected default name 'arrowTest' for arrow function, got: %s", runner.functionNames["arrowTest"])
	}
}

func TestInjectJSFunction_AnonymousFunctions(t *testing.T) {
	vm := goja.New()
	runner := &RulesRunner[any]{
		functionNames: make(map[string]string),
	}
	
	anonFunc := "function() { return 42; }"
	err := runner.injectJSFunction(vm, "anonTest", anonFunc)
	
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	
	// Should use default name for anonymous functions
	if runner.functionNames["anonTest"] != "anonTest" {
		t.Errorf("Expected default name 'anonTest' for anonymous function, got: %s", runner.functionNames["anonTest"])
	}
}

func TestInjectJSFunction_InvalidFunctionCode(t *testing.T) {
	vm := goja.New()
	runner := &RulesRunner[any]{
		functionNames: make(map[string]string),
	}
	
	invalidCode := "function broken() { return"
	err := runner.injectJSFunction(vm, "invalid", invalidCode)
	
	if err == nil {
		t.Fatal("Expected error for invalid function code")
	}
	if !strings.Contains(err.Error(), "error injecting function") {
		t.Errorf("Expected function injection error, got: %v", err)
	}
}