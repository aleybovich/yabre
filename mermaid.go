package yabre

import (
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v2"
)

func ExportMermaidFromLibrary(library *RulesLibrary, ruleName string, defaultConditionName string) (string, error) {
	// Load the complete rule set with all dependencies resolved
	rules, err := library.LoadRules(ruleName)
	if err != nil {
		return "", fmt.Errorf("failed to load rules: %w", err)
	}

	yamlData, err := yaml.Marshal(rules)
	if err != nil {
		return "", fmt.Errorf("failed to marshal rules: %w", err)
	}

	return ExportMermaid(yamlData, defaultConditionName)
}

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
		declareCondition(&condition, &mermaid)
	}

	for _, condition := range rules.Conditions {
		renderCondition(rules.Conditions, &condition, &mermaid)
	}

	result := mermaid.String()
	return result, nil
}

func declareCondition(condition *Condition, mermaid *strings.Builder) {
	mermaid.WriteString(fmt.Sprintf("    %s{\"`%s`\"}\n", condition.Name, escape(ifEmpty(condition.Description, condition.Name))))

	conditionName := condition.Name

	decisions := []*Decision{condition.True, condition.False}
	for _, decision := range decisions {
		if decision != nil {
			decisionValue := strconv.FormatBool(decision.Value)
			if decision.Action != "" {
				mermaid.WriteString(fmt.Sprintf("    %s_%s[\"%s\"]\n", conditionName, decisionValue, escape(ifEmpty(decision.Description, conditionName+"_"+decisionValue))))
			}

			if decision.Terminate {
				mermaid.WriteString(fmt.Sprintf("    %s_%s_end((( )))\n", conditionName, decisionValue))
			}
		}
	}
}

func renderCondition(conditions map[string]Condition, condition *Condition, mermaid *strings.Builder) {
	decisions := []*Decision{condition.True, condition.False}
	for _, decision := range decisions {
		if decision != nil {
			renderDecision(conditions, condition, decision, mermaid)
		}
	}
}

func renderDecision(
	conditions map[string]Condition,
	condition *Condition,
	decision *Decision,
	mermaid *strings.Builder) {

	optionalDescription := ""
	// Check whether the condition, referenced in Next, is defined in this ruleset or is external
	// External conditions are not declared in `declareCondition` and thus don't show descriptions by default
	// For such conditions, we need to add the description to the block explicitly
	if decision.Next != "" && decision.Description != "" {
		_, isInternal := conditions[decision.Next]
		if !isInternal {
			optionalDescription = fmt.Sprintf("[%s]", decision.Description)
		}
	}

	if decision.Action != "" {
		// connection from condition to True/False action
		mermaid.WriteString(fmt.Sprintf("    %s --> |%t| %s\n", condition.Name, decision.Value, decision.Name))

		if decision.Next != "" {
			// connection from True/False action to next condition
			mermaid.WriteString(fmt.Sprintf("    %s --> %s%s\n", decision.Name, decision.Next, optionalDescription))
		}

		if decision.Terminate {
			// terminator from True/False action
			mermaid.WriteString(fmt.Sprintf("    %s --> %s_end\n", decision.Name, decision.Name))
		}
	} else {
		if decision.Next != "" {
			// connection from condition to next condition
			mermaid.WriteString(fmt.Sprintf("    %s --> |%t| %s%s\n", condition.Name, decision.Value, decision.Next, optionalDescription))
		}

		if decision.Terminate {
			// terminator from condition
			mermaid.WriteString(fmt.Sprintf("    %s --> |%t| %s_end\n", condition.Name, decision.Value, decision.Name))
		}
	}
}

func ifEmpty(first, second string) string {
	if first == "" {
		return second
	}
	return first
}

func escape(s string) string {
	return strings.ReplaceAll(s, "\"", "&quot")
}
