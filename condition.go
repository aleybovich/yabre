package yabre

import (
	"fmt"

	"github.com/dop251/goja"
)

type Condition struct {
	Default     bool      `yaml:"default"`
	Name        string    `yaml:"name"`
	Description string    `yaml:"description"`
	Check       string    `yaml:"check"`
	True        *Decision `yaml:"true"`
	False       *Decision `yaml:"false"`
}

type Decision struct {
	Name        string `yaml:"-"`
	Description string `yaml:"description"`
	Action      string `yaml:"action"`
	Next        string `yaml:"next"`
	Terminate   bool   `yaml:"terminate"`
	Value       bool   `yaml:"-"`
}

func (cr *Decision) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type decision Decision // we need to create an intermediate type to avoid infinite recursion
	var dsn decision
	if err := unmarshal(&dsn); err != nil {
		return err
	}

	if dsn.Next != "" && dsn.Terminate {
		return fmt.Errorf("next and terminate cannot be used together")
	}

	*cr = Decision(dsn)
	return nil
}

// Run the conditions recursively
func (runner *RulesRunner[Context]) runCondition(vm *goja.Runtime, rules *Rules, condition *Condition) error {
	runner.decisionCallback("Evaluating condition: [%s] %s\n", condition.Name, condition.Description)

	// Get the custom function name for the check function
	checkFuncName := runner.getFunctionName(condition.Name)

	// Evaluate the check function
	checkFunc, ok := goja.AssertFunction(vm.Get(checkFuncName))
	if !ok {
		return fmt.Errorf("check function not found: %s", checkFuncName)
	}
	checkResult, err := checkFunc(goja.Undefined())
	if err != nil {
		return fmt.Errorf("error evaluating check function %s: %v", checkFuncName, err)
	}

	if checkResult.ToBoolean() {
		runner.decisionCallback("Condition [%s] evaluated to [true]\n", condition.Name)
		if condition.True == nil {
			runner.decisionCallback("No action or next condition defined, terminating")
			return nil
		}
		return runner.runAction(vm, rules, condition.True)
	} else {
		runner.decisionCallback("Condition [%s] evaluated to [false]\n", condition.Name)
		if condition.False == nil {
			runner.decisionCallback("No action or next condition defined, terminating")
			return nil
		}
		return runner.runAction(vm, rules, condition.False)
	}
}

// Helper function to run the action
func (runner *RulesRunner[Context]) runAction(vm *goja.Runtime, rules *Rules, result *Decision) error {
	if result.Action != "" {
		actionFuncName := runner.getFunctionName(result.Name)
		runner.decisionCallback("Running action: [%s] %s\n", actionFuncName, result.Description)
		actionFunc, ok := goja.AssertFunction(vm.Get(actionFuncName))
		if !ok {
			return fmt.Errorf("action function not found: %s", actionFuncName)
		}
		_, err := actionFunc(goja.Undefined())
		if err != nil {
			return fmt.Errorf("error running action: %v", err)
		}
	}

	if result.Next != "" {
		nextCondition, err := findConditionByName(rules, result.Next)
		if err != nil {
			return fmt.Errorf("unexpected error: condition '%s' not found", result.Next)
		}
		runner.decisionCallback("Moving to next condition:[%s]\n", nextCondition.Name)
		err = runner.runCondition(vm, rules, nextCondition)
		if err != nil {
			return fmt.Errorf("error while evaluating condition '%s': %v", result.Next, err)
		}
	}

	if result.Terminate {
		runner.decisionCallback("Terminating\n")
		return nil
	}

	return nil
}

func findConditionByName(rule *Rules, name string) (*Condition, error) {
	if condition, ok := rule.Conditions[name]; ok {
		return &condition, nil
	}

	return nil, fmt.Errorf("Condition not found: %s", name)
}
