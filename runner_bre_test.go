package yabre

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

type BreContext struct {
	RuleSet      string `json:"rule_set"`
	TextRuleSet1 string `json:"text_rule_set_1"`
	TextRuleSet2 string `json:"text_rule_set_2"`
	TextRuleSet3 string `json:"text_rule_set_3"`
}

func TestRunnerBre(t *testing.T) {
	breContext := &BreContext{RuleSet: "ruleset1"}
	var debugMessage string

	ruleLibrary, err := NewRulesLibrary(RulesLibrarySettings{BasePath: "./test/bre"})
	assert.NoError(t, err)

	runner, err := NewRulesRunnerFromLibrary(ruleLibrary, "main", breContext,
		WithDebugCallback[BreContext](
			func(data ...any) {
				if len(data) > 0 {
					debugMessage = fmt.Sprintf("%v", data[0])
				}
			}),

		WithDecisionCallback[BreContext](func(msg string, args ...any) {
			fmt.Printf("    "+msg+"\n", args...)
		}))
	assert.NoError(t, err)

	_, err = runner.RunRules(breContext, nil)
	assert.NoError(t, err)
	assert.Equal(t, "RuleSet1 executed", debugMessage)

	breContext.RuleSet = "ruleset2"
	_, err = runner.RunRules(breContext, nil)
	assert.NoError(t, err)
	assert.Equal(t, "RuleSet2 executed", debugMessage)

	breContext.RuleSet = ""
	_, err = runner.RunRules(breContext, nil)
	assert.NoError(t, err)
	assert.Equal(t, "RuleSet3 executed", debugMessage)
}
