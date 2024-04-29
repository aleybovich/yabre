package main

import (
	"fmt"
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

	_, err = runner.RunRules(&context, "check_debug")
	assert.NoError(t, err)

	assert.Equal(t, "Go function result: 5", debugMessage)
}

func TestRunnerUpdateContext(t *testing.T) {
	type TestContext struct{ Value string }
	context := TestContext{Value: "Initial"}

	runner, err := NewRulesRunnerFromYaml("test/update_context.yaml", &context)
	assert.NoError(t, err)

	updatedContext, err := runner.RunRules(&context, "check_update_context")
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

	runner, err := NewRulesRunnerFromYaml("test/aliquoting_rules.yaml", &context, WithDebugCallback(
		func(ctx RecipeContext, data interface{}) {
			debugData = data
		}))
	assert.NoError(t, err)

	// Run the rules
	updatedContext, err := runner.RunRules(&context, "check_powder_protocols")
	assert.NoError(t, err)
	assert.NotNil(t, updatedContext)

	// check debug string
	debugString, ok := debugData.(string)

	assert.True(t, ok)
	assert.Equal(t, "I'm in check_powder_protocols", debugString)

	// Check the updated context
	assert.Equal(t, 2, len(updatedContext.Products))
}
