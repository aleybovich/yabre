package yabre

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type Product struct {
	Ref          int    `json:"ref"`
	Rank         int    `json:"rank"`
	OrderItemRef int    `json:"orderItemRef"`
	ProductType  string `json:"productType"`
	Amount       int    `json:"amount"`
	State        string `json:"state"`
}

type OrderItem struct {
	Ref           int    `json:"ref"`
	Rank          int    `json:"rank"`
	ProductType   string `json:"productType"`
	Amount        int    `json:"amount"`
	Concentration int    `json:"concentration"`
	Solvant       string `json:"solvant"`
	OrderType     string `json:"orderType"`
	State         string `json:"state"`
}

type Container struct {
	Amount int `json:"amount"`
}

type RecipeContext struct {
	Container  Container   `json:"container"`
	OrderItems []OrderItem `json:"orderItems"`
	Products   []Product   `json:"products"`
}

func TestRunnerGoFunctions(t *testing.T) {
	type TestContext struct{}
	var debugMessage string
	context := TestContext{}

	add := func(args ...interface{}) (interface{}, error) {
		res := int64(0)
		for _, arg := range args {
			res += arg.(int64)
		}
		return res, nil
	}

	runner, err := NewRulesRunnerFromYaml("test/go_rules.yaml", &context,
		WithDebugCallback(
			func(ctx TestContext, data interface{}) {
				debugMessage = fmt.Sprintf("%v", data)
			}),
		WithGoFunction[TestContext]("add", add))
	assert.NoError(t, err)

	_, err = runner.RunRules(&context, nil)
	assert.NoError(t, err)

	assert.Equal(t, "Go function result: 5", debugMessage)
}

func TestRunnerUpdateContext(t *testing.T) {
	type TestContext struct{ Value string }
	context := TestContext{Value: "Initial"}

	runner, err := NewRulesRunnerFromYaml("test/update_context.yaml", &context)
	assert.NoError(t, err)

	updatedContext, err := runner.RunRules(&context, nil)
	assert.NoError(t, err)

	assert.Equal(t, "Updated", updatedContext.Value)
}

func TestRunner(t *testing.T) {
	// Create a sample context
	context := RecipeContext{
		Container: Container{Amount: 2100},
		OrderItems: []OrderItem{
			{Ref: 1, Rank: 1, ProductType: "powder", Amount: 300, Concentration: 10, Solvant: "water", OrderType: "primary", State: "pending"},
			{Ref: 2, Rank: 2, ProductType: "solution", Amount: 500, Concentration: 10, Solvant: "water", OrderType: "primary", State: "pending"},
			{Ref: 3, Rank: 3, ProductType: "solution", Amount: 300, Concentration: 10, Solvant: "water", OrderType: "early", State: "pending"},
		},
		Products: []Product{},
	}

	var debugData interface{}

	decisions := []string{}

	runner, err := NewRulesRunnerFromYaml(
		"test/aliquoting_rules.yaml",
		&context,
		WithDebugCallback(
			func(ctx RecipeContext, data interface{}) {
				debugData = data
			}),
		WithDecisionCallback[RecipeContext](func(msg string, args ...interface{}) {
			msg = fmt.Sprintf(msg, args...)
			fmt.Print(msg)
			decisions = append(decisions, strings.TrimLeft(strings.TrimRight(msg, "\n"), "\t"))
		}))
	assert.NoError(t, err)

	// Run the rules
	updatedContext, err := runner.RunRules(&context, nil)
	assert.NoError(t, err)
	assert.NotNil(t, updatedContext)

	// check debug string
	debugString, ok := debugData.(string)

	assert.True(t, ok)
	assert.Equal(t, "I'm in check_powder_protocols", debugString)

	// Check the updated context
	assert.Equal(t, 2, len(updatedContext.Products))
	assert.Equal(t, 900, updatedContext.Products[0].Amount)
	assert.Equal(t, 400, updatedContext.Products[1].Amount)

	expectedDecisions := []string{
		"Running condition: [check_powder_protocols] Check if there are any powder protocols among the products.",
		"Running action: [check_powder_protocols_true] Fail all powder products and their corresponding order items.",
		"Moving to next condition: check_mixed_solvents",
		"Running condition: [check_mixed_solvents] Check if there are mixed solvents or concentrations among the solution order items.",
		"Moving to next condition: check_overflow",
		"Running condition: [check_overflow] Check if the total required amount exceeds the container amount.",
		"Moving to next condition: check_amount_less_than_required",
		"Running condition: [check_amount_less_than_required] Check if the actual amount is less than the required amount.",
		"Moving to next condition: check_amount_equal_to_required",
		"Running condition: [check_amount_equal_to_required] Check if the actual amount is equal to the required amount.",
		"Moving to next condition: check_amount_more_than_required",
		"Running condition: [check_amount_more_than_required] Check if the actual amount is more than the required amount.",
		"Moving to next condition: check_remainder_less_than_50",
		"Running condition: [check_remainder_less_than_50] Check if the remainder is less than 50 μl.",
		"Moving to next condition: check_remainder_between_50_and_950",
		"Running condition: [check_remainder_between_50_and_950] Check if the remainder is between 50 μl and 950 μl.",
		"Moving to next condition: check_remainder_between_950_and_1800",
		"Running condition: [check_remainder_between_950_and_1800] Check if the remainder is between 950 μl and 1800 μl.",
		"Running action: [check_remainder_between_950_and_1800_true] Create two spare tubes, one with 900 μl and another with the remaining amount.",
		"Terminating",
	}
	assert.Equal(t, expectedDecisions, decisions)
}
