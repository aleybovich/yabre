package main

import (
	"fmt"

	"github.com/dop251/goja"
)

type Condition struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Check       string `yaml:"check"`
	True        *struct {
		Description string `yaml:"description"`
		Action      string `yaml:"action"`
		Next        string `yaml:"next"`
		Terminate   bool   `yaml:"terminate"`
	} `yaml:"true"`
	False *struct {
		Description string `yaml:"description"`
		Action      string `yaml:"action"`
		Next        string `yaml:"next"`
		Terminate   bool   `yaml:"terminate"`
	} `yaml:"false"`
}

// Run the conditions recursively
func (rr *RulesRunner[Context]) runCondition(vm *goja.Runtime, rules *Rules, condition *Condition) error {
	fmt.Print("Running condition: ", condition.Name, "\n")

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
			fmt.Print("\tRunning TRUE check: ", condition.True.Description, "\n")
			if condition.True.Action != "" {
				// Run the action function
				actionFuncName := condition.Name + "_true"
				fmt.Print("\t\tRunning action function: ", actionFuncName, "\n")
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
				err = rr.runCondition(vm, rules, nextCondition)
				if err != nil {
					return fmt.Errorf("error while running condition '%s': %v", condition.True.Next, err)
				}
			}
			if condition.True.Terminate {
				fmt.Print("Terminating\n")
				return nil
			}
		}
	} else { // if check was false
		if condition.False != nil {
			fmt.Print("\tRunning FALSE check: ", condition.False.Description, "\n")
			if condition.False.Action != "" {
				// Run the action function
				actionFuncName := condition.Name + "_false"
				fmt.Print("\t\tRunning action function: ", actionFuncName, "\n")
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

				err = rr.runCondition(vm, rules, nextCondition)
				if err != nil {
					return fmt.Errorf("error while running condition '%s': %v", condition.False.Next, err)
				}
			}
			if condition.False.Terminate {
				fmt.Print("Terminating\n")
				return nil
			}
		}
	}

	return nil // return from runCondition
}

func findConditionByName(rule *Rules, name string) (*Condition, error) {
	for _, condition := range rule.Conditions {
		if condition.Name == name {
			return &condition, nil
		}
	}
	return nil, fmt.Errorf("Condition not found: %s", name)
}
