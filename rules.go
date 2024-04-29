package main

import (
	"fmt"
	"os"

	"github.com/dop251/goja"
	"gopkg.in/yaml.v2"
)

type Rules struct {
	Conditions map[string]Condition `yaml:"conditions"`
}

func (rr *RulesRunner[Context]) RunRules(context *Context, startCondition string) (*Context, error) {
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

	// Start running the conditions from the first condition
	condition, err := findConditionByName(rules, startCondition)
	if err != nil {
		return nil, err
	}

	// Start running the conditions from the first condition
	err = rr.runCondition(vm, rules, condition)

	// Get the updated context
	*context = vm.Get("context").ToObject(vm).Export().(Context)

	return context, err
}

func (rr *RulesRunner[Context]) loadRulesFromYaml(fileName string) (*Rules, error) {
	yamlFile, err := os.ReadFile(fileName)
	if err != nil {
		return nil, fmt.Errorf("error reading YAML file: %v", err)
	}

	// Parse the YAML into a Rule struct
	var rules Rules
	err = yaml.Unmarshal(yamlFile, &rules)
	if err != nil {
		return nil, fmt.Errorf("error parsing YAML: %v", err)
	}

	// add names to conditions from the Rule map
	for name, condition := range rules.Conditions {
		condition.Name = name
		rules.Conditions[name] = condition
	}

	return &rules, nil
}

func (rr *RulesRunner[Context]) addJsFunctions(vm *goja.Runtime) error {
	// add all js functions to the vm
	for _, condition := range rr.Rules.Conditions {
		if condition.Check != "" {
			_, err := vm.RunString(condition.Check)
			if err != nil {
				return fmt.Errorf("error injecting check function into vm: %v", err)
			}
		}
		if condition.True != nil && condition.True.Action != "" {
			_, err := vm.RunString(condition.True.Action)
			if err != nil {
				return fmt.Errorf("error injecting action function into vm: %v", err)
			}
		}
		if condition.False != nil && condition.False.Action != "" {
			_, err := vm.RunString(condition.False.Action)
			if err != nil {
				return fmt.Errorf("error injecting action function into vm: %v", err)
			}
		}
	}

	return nil
}
