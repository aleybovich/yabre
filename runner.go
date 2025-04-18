package yabre

import (
	"fmt"

	"github.com/dop251/goja"
)

type RulesRunner[Context interface{}] struct {
	Rules         *Rules
	Context       *Context
	debugCallback func(...interface{})
	goFunctions   map[string]func(...interface{}) (interface{}, error)
	// callback to be called when a decision is made
	decisionCallback func(msg string, args ...interface{})
	// mapping of js functions in business rules to standard names
	functionNames map[string]string
}

type WithOption[Context interface{}] func(*RulesRunner[Context]) error

// WithDebugCallback sets the DebugCallback option
func WithDebugCallback[Context interface{}](callback func(...interface{})) WithOption[Context] {
	return func(runner *RulesRunner[Context]) error {
		runner.debugCallback = callback
		return nil
	}
}

func WithGoFunction[Context interface{}](name string, f any) WithOption[Context] {
	return func(runner *RulesRunner[Context]) error {
		if runner.goFunctions == nil {
			runner.goFunctions = make(map[string]func(...interface{}) (interface{}, error))
		}

		var fn func(...interface{}) (interface{}, error)

		// if the function is NOT of expected signature `func(...interface{}) (interface{}, error)` then wrap it
		dontWrap, err := checkVariadicAnySignature(f)
		if err != nil {
			return fmt.Errorf("invalid go function signature: %w", err)
		} else if !dontWrap {
			fn = goFuncWrapper(f)
		} else {
			fn = f.(func(...interface{}) (interface{}, error))
		}

		runner.goFunctions[name] = fn
		return nil
	}
}

func WithDecisionCallback[Context interface{}](callback func(msg string, args ...interface{})) WithOption[Context] {
	return func(runner *RulesRunner[Context]) error {
		runner.decisionCallback = callback
		return nil
	}
}

func (runner *RulesRunner[Context]) getFunctionName(name string) string {
	if functionName, ok := runner.functionNames[name]; ok {
		return functionName
	}
	return name
}

func NewRulesRunnerFromLibrary[Context interface{}](
	library *RulesLibrary,
	rulesName string,
	context *Context,
	options ...WithOption[Context],
) (*RulesRunner[Context], error) {
	// Load rules and their dependencies from library
	rules, err := library.LoadRules(rulesName)
	if err != nil {
		return nil, fmt.Errorf("failed to load rules: %w", err)
	}

	runner := &RulesRunner[Context]{
		Context:          context,
		Rules:            rules,
		functionNames:    map[string]string{},
		decisionCallback: func(msg string, args ...interface{}) {},
	}

	// Execute options
	for _, op := range options {
		if err := op(runner); err != nil {
			return nil, err
		}
	}

	return runner, nil
}

// Deprecated: Use NewRulesRunnerFromLibrary instead
func NewRulesRunnerFromYaml[Context interface{}](yamlData []byte, context *Context, options ...WithOption[Context]) (*RulesRunner[Context], error) {
	runner := &RulesRunner[Context]{
		Context:          context,
		functionNames:    map[string]string{},
		decisionCallback: func(msg string, args ...interface{}) {},
	}

	// Execute options
	for _, op := range options {
		err := op(runner)
		if err != nil {
			return nil, err
		}
	}

	// Load the rules from the YAML data
	rules, err := runner.loadRulesFromYaml(yamlData)
	if err != nil {
		return nil, err
	}
	runner.Rules = rules

	return runner, nil
}

func (rr *RulesRunner[Context]) RunRules(context *Context, startCondition *Condition) (*Context, error) {
	rules := rr.Rules
	vm := goja.New()

	// Add context to vm
	vm.Set("context", *context)

	// Add debug function to vm
	if rr.debugCallback != nil {
		vm.Set("debug", rr.debugCallback)
	}

	// Add go functions to vm
	if rr.goFunctions != nil {
		for name, f := range rr.goFunctions {
			vm.Set(name, f)
		}
	}

	// Add all js functions to the vm
	err := rr.addJsFunctions(vm)
	if err != nil {
		return nil, err
	}

	if startCondition == nil {
		startCondition = rules.DefaultCondition
	}

	if startCondition == nil && rules.DefaultCondition == nil {
		return nil, fmt.Errorf("no default condition found")
	}

	// Start running the conditions from the first condition
	err = rr.runCondition(vm, rules, startCondition)

	// Get the updated context
	*context = vm.Get("context").ToObject(vm).Export().(Context)

	return context, err
}
