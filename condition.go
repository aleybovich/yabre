package yabre

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/dop251/goja"
)

// ConditionType defines the evaluation mode of a condition.
type ConditionType string

const (
	// ConditionTypeBool is the default binary (true/false) condition type.
	ConditionTypeBool ConditionType = "bool"
	// ConditionTypeSwitch is a multi-value branching condition whose check returns a string.
	ConditionTypeSwitch ConditionType = "switch"
)

type Condition struct {
	Default     bool                   `yaml:"default"`
	Name        string                 `yaml:"name"`
	Description string                 `yaml:"description"`
	Type        ConditionType          `yaml:"type"`
	Check       string                 `yaml:"check"`
	True        *Decision              `yaml:"true"`
	False       *Decision              `yaml:"false"`
	Cases       map[string]*SwitchCase `yaml:"cases"`
}

// IsBool returns true if the condition is a standard binary (true/false) condition.
// An empty Type is treated as bool (the default).
func (c *Condition) IsBool() bool {
	return c.Type == ConditionTypeBool || c.Type == ""
}

// IsSwitch returns true if the condition is a switch (multi-value branching) condition.
func (c *Condition) IsSwitch() bool {
	return c.Type == ConditionTypeSwitch
}

type Decision struct {
	Name        string `yaml:"-"`
	Description string `yaml:"description"`
	Action      string `yaml:"action"`
	Next        string `yaml:"next"`
	Terminate   bool   `yaml:"terminate"`
	Value       bool   `yaml:"-"`
}

// SwitchCase represents one branch of a switch condition.
type SwitchCase struct {
	Name        string `yaml:"-"` // conditionName_case_caseKey, set during UnmarshalYAML
	CaseKey     string `yaml:"-"` // the literal YAML map key, set during UnmarshalYAML
	Description string `yaml:"description"`
	Action      string `yaml:"action"`
	Next        string `yaml:"next"`
	Terminate   bool   `yaml:"terminate"`
}

func (sc *SwitchCase) UnmarshalYAML(unmarshal func(any) error) error {
	type switchCase SwitchCase
	var raw switchCase
	if err := unmarshal(&raw); err != nil {
		return err
	}
	if raw.Next != "" && raw.Terminate {
		return errors.New("next and terminate cannot be used together")
	}
	*sc = SwitchCase(raw)
	return nil
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

// switchCaseActionFuncName generates the deterministic JS variable name for a switch case's action function.
// The caseKey is sanitized to ensure it forms a valid JavaScript identifier.
func switchCaseActionFuncName(conditionName, caseKey string) string {
	return conditionName + "_case_" + sanitizeJSIdentifier(caseKey)
}

// sanitizeJSIdentifier replaces any characters that are not valid in a
// JavaScript identifier (letters, digits, underscore, $) with underscores.
func sanitizeJSIdentifier(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '$' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}

// switchCaseToDecision adapts a SwitchCase to a Decision so it can be passed to runAction.
func switchCaseToDecision(sc *SwitchCase) *Decision {
	return &Decision{
		Name:        sc.Name,
		Description: sc.Description,
		Action:      sc.Action,
		Next:        sc.Next,
		Terminate:   sc.Terminate,
	}
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

// runCondition runs the conditions recursively. tc may be nil (RunRules path); when
// non-nil a TraceEntry is appended for every condition that is evaluated.
func (runner *RulesRunner[Context]) runCondition(vm *goja.Runtime, rules *Rules, condition *Condition, tc *traceCollector) error {
	runner.decisionCallback("Evaluating condition: [%s] %s", condition.Name, condition.Description)

	// Reserve a slot in the trace BEFORE evaluating the check so that
	// entries remain in chronological order even after recursive calls.
	var slotIdx int
	var checkStart time.Time
	if tc != nil {
		slotIdx = len(tc.entries)
		tc.entries = append(tc.entries, TraceEntry{
			ConditionName: condition.Name,
			Description:   condition.Description,
		})
		checkStart = time.Now()
	}

	// Evaluate the check function.
	funcName := conditionCheckFuncName(condition.Name)
	checkFunc, ok := goja.AssertFunction(vm.Get(funcName))
	if !ok {
		err := fmt.Errorf("check function not found: %s", funcName)
		if tc != nil {
			tc.entries[slotIdx].Duration = time.Since(checkStart)
			tc.entries[slotIdx].Error = err
		}
		return err
	}
	checkResult, err := checkFunc(goja.Undefined())

	var checkDuration time.Duration
	if tc != nil {
		checkDuration = time.Since(checkStart)
	}

	if err != nil {
		err = fmt.Errorf("error evaluating check function %s: %w", funcName, err)
		if tc != nil {
			tc.entries[slotIdx].Duration = checkDuration
			tc.entries[slotIdx].Error = err
		}
		return err
	}

	// ── DISPATCH BY CONDITION TYPE ───────────────────────────────────────────
	switch {
	case condition.IsSwitch():
		return runner.runSwitchCondition(vm, rules, condition, checkResult, checkDuration, tc, slotIdx)

	case condition.IsBool():
		return runner.runBoolCondition(vm, rules, condition, checkResult, checkDuration, tc, slotIdx)

	default:
		return fmt.Errorf("unsupported condition type '%s' on condition '%s'", condition.Type, condition.Name)
	}
}

// runSwitchCondition handles the multi-value branching path after a check function returns a string key.
func (runner *RulesRunner[Context]) runSwitchCondition(vm *goja.Runtime, rules *Rules, condition *Condition, checkResult goja.Value, checkDuration time.Duration, tc *traceCollector, slotIdx int) error {
	caseKey := checkResult.String()
	runner.decisionCallback("Switch condition [%s] evaluated to [%s]", condition.Name, caseKey)

	sc := condition.Cases[caseKey]
	if sc == nil {
		runner.decisionCallback("No matching case [%s] for [%s], falling back to default", caseKey, condition.Name)
		sc = condition.Cases["default"]
	}

	if sc == nil {
		runner.decisionCallback("No default for [%s], terminating", condition.Name)
		if tc != nil {
			tc.entries[slotIdx].SwitchResult = caseKey
			tc.entries[slotIdx].Terminated = true
			tc.entries[slotIdx].Duration = checkDuration
		}
		return nil
	}

	decision := switchCaseToDecision(sc)
	if tc != nil {
		tc.entries[slotIdx].SwitchResult = caseKey
		tc.entries[slotIdx].HasAction = decision.Action != ""
		tc.entries[slotIdx].NextCondition = decision.Next
		tc.entries[slotIdx].Terminated = decision.Terminate
	}

	var actionDuration time.Duration
	var actionDurationPtr *time.Duration
	if tc != nil {
		actionDurationPtr = &actionDuration
	}

	err := runner.runAction(vm, rules, decision, tc, actionDurationPtr)
	if tc != nil {
		tc.entries[slotIdx].Duration = checkDuration + actionDuration
		tc.entries[slotIdx].Error = err
	}
	return err
}

// runBoolCondition handles the standard true/false branching path after a check function returns a boolean.
func (runner *RulesRunner[Context]) runBoolCondition(vm *goja.Runtime, rules *Rules, condition *Condition, checkResult goja.Value, checkDuration time.Duration, tc *traceCollector, slotIdx int) error {
	result := checkResult.ToBoolean()

	var decision *Decision
	if result {
		runner.decisionCallback("Condition [%s] evaluated to [true]", condition.Name)
		decision = condition.True
	} else {
		runner.decisionCallback("Condition [%s] evaluated to [false]", condition.Name)
		decision = condition.False
	}

	if decision == nil {
		runner.decisionCallback("No action or next condition defined, terminating")
		if tc != nil {
			tc.entries[slotIdx].Result = result
			tc.entries[slotIdx].Terminated = true
			tc.entries[slotIdx].Duration = checkDuration
		}
		return nil
	}

	// Pre-fill the trace fields that are known from the Decision struct before
	// running the action/next-condition chain. These are set regardless of
	// whether the action or downstream conditions later fail.
	if tc != nil {
		tc.entries[slotIdx].Result = result
		tc.entries[slotIdx].HasAction = decision.Action != ""
		tc.entries[slotIdx].NextCondition = decision.Next
		tc.entries[slotIdx].Terminated = decision.Terminate
	}

	// actionDuration captures only the wall-clock time of the direct action
	// for this condition (not the entire downstream chain).
	var actionDuration time.Duration
	var actionDurationPtr *time.Duration
	if tc != nil {
		actionDurationPtr = &actionDuration
	}

	err := runner.runAction(vm, rules, decision, tc, actionDurationPtr)

	if tc != nil {
		tc.entries[slotIdx].Duration = checkDuration + actionDuration
		tc.entries[slotIdx].Error = err
	}

	return err
}

// runAction runs the branch action (if any) and then recurses into the next condition.
// tc may be nil. actionDuration, when non-nil, receives the wall-clock time spent
// executing the direct action function only (not downstream conditions).
func (runner *RulesRunner[Context]) runAction(vm *goja.Runtime, rules *Rules, result *Decision, tc *traceCollector, actionDuration *time.Duration) error {
	if result.Action != "" {
		runner.decisionCallback("Running action: [%s] %s", result.Name, result.Description)
		actionFunc, ok := goja.AssertFunction(vm.Get(result.Name))
		if !ok {
			return fmt.Errorf("action function not found: %s", result.Name)
		}

		var actionStart time.Time
		if actionDuration != nil {
			actionStart = time.Now()
		}

		_, err := actionFunc(goja.Undefined())

		if actionDuration != nil {
			*actionDuration = time.Since(actionStart)
		}

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
		err = runner.runCondition(vm, rules, nextCondition, tc)
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
