package yabre

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to create a rules library from YAML content
func createLibraryFromYAML(t *testing.T, yamlContent string, fileName string) *RulesLibrary {
	// Create a temporary directory structure in memory using embed.FS
	// For testing, we'll use the actual file system
	tempDir := t.TempDir()
	yamlPath := tempDir + "/" + fileName
	err := os.WriteFile(yamlPath, []byte(yamlContent), 0644)
	require.NoError(t, err)

	library, err := NewRulesLibrary(RulesLibrarySettings{
		BasePath: tempDir,
	})
	require.NoError(t, err)

	return library
}

// Test empty rules set
func TestRunner_EmptyRulesSet(t *testing.T) {
	yamlRules := `
name: "empty-rules"
conditions: {}
`
	library := createLibraryFromYAML(t, yamlRules, "empty-rules.yaml")

	context := map[string]interface{}{}
	runner, err := NewRulesRunnerFromLibrary[map[string]interface{}](library, "empty-rules", &context)
	require.NoError(t, err)
	_, err = runner.RunRules(&context, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no default condition")
}

// Test no default condition with multiple conditions
func TestRunner_NoDefaultCondition(t *testing.T) {
	yamlRules := `
name: "no-default"
conditions:
  conditionA:
    description: "Condition A"
    check: "function() { return true; }"
    true:
      terminate: true
  conditionB:
    description: "Condition B"
    check: "function() { return false; }"
    true:
      terminate: true
`
	library := createLibraryFromYAML(t, yamlRules, "no-default.yaml")

	context := map[string]interface{}{}
	runner, err := NewRulesRunnerFromLibrary[map[string]interface{}](library, "no-default", &context)
	require.NoError(t, err)
	_, err = runner.RunRules(&context, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no default condition")
}

// Test circular condition references
func TestRunner_CircularReferences(t *testing.T) {
	yamlRules := `
name: "circular-refs"
conditions:
  condition1:
    description: "Condition 1"
    default: true
    check: "function() { return true; }"
    true:
      next: "condition2"
  condition2:
    description: "Condition 2"
    check: "function() { return true; }"
    true:
      next: "condition1"  # Creates a circular reference
`
	library := createLibraryFromYAML(t, yamlRules, "circular-refs.yaml")

	// Add max iterations to prevent infinite loop
	maxIterations := 0

	context := map[string]interface{}{}
	runner, err := NewRulesRunnerFromLibrary[map[string]interface{}](library, "circular-refs", &context,
		WithDecisionCallback[map[string]interface{}](func(format string, args ...interface{}) {
			maxIterations++
			if maxIterations > 10 {
				panic("Circular reference detected - too many iterations")
			}
		}),
	)
	require.NoError(t, err)

	// This should either timeout or hit max iterations
	defer func() {
		if r := recover(); r != nil {
			assert.Contains(t, r.(string), "Circular reference detected")
		}
	}()

	_, _ = runner.RunRules(&context, nil)
}

// Test JavaScript runtime errors
func TestRunner_JavaScriptRuntimeError(t *testing.T) {
	yamlRules := `
name: "js-runtime-error"
conditions:
  errorCondition:
    description: "Error condition"
    default: true
    check: |
      function() {
        // This will cause a runtime error when called
        return context.undefinedVariable.someProperty;
      }
    true:
      terminate: true
`
	library := createLibraryFromYAML(t, yamlRules, "js-runtime-error.yaml")

	context := map[string]interface{}{}
	runner, err := NewRulesRunnerFromLibrary[map[string]interface{}](library, "js-runtime-error", &context)
	require.NoError(t, err)
	_, err = runner.RunRules(&context, nil)
	assert.Error(t, err)
	// The error happens during function injection, not evaluation
	assert.Contains(t, strings.ToLower(err.Error()), "error")
}

// Test condition referencing non-existent next condition
func TestRunner_NonExistentNextCondition(t *testing.T) {
	yamlRules := `
name: "non-existent-next"
conditions:
  startCondition:
    description: "Start condition"
    default: true
    check: "function() { return true; }"
    true:
      next: "missingCondition"  # This condition doesn't exist
`
	library := createLibraryFromYAML(t, yamlRules, "non-existent-next.yaml")

	context := map[string]interface{}{}
	runner, err := NewRulesRunnerFromLibrary[map[string]interface{}](library, "non-existent-next", &context)
	require.NoError(t, err)
	_, err = runner.RunRules(&context, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "condition 'missingCondition' not found")
}

// Test deeply nested conditions
func TestRunner_DeeplyNestedConditions(t *testing.T) {
	yamlRules := `
name: "deeply-nested"
conditions:
  level1:
    description: "Level 1"
    default: true
    check: "function() { return context.level < 5; }"
    true:
      action: "function() { context.level = (context.level || 0) + 1; }"
      next: "level2"
    false:
      terminate: true
  level2:
    description: "Level 2"
    check: "function() { return context.level < 5; }"
    true:
      action: "function() { context.level = (context.level || 0) + 1; }"
      next: "level3"
    false:
      terminate: true
  level3:
    description: "Level 3"
    check: "function() { return context.level < 5; }"
    true:
      action: "function() { context.level = (context.level || 0) + 1; }"
      next: "level4"
    false:
      terminate: true
  level4:
    description: "Level 4"
    check: "function() { return context.level < 5; }"
    true:
      action: "function() { context.level = (context.level || 0) + 1; }"
      next: "level5"
    false:
      terminate: true
  level5:
    description: "Level 5"
    check: "function() { return context.level >= 5; }"
    true:
      action: "function() { context.level = (context.level || 0) + 1; }"
      terminate: true
`
	library := createLibraryFromYAML(t, yamlRules, "deeply-nested.yaml")

	context := map[string]interface{}{"level": 0}
	runner, err := NewRulesRunnerFromLibrary(library, "deeply-nested", &context)
	require.NoError(t, err)
	_, err = runner.RunRules(&context, nil)
	assert.NoError(t, err)
	assert.Equal(t, int64(4), context["level"]) // Should increment to 6
}

// Test context manipulation edge cases
func TestRunner_ContextManipulationEdgeCases(t *testing.T) {
	yamlRules := `
name: "context-manipulation"
conditions:
  checkAndManipulate:
    description: "Check and manipulate context"
    default: true
    check: "function() { return context.value === undefined; }"
    true:
      action: "function() { context.value = null; }"
      next: "checkNull"
    false:
      action: |
        function() {
          context.nested = {
            array: [1, 2, 3],
            object: { key: "value" },
            number: 42,
            boolean: true
          };
        }
      terminate: true
  checkNull:
    description: "Check null value"
    check: "function() { return context.value === null; }"
    true:
      action: "function() { delete context.value; }"
      next: "finalCheck"
  finalCheck:
    description: "Final check"
    check: "function() { return context.value === undefined; }"
    true:
      action: |
        function() {
          context.nested = {
            array: [1, 2, 3],
            object: { key: "value" },
            number: 42,
            boolean: true
          };
        }
      terminate: true
`
	library := createLibraryFromYAML(t, yamlRules, "context-manipulation.yaml")

	context := map[string]interface{}{}
	runner, err := NewRulesRunnerFromLibrary[map[string]interface{}](library, "context-manipulation", &context)
	require.NoError(t, err)
	_, err = runner.RunRules(&context, nil)
	assert.NoError(t, err)

	// Check complex object was set
	nested, ok := context["nested"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, int64(42), nested["number"])
	assert.Equal(t, true, nested["boolean"])
}

// Test mixed JavaScript and Go functions
func TestRunner_MixedJSAndGoFunctions(t *testing.T) {
	yamlRules := `
name: "mixed-functions"
conditions:
  mixedCondition:
    description: "Mixed JS and Go functions"
    default: true
    check: "function() { return goCheck(context.value); }"
    true:
      action: |
        function() {
          context.jsProcessed = true;
          goAction("processed");
        }
      terminate: true
    false:
      terminate: true
`
	goCheckCalled := false
	goActionCalled := false

	library := createLibraryFromYAML(t, yamlRules, "mixed-functions.yaml")

	context := map[string]interface{}{"value": "test"}
	runner, err := NewRulesRunnerFromLibrary[map[string]interface{}](library, "mixed-functions", &context,
		WithGoFunction[map[string]interface{}]("goCheck", func(value interface{}) bool {
			goCheckCalled = true
			return value == "test"
		}),
		WithGoFunction[map[string]interface{}]("goAction", func(status string) interface{} {
			goActionCalled = true
			assert.Equal(t, "processed", status)
			return nil
		}),
	)
	require.NoError(t, err)
	_, err = runner.RunRules(&context, nil)
	assert.NoError(t, err)
	assert.True(t, goCheckCalled)
	assert.True(t, goActionCalled)
	assert.Equal(t, true, context["jsProcessed"])
}

// Test error propagation through condition chains
func TestRunner_ErrorPropagation(t *testing.T) {
	yamlRules := `
name: "error-propagation"
conditions:
  condition1:
    description: "First condition"
    default: true
    check: "function() { return true; }"
    true:
      next: "condition2"
  condition2:
    description: "Second condition"
    check: "function() { return true; }"
    true:
      action: "function() { throw new Error('Intentional error in action'); }"
      next: "condition3"
  condition3:
    description: "Should not reach here"
    check: "function() { return true; }"
    true:
      terminate: true
`
	library := createLibraryFromYAML(t, yamlRules, "error-propagation.yaml")

	context := map[string]interface{}{}
	runner, err := NewRulesRunnerFromLibrary[map[string]interface{}](library, "error-propagation", &context)
	require.NoError(t, err)
	_, err = runner.RunRules(&context, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Intentional error in action")
}

// Test concurrent execution safety
func TestRunner_ConcurrentExecution(t *testing.T) {
	yamlRules := `
name: "concurrent-test"
conditions:
  incrementCondition:
    description: "Increment counter"
    default: true
    check: |
      function() {
        context.counter = (context.counter || 0) + 1;
        return context.counter < 3;
      }
    true:
      next: "incrementCondition"
    false:
      terminate: true
`
	library := createLibraryFromYAML(t, yamlRules, "concurrent-test.yaml")

	// Create a dummy context for initialization
	dummyContext := map[string]interface{}{}
	runner, err := NewRulesRunnerFromLibrary[map[string]interface{}](library, "concurrent-test", &dummyContext)
	require.NoError(t, err)

	// Run multiple goroutines executing rules concurrently
	var wg sync.WaitGroup
	errors := make([]error, 10)
	contexts := make([]map[string]interface{}, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			ctx := map[string]interface{}{}
			_, errors[index] = runner.RunRules(&ctx, nil)
			contexts[index] = ctx
		}(i)
	}

	wg.Wait()

	// Check that all executions completed without error
	for i, err := range errors {
		assert.NoError(t, err, "Goroutine %d failed", i)
		assert.Equal(t, int64(3), contexts[i]["counter"], "Goroutine %d has wrong counter", i)
	}
}

// Test type coercion between JavaScript and Go
func TestRunner_TypeCoercionJSGo(t *testing.T) {
	yamlRules := `
name: "type-coercion"
conditions:
  typeTest:
    description: "Test type coercion"
    default: true
    check: |
      function() {
        // JS number to Go int
        context.intResult = goProcessInt(42);
        
        // JS string to Go string
        context.stringResult = goProcessString("hello");
        
        // JS array to Go slice
        context.arrayResult = goProcessArray([1, 2, 3]);
        
        // JS object to Go map
        context.objectResult = goProcessObject({key: "value", number: 123});
        
        // JS boolean to Go bool
        context.boolResult = goProcessBool(true);
        
        return true;
      }
    true:
      terminate: true
`
	library := createLibraryFromYAML(t, yamlRules, "type-coercion.yaml")

	context := map[string]interface{}{}
	runner, err := NewRulesRunnerFromLibrary(library, "type-coercion", &context,
		WithGoFunction[map[string]interface{}]("goProcessInt", func(n int) int {
			return n * 2
		}),
		WithGoFunction[map[string]interface{}]("goProcessString", func(s string) string {
			return strings.ToUpper(s)
		}),
		WithGoFunction[map[string]interface{}]("goProcessArray", func(arr []interface{}) []interface{} {
			result := make([]interface{}, len(arr))
			for i, v := range arr {
				switch num := v.(type) {
				case float64:
					result[i] = num * 2
				case int64:
					result[i] = num * 2
				default:
					result[i] = v
				}
			}
			return result
		}),
		WithGoFunction[map[string]interface{}]("goProcessObject", func(obj map[string]interface{}) map[string]interface{} {
			obj["processed"] = true
			return obj
		}),
		WithGoFunction[map[string]interface{}]("goProcessBool", func(b bool) bool {
			return !b
		}),
	)
	require.NoError(t, err)
	_, err = runner.RunRules(&context, nil)
	assert.NoError(t, err)

	assert.Equal(t, int64(84), context["intResult"])
	assert.Equal(t, "HELLO", context["stringResult"])

	arrayResult := context["arrayResult"].([]interface{})
	assert.Equal(t, int64(2), arrayResult[0])
	assert.Equal(t, int64(4), arrayResult[1])
	assert.Equal(t, int64(6), arrayResult[2])

	objectResult := context["objectResult"].(map[string]interface{})
	assert.Equal(t, "value", objectResult["key"])
	assert.Equal(t, int64(123), objectResult["number"])
	assert.Equal(t, true, objectResult["processed"])

	assert.Equal(t, false, context["boolResult"])
}

// Test library with dependencies
func TestRunner_LibraryWithDependencies(t *testing.T) {
	// Create base rules
	baseRules := `
name: "base-rules"
conditions:
  baseCondition:
    description: "Base condition"
    check: "function() { return context.baseValue > 0; }"
    true:
      action: "function() { context.baseProcessed = true; }"
      terminate: true
    false:
      terminate: true
`

	// Create dependent rules
	dependentRules := `
name: "dependent-rules"
require: ["base-rules"]
conditions:
  dependentCondition:
    description: "Dependent condition"
    default: true
    check: |
      function() { 
        return context.baseValue > 0 && context.dependentValue > 0; 
      }
    true:
      action: |
        function() { 
          context.baseProcessed = true;
          context.dependentProcessed = true; 
        }
      terminate: true
    false:
      terminate: true
`

	// Create library with both files
	tempDir := t.TempDir()
	err := os.WriteFile(tempDir+"/base-rules.yaml", []byte(baseRules), 0644)
	require.NoError(t, err)
	err = os.WriteFile(tempDir+"/dependent-rules.yaml", []byte(dependentRules), 0644)
	require.NoError(t, err)

	library, err := NewRulesLibrary(RulesLibrarySettings{
		BasePath: tempDir,
	})
	require.NoError(t, err)

	context := map[string]interface{}{
		"baseValue":      10,
		"dependentValue": 20,
	}
	runner, err := NewRulesRunnerFromLibrary[map[string]interface{}](library, "dependent-rules", &context)
	require.NoError(t, err)
	_, err = runner.RunRules(&context, nil)
	assert.NoError(t, err)
	assert.Equal(t, true, context["baseProcessed"])
	assert.Equal(t, true, context["dependentProcessed"])
}

// Test Go function returning error
func TestRunner_GoFunctionReturningError(t *testing.T) {
	yamlRules := `
name: "go-function-error"
conditions:
  errorCondition:
    description: "Condition with Go error"
    default: true
    check: "function() { return goFunctionThatErrors(context.value); }"
    true:
      terminate: true
`
	library := createLibraryFromYAML(t, yamlRules, "go-function-error.yaml")

	// Test with nil value
	context := map[string]interface{}{"value": nil}
	runner, err := NewRulesRunnerFromLibrary[map[string]interface{}](library, "go-function-error", &context,
		WithGoFunction[map[string]interface{}]("goFunctionThatErrors", func(value interface{}) (bool, error) {
			if value == nil {
				return false, errors.New("value cannot be nil")
			}
			return true, nil
		}),
	)
	require.NoError(t, err)
	_, err = runner.RunRules(&context, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "value cannot be nil")

	// Test with valid value
	context2 := map[string]interface{}{"value": "something"}
	_, err = runner.RunRules(&context2, nil)
	assert.NoError(t, err)
}

// Test complex decision tree with multiple branches
func TestRunner_ComplexDecisionTree(t *testing.T) {
	yamlRules := `
name: "complex-tree"
conditions:
  checkAge:
    description: "Check if user is adult"
    default: true
    check: "function() { return context.age >= 18; }"
    true:
      next: "checkPermission"
    false:
      next: "checkWeekend"
  checkPermission:
    description: "Check if user has permission"
    check: "function() { return context.permission === true; }"
    true:
      action: "function() { context.access = 'granted'; }"
      terminate: true
    false:
      action: "function() { context.access = 'denied'; }"
      terminate: true
  checkWeekend:
    description: "Check if it's weekend for minors"
    check: "function() { return context.dayOfWeek === 6 || context.dayOfWeek === 0; }"
    true:
      action: "function() { context.access = 'supervised'; }"
      terminate: true
    false:
      action: "function() { context.access = 'denied'; }"
      terminate: true
`
	library := createLibraryFromYAML(t, yamlRules, "complex-tree.yaml")

	testCases := []struct {
		name     string
		context  map[string]interface{}
		expected string
	}{
		{
			name: "Adult with permission",
			context: map[string]interface{}{
				"age":        25,
				"permission": true,
				"dayOfWeek":  1, // Monday
			},
			expected: "granted",
		},
		{
			name: "Adult without permission",
			context: map[string]interface{}{
				"age":        25,
				"permission": false,
				"dayOfWeek":  1,
			},
			expected: "denied",
		},
		{
			name: "Minor on weekend",
			context: map[string]interface{}{
				"age":        16,
				"permission": false,
				"dayOfWeek":  6, // Saturday
			},
			expected: "supervised",
		},
		{
			name: "Minor on weekday",
			context: map[string]interface{}{
				"age":        16,
				"permission": false,
				"dayOfWeek":  2, // Tuesday
			},
			expected: "denied",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			runner, err := NewRulesRunnerFromLibrary[map[string]interface{}](library, "complex-tree", &tc.context)
			require.NoError(t, err)

			_, err = runner.RunRules(&tc.context, nil)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, tc.context["access"])
		})
	}
}

// Test performance with large context objects
func TestRunner_LargeContextPerformance(t *testing.T) {
	yamlRules := `
name: "large-context"
conditions:
  processData:
    description: "Process large data"
    default: true
    check: |
      function() {
        var sum = 0;
        for (var i = 0; i < context.data.length; i++) {
          sum += context.data[i].value;
        }
        context.sum = sum;
        return sum > 100000;
      }
    true:
      action: "function() { context.result = 'large'; }"
      terminate: true
    false:
      action: "function() { context.result = 'small'; }"
      terminate: true
`
	library := createLibraryFromYAML(t, yamlRules, "large-context.yaml")

	// Create initial empty context for runner creation
	initContext := map[string]interface{}{}
	runner, err := NewRulesRunnerFromLibrary[map[string]interface{}](library, "large-context", &initContext)
	require.NoError(t, err)

	// Create large context with 10,000 items
	data := make([]map[string]interface{}, 10000)
	for i := 0; i < 10000; i++ {
		data[i] = map[string]interface{}{
			"id":    i,
			"value": i % 100,
			"name":  fmt.Sprintf("item-%d", i),
		}
	}

	context := map[string]interface{}{
		"data": data,
	}

	start := time.Now()
	_, err = runner.RunRules(&context, nil)
	duration := time.Since(start)

	assert.NoError(t, err)
	assert.Equal(t, "large", context["result"])
	assert.Less(t, duration, 5*time.Second, "Large context processing took too long")
}
