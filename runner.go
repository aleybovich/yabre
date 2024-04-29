package main

type RulesRunner[Context interface{}] struct {
	Rules         *Rules
	Context       *Context
	DebugCallback func(Context, interface{})
	GoFunctions   map[string]func(...interface{}) (interface{}, error)
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
