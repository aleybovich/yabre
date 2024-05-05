package main

import (
	"fmt"
	"os"

	"github.com/dop251/goja"
	"gopkg.in/yaml.v2"
)

type Rules struct {
	Conditions       map[string]Condition `yaml:"conditions"`
	DefaultCondition *Condition           `yaml:"-"`
}

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
			condition.True.Designation = True
		}

		if condition.False != nil {
			condition.False.Name = condition.Name + "_false"
			condition.False.Designation = False
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

	// Now unmarshal the same yaml into an ordered list to get the first condition

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
