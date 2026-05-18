package yabre

import (
	"errors"
	"fmt"
	"maps"

	"github.com/dop251/goja"
)

type Rules struct {
	Name             string               `yaml:"name"`
	Require          []string             `yaml:"require,omitempty"`
	Scripts          string               `yaml:"scripts"`
	Conditions       map[string]Condition `yaml:"conditions"`
	DefaultCondition *Condition           `yaml:"-"`
}

// Perform enrichment and validation of rules data during unmarshalling
func (r *Rules) UnmarshalYAML(unmarshal func(any) error) error {
	type rules Rules // we need to create an intermediate type to avoid infinite recursion
	var rr rules
	if err := unmarshal(&rr); err != nil {
		return err
	}

	defaultFound := false

	// add names to conditions
	// and find the default condition
	for name, condition := range rr.Conditions {
		condition.Name = name

		if condition.True != nil {
			condition.True.Name = decisionActionFuncName(condition.Name, true)
			condition.True.Value = true
		}

		if condition.False != nil {
			condition.False.Name = decisionActionFuncName(condition.Name, false)
			condition.False.Value = false
		}

		rr.Conditions[name] = condition

		if condition.Default {
			if !defaultFound {
				rr.DefaultCondition = &condition
				defaultFound = true
			} else {
				return errors.New("multiple default conditions found")
			}
		}
	}

	*r = Rules(rr)
	return nil
}

// copy returns a shallow copy of the Rules struct with a new Conditions map.
// This allows merging without mutating cached rule sets.
func (r *Rules) copy() *Rules {
	conditions := make(map[string]Condition, len(r.Conditions))
	maps.Copy(conditions, r.Conditions)

	return &Rules{
		Name:             r.Name,
		Require:          r.Require,
		Scripts:          r.Scripts,
		Conditions:       conditions,
		DefaultCondition: r.DefaultCondition,
	}
}

func (runner *RulesRunner[Context]) addJsFunctions(vm *goja.Runtime) error {
	// add all js functions to the vm
	if runner.Rules.Scripts != "" {
		_, err := vm.RunString(runner.Rules.Scripts)
		if err != nil {
			return fmt.Errorf("error injecting scripts into vm: %w", err)
		}
	}

	for _, condition := range runner.Rules.Conditions {
		if condition.Check != "" {
			if err := injectJSFunction(vm, conditionCheckFuncName(condition.Name), condition.Check); err != nil {
				return fmt.Errorf("error injecting condition function into vm: %w", err)
			}
		}
		if condition.True != nil && condition.True.Action != "" {
			if err := injectJSFunction(vm, condition.True.Name, condition.True.Action); err != nil {
				return fmt.Errorf("error injecting action function into vm: %w", err)
			}
		}
		if condition.False != nil && condition.False.Action != "" {
			if err := injectJSFunction(vm, condition.False.Name, condition.False.Action); err != nil {
				return fmt.Errorf("error injecting action function into vm: %w", err)
			}
		}
	}

	return nil
}

func injectJSFunction(vm *goja.Runtime, varName, funcCode string) error {
	_, err := vm.RunString(fmt.Sprintf("var %s = %s", varName, funcCode))
	if err != nil {
		return fmt.Errorf("error injecting function %s into vm: %w", varName, err)
	}
	return nil
}
