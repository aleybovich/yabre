package yabre

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
		mermaid.WriteString(fmt.Sprintf("    %s{\"`%s`\"}\n", condition.Name, escape(ifEmpty(condition.Description, condition.Name))))
		if condition.True != nil {
			if condition.True.Action != "" {
				mermaid.WriteString(fmt.Sprintf("    %s_true[\"`%s`\"]\n", condition.Name, escape(ifEmpty(condition.True.Description, condition.Name+"_true"))))
			}

			if condition.True.Terminate {
				mermaid.WriteString(fmt.Sprintf("    %s_true_end((( )))\n", condition.Name))
			}
		}

		if condition.False != nil {
			if condition.False.Action != "" {
				mermaid.WriteString(fmt.Sprintf("    %s_false[\"%s\"]\n", condition.Name, escape(ifEmpty(condition.False.Description, condition.Name+"_false"))))
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
		renderDecision(condition, condition.True, mermaid)
	}
	if condition.False != nil {
		renderDecision(condition, condition.False, mermaid)
	}

	return nil
}

func renderDecision(
	condition *Condition,
	decision *Decision,
	mermaid *strings.Builder) error {

	if decision.Action != "" {
		// connection from condition to True/False action
		mermaid.WriteString(fmt.Sprintf("    %s --> |%t| %s\n", condition.Name, decision.Value, decision.Name))

		if decision.Next != "" {
			// connection from True/False action to next condition
			mermaid.WriteString(fmt.Sprintf("    %s --> %s\n", decision.Name, decision.Next))
		}

		if decision.Terminate {
			// terminator from True/False action
			mermaid.WriteString(fmt.Sprintf("    %s --> %s_end\n", decision.Name, decision.Name))
		}
	} else {
		if decision.Next != "" {
			// connection from condition to next condition
			mermaid.WriteString(fmt.Sprintf("    %s --> |%t| %s\n", condition.Name, decision.Value, decision.Next))
		}

		if decision.Terminate {
			// terminator from condition
			mermaid.WriteString(fmt.Sprintf("    %s --> |%t| %s_end\n", condition.Name, decision.Value, decision.Name))
		}
	}
	return nil
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
