package yabre

import (
	"fmt"

	"github.com/dop251/goja"
)

type RulesRunner[Context any] struct {
	Rules         *Rules
	Context       *Context
	debugCallback func(...any)
	goFunctions   map[string]func(...any) (any, error)
	// callback to be called when a decision is made
	decisionCallback func(msg string, args ...any)
}

type WithOption[Context any] func(*RulesRunner[Context]) error

// WithDebugCallback sets the DebugCallback option
func WithDebugCallback[Context any](callback func(...any)) WithOption[Context] {
	return func(runner *RulesRunner[Context]) error {
		runner.debugCallback = callback
		return nil
	}
}

func WithGoFunction[Context any](name string, f any) WithOption[Context] {
	return func(runner *RulesRunner[Context]) error {
		if runner.goFunctions == nil {
			runner.goFunctions = make(map[string]func(...any) (any, error))
		}

		var fn func(...any) (any, error)

		// if the function is NOT of expected signature `func(...any) (any, error)` then wrap it
		dontWrap, err := checkVariadicAnySignature(f)
		if err != nil {
			return fmt.Errorf("invalid go function signature: %w", err)
		} else if !dontWrap {
			fn = goFuncWrapper(f)
		} else {
			fn = f.(func(...any) (any, error))
		}

		runner.goFunctions[name] = fn
		return nil
	}
}

func WithDecisionCallback[Context any](callback func(msg string, args ...any)) WithOption[Context] {
	return func(runner *RulesRunner[Context]) error {
		runner.decisionCallback = callback
		return nil
	}
}

func NewRulesRunnerFromLibrary[Context any](
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
		decisionCallback: func(msg string, args ...any) {},
	}

	// Execute options
	for _, op := range options {
		if err := op(runner); err != nil {
			return nil, err
		}
	}

	return runner, nil
}

func (rr *RulesRunner[Context]) RunRules(context *Context, startCondition *Condition) (*Context, error) {
	ctx, _, err := rr.executeRules(context, startCondition, nil)
	return ctx, err
}

// RunRulesWithTrace executes the rule set identically to RunRules but also
// returns a []TraceEntry that records every condition evaluation in chronological
// order. Each entry captures the condition name, description, check result,
// whether an action ran, the next condition name, termination flag, the
// wall-clock duration for that condition's check + direct action, and any error.
//
// Because the traceCollector is allocated per call and never stored on the
// runner, concurrent calls are safe.
func (rr *RulesRunner[Context]) RunRulesWithTrace(context *Context, startCondition *Condition) (*Context, []TraceEntry, error) {
	tc := &traceCollector{}
	ctx, trace, err := rr.executeRules(context, startCondition, tc)
	return ctx, trace, err
}

// executeRules is the shared implementation for RunRules and RunRulesWithTrace.
// When tc is nil, no tracing overhead is incurred.
func (rr *RulesRunner[Context]) executeRules(context *Context, startCondition *Condition, tc *traceCollector) (*Context, []TraceEntry, error) {
	rules := rr.Rules
	vm := goja.New()

	// Add context to vm
	vm.Set("context", *context)

	// Add debug function to vm (always inject so JS code can call debug() without error)
	if rr.debugCallback != nil {
		vm.Set("debug", rr.debugCallback)
	} else {
		vm.Set("debug", func(...any) {})
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
		return nil, nil, err
	}

	if startCondition == nil {
		startCondition = rules.DefaultCondition
	}

	if startCondition == nil && rules.DefaultCondition == nil {
		return nil, nil, fmt.Errorf("no default condition found")
	}

	// Start running the conditions from the first condition
	err = rr.runCondition(vm, rules, startCondition, tc)

	// Get the updated context
	*context = vm.Get("context").ToObject(vm).Export().(Context)

	var entries []TraceEntry
	if tc != nil {
		entries = tc.entries
	}

	return context, entries, err
}
