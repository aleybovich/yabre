package main

import (
	"fmt"

	"github.com/dop251/goja"
)

type RulesRunner[Context interface{}] struct {
	Rules            *Rules
	Context          *Context
	DebugCallback    func(Context, interface{})
	GoFunctions      map[string]func(...interface{}) (interface{}, error)
	decisionCallback func(msg string, args ...interface{})
}

type WithOption[Context interface{}] func(*RulesRunner[Context])

// WithDebugCallback sets the DebugCallback option
func WithDebugCallback[Context interface{}](callback func(Context, interface{})) WithOption[Context] {
	return func(runner *RulesRunner[Context]) {
		runner.DebugCallback = callback
	}
}

func WithGoFunction[Context interface{}](name string, f func(...interface{}) (interface{}, error)) WithOption[Context] {
	return func(runner *RulesRunner[Context]) {
		if runner.GoFunctions == nil {
			runner.GoFunctions = make(map[string]func(...interface{}) (interface{}, error))
		}
		runner.GoFunctions[name] = f
	}
}

func WithDecisionCallback[Context interface{}](callback func(msg string, args ...interface{})) WithOption[Context] {
	return func(runner *RulesRunner[Context]) {
		runner.decisionCallback = callback
	}
}

func (runner *RulesRunner[Context]) DecisionCallback(msg string, args ...interface{}) {
	if runner.decisionCallback != nil {
		runner.decisionCallback(msg, args...)
	}
}

func NewRulesRunnerFromYaml[Context interface{}](fileName string, context *Context, options ...WithOption[Context]) (*RulesRunner[Context], error) {
	rr := &RulesRunner[Context]{Context: context}

	// Execute options
	for _, op := range options {
		op(rr)
	}

	// Load the rules from the YAML file
	rules, err := rr.loadRulesFromYaml(fileName)
	if err != nil {
		return nil, err
	}
	rr.Rules = rules

	return rr, nil
}

func (rr *RulesRunner[Context]) RunRules(context *Context, startCondition *Condition) (*Context, error) {
	rules := rr.Rules
	vm := goja.New()

	// Add context to vm
	vm.Set("context", *context)

	// Add debug function to vm
	if rr.DebugCallback != nil {
		vm.Set("debug", rr.DebugCallback)
	}

	// Add go functions to vm
	if rr.GoFunctions != nil {
		for name, f := range rr.GoFunctions {
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
