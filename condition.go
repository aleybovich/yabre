package main

import (
	"fmt"

	"github.com/dop251/goja"
)

type Designation string

const (
	True  Designation = "true"
	False Designation = "false"
)

type Condition struct {
	Default     bool             `yaml:"default"`
	Name        string           `yaml:"name"`
	Description string           `yaml:"description"`
	Check       string           `yaml:"check"`
	True        *ConditionResult `yaml:"true"`
	False       *ConditionResult `yaml:"false"`
}

type ConditionResult struct {
	Name        string      `yaml:"-"`
	Description string      `yaml:"description"`
	Action      string      `yaml:"action"`
	Next        string      `yaml:"next"`
	Terminate   bool        `yaml:"terminate"`
	Designation Designation `yaml:"-"` // true or false
}

func (cr *ConditionResult) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type conditionResult ConditionResult // we need to create an intermediate type to avoid infinite recursion
	var ccr conditionResult
	if err := unmarshal(&ccr); err != nil {
		return err
	}

	if ccr.Next != "" && ccr.Terminate {
		return fmt.Errorf("next and terminate cannot be used together")
	}

	*cr = ConditionResult(ccr)
	return nil
}

// Run the conditions recursively
func (runner *RulesRunner[Context]) runCondition(vm *goja.Runtime, rules *Rules, condition *Condition) error {
	runner.DecisionCallback("Running condition: %s\n", condition.Name)

	// Evaluate the check function
	checkFunc, ok := goja.AssertFunction(vm.Get(condition.Name))
	if !ok {
		return fmt.Errorf("check function not found: %s", condition.Name)
	}
	checkResult, err := checkFunc(goja.Undefined())
	if err != nil {
		return fmt.Errorf("error evaluating check function %s: %v", condition.Name, err)
	}

	if checkResult.ToBoolean() { // If check was true
		if condition.True != nil {
			runner.DecisionCallback("\tRunning TRUE check: %s\n", condition.True.Description)
			if condition.True.Action != "" {
				// Run the action function
				actionFuncName := condition.Name + "_true"
				runner.DecisionCallback("\t\tRunning action function: %s\n", actionFuncName)
				actionFunc, ok := goja.AssertFunction(vm.Get(actionFuncName))
				if !ok {
					return fmt.Errorf("action function not found: %s", actionFuncName)
				}
				_, err := actionFunc(goja.Undefined())
				if err != nil {
					return fmt.Errorf("error running action function: %v", err)
				}
			}
			if condition.True.Next != "" {
				nextCondition, err := findConditionByName(rules, condition.True.Next)
				if err != nil {
					return fmt.Errorf("unexpected error: condition '%s' not found", condition.True.Next)
				}
				err = runner.runCondition(vm, rules, nextCondition)
				if err != nil {
					return fmt.Errorf("error while running condition '%s': %v", condition.True.Next, err)
				}
			}
			if condition.True.Terminate {
				runner.DecisionCallback("Terminating\n")
				return nil
			}
		}
	} else { // if check was false
		if condition.False != nil {
			runner.DecisionCallback("\tRunning FALSE check: %s\n", condition.False.Description)
			if condition.False.Action != "" {
				// Run the action function
				actionFuncName := condition.Name + "_false"
				runner.DecisionCallback("\t\tRunning action function: %s\n", actionFuncName)
				actionFunc, ok := goja.AssertFunction(vm.Get(actionFuncName))
				if !ok {
					return fmt.Errorf("action function not found: %s", actionFuncName)
				}
				_, err := actionFunc(goja.Undefined())
				if err != nil {
					return fmt.Errorf("error running action function: %v", err)
				}
			}
			if condition.False.Next != "" {
				nextCondition, err := findConditionByName(rules, condition.False.Next)
				if err != nil {
					return fmt.Errorf("unexpected error: condition '%s' not found", condition.False.Next)
				}

				err = runner.runCondition(vm, rules, nextCondition)
				if err != nil {
					return fmt.Errorf("error while running condition '%s': %v", condition.False.Next, err)
				}
			}
			if condition.False.Terminate {
				runner.DecisionCallback("Terminating\n")
				return nil
			}
		}
	}

	return nil // return from runCondition
}

func findConditionByName(rule *Rules, name string) (*Condition, error) {
	if condition, ok := rule.Conditions[name]; ok {
		return &condition, nil
	}

	return nil, fmt.Errorf("Condition not found: %s", name)
}
