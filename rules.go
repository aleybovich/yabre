package yabre

import (
	"fmt"
	"regexp"

	"github.com/dop251/goja"
	"gopkg.in/yaml.v2"
)

type Rules struct {
	Conditions       map[string]Condition `yaml:"conditions"`
	DefaultCondition *Condition           `yaml:"-"`
}

// Perform enrichment and validation of rules data during unmarshalling
func (r *Rules) UnmarshalYAML(unmarshal func(interface{}) error) error {
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
			condition.True.Name = condition.Name + "_true"
			condition.True.Value = true
		}

		if condition.False != nil {
			condition.False.Name = condition.Name + "_false"
			condition.False.Value = false
		}

		rr.Conditions[name] = condition

		if condition.Default {
			if !defaultFound {
				rr.DefaultCondition = &condition
				defaultFound = true
			} else {
				return fmt.Errorf("multiple default conditions found")
			}
		}
	}

	*r = Rules(rr)
	return nil
}

func (rr *RulesRunner[Context]) loadRulesFromYaml(yamlFile []byte) (*Rules, error) {
	// Parse the YAML into a Rule struct
	var rules Rules
	err := yaml.Unmarshal(yamlFile, &rules)
	if err != nil {
		return nil, fmt.Errorf("error parsing YAML: %v", err)
	}

	// Now unmarshal the same yaml into an ordered list to get the first condition

	return &rules, nil
}

func (runner *RulesRunner[Context]) addJsFunctions(vm *goja.Runtime) error {
	// add all js functions to the vm
	for _, condition := range runner.Rules.Conditions {
		if condition.Check != "" {
			checkName := condition.Name
			if err := runner.injectJSFunction(vm, checkName, condition.Check); err != nil {
				return fmt.Errorf("error injecting condition function into vm: %v", err)
			}
		}
		if condition.True != nil && condition.True.Action != "" {
			actionName := fmt.Sprintf("%s_%t", condition.Name, condition.True.Value)
			if err := runner.injectJSFunction(vm, actionName, condition.True.Action); err != nil {
				return fmt.Errorf("error injecting action function into vm: %v", err)
			}
		}
		if condition.False != nil && condition.False.Action != "" {
			actionName := fmt.Sprintf("%s_%t", condition.Name, condition.False.Value)
			if err := runner.injectJSFunction(vm, actionName, condition.True.Action); err != nil {
				return fmt.Errorf("error injecting action function into vm: %v", err)
			}
		}
	}

	return nil
}

func (runner *RulesRunner[Context]) injectJSFunction(vm *goja.Runtime, defaultName, funcCode string) error {
	funcName := defaultName

	re := regexp.MustCompile(`function\s+(\w+)\s*\(`)
	matches := re.FindStringSubmatch(funcCode)
	if len(matches) > 1 {
		funcName = matches[1]
	}

	runner.functionNames[defaultName] = funcName // Store the function name mapping
	_, err := vm.RunString(fmt.Sprintf("%s = %s", funcName, funcCode))
	if err != nil {
		return fmt.Errorf("error injecting function %s into vm: %v", funcName, err)
	}

	return nil
}
