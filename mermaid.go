package main

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v2"
)

func ExportMermaid(yamlString []byte, defaultConditionName string) (string, error) {
	// Parse the YAML into a Rule struct
	var rules Rules
	err := yaml.Unmarshal(yamlString, &rules)
	if err != nil {
		return "", fmt.Errorf("error parsing YAML: %v", err)
	}

	var mermaid strings.Builder
	mermaid.WriteString("flowchart TD\n")
	mermaid.WriteString("    %% Definitions\n")

	// Declare all elements
	for _, condition := range rules.Conditions {
		mermaid.WriteString(fmt.Sprintf("    %s{\"`%s`\"}\n", condition.Name, condition.Description))
		if condition.True != nil {
			if condition.True.Action != "" {
				mermaid.WriteString(fmt.Sprintf("    %s_true[\"`%s`\"]\n", condition.Name, condition.True.Description))
			}

			if condition.True.Terminate {
				mermaid.WriteString(fmt.Sprintf("    %s_true_end((( )))\n", condition.Name))
			}
		}

		if condition.False != nil {
			if condition.False.Action != "" {
				mermaid.WriteString(fmt.Sprintf("    %s_false[\"%s\"]\n", condition.Name, condition.False.Description))
			}

			if condition.False.Terminate {
				mermaid.WriteString(fmt.Sprintf("    %s_false_end((( )))\n", condition.Name))
			}
		}
	}

	for _, condition := range rules.Conditions {
		renderCondition(&condition, &mermaid)
	}

	result := mermaid.String()
	return result, nil
}

func renderCondition(condition *Condition, mermaid *strings.Builder) error {
	if condition.True != nil {
		renderConditionResult(condition, condition.True, mermaid)
	}
	if condition.False != nil {
		renderConditionResult(condition, condition.False, mermaid)
	}

	return nil
}

func renderConditionResult(
	condition *Condition,
	conditionResult *ConditionResult,
	mermaid *strings.Builder) error {

	if conditionResult.Action != "" {
		// connection from condition to True/False action
		mermaid.WriteString(fmt.Sprintf("    %s --> |%s| %s\n", condition.Name, conditionResult.Designation, conditionResult.Name))

		if conditionResult.Next != "" {
			// connection from True/False action to next condition
			mermaid.WriteString(fmt.Sprintf("    %s --> %s\n", conditionResult.Name, conditionResult.Next))
		}

		if conditionResult.Terminate {
			// terminator from True/False action
			mermaid.WriteString(fmt.Sprintf("    %s --> %s_end\n", conditionResult.Name, conditionResult.Name))
		}
	} else {
		if conditionResult.Next != "" {
			// connection from condition to next condition
			mermaid.WriteString(fmt.Sprintf("    %s --> |%s| %s\n", condition.Name, conditionResult.Designation, conditionResult.Next))
		}

		if conditionResult.Terminate {
			// terminator from condition
			mermaid.WriteString(fmt.Sprintf("    %s --> |%s| %s_end\n", condition.Name, conditionResult.Designation, conditionResult.Name))
		}
	}
	return nil
}
