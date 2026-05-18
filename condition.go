package yabre

import (
	"errors"
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

// conditionCheckFuncName generates the deterministic JS variable name for a condition's check function.
func conditionCheckFuncName(conditionName string) string {
	return conditionName + "_check"
}

// decisionActionFuncName generates the deterministic JS variable name for a decision's action function.
func decisionActionFuncName(conditionName string, branch bool) string {
	if branch {
		return conditionName + "_true"
	}
	return conditionName + "_false"
}

func (cr *Decision) UnmarshalYAML(unmarshal func(any) error) error {
	type decision Decision // we need to create an intermediate type to avoid infinite recursion
	var dsn decision
	if err := unmarshal(&dsn); err != nil {
		return err
	}

	if dsn.Next != "" && dsn.Terminate {
		return errors.New("next and terminate cannot be used together")
	}

	*cr = Decision(dsn)
	return nil
}

// Run the conditions recursively
func (runner *RulesRunner[Context]) runCondition(vm *goja.Runtime, rules *Rules, condition *Condition) error {
	runner.decisionCallback("Evaluating condition: [%s] %s", condition.Name, condition.Description)

	// Evaluate the check function
	funcName := conditionCheckFuncName(condition.Name)
	checkFunc, ok := goja.AssertFunction(vm.Get(funcName))
	if !ok {
		return fmt.Errorf("check function not found: %s", funcName)
	}
	checkResult, err := checkFunc(goja.Undefined())
	if err != nil {
		return fmt.Errorf("error evaluating check function %s: %w", funcName, err)
	}

	if checkResult.ToBoolean() {
		runner.decisionCallback("Condition [%s] evaluated to [true]", condition.Name)
		if condition.True == nil {
			runner.decisionCallback("No action or next condition defined, terminating")
			return nil
		}
		return runner.runAction(vm, rules, condition.True)
	} else {
		runner.decisionCallback("Condition [%s] evaluated to [false]", condition.Name)
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
		runner.decisionCallback("Running action: [%s] %s", result.Name, result.Description)
		actionFunc, ok := goja.AssertFunction(vm.Get(result.Name))
		if !ok {
			return fmt.Errorf("action function not found: %s", result.Name)
		}
		_, err := actionFunc(goja.Undefined())
		if err != nil {
			return fmt.Errorf("error running action: %w", err)
		}
	}

	if result.Next != "" {
		nextCondition, err := findConditionByName(rules, result.Next)
		if err != nil {
			return fmt.Errorf("unexpected error: condition '%s' not found", result.Next)
		}
		runner.decisionCallback("Moving to next condition:[%s]", nextCondition.Name)
		err = runner.runCondition(vm, rules, nextCondition)
		if err != nil {
			return fmt.Errorf("error while evaluating condition '%s': %w", result.Next, err)
		}
	}

	if result.Terminate {
		runner.decisionCallback("Terminating")
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
