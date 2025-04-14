package yabre

import (
	"embed"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type Product struct {
	Ref           int    `json:"ref"`
	Rank          int    `json:"rank"`
	OrderItemRef  int    `json:"orderItemRef"`
	OrderType     string `json:"orderType"`
	ProductType   string `json:"productType"`
	Amount        int    `json:"amount"`
	Concentration int    `json:"concentration"`
	Solvent       string `json:"solvent"`
	State         string `json:"state"`
}

type OrderItem struct {
	Ref           int    `json:"ref"`
	Rank          int    `json:"rank"`
	ProductType   string `json:"productType"`
	Amount        int    `json:"amount"`
	Concentration int    `json:"concentration"`
	Solvent       string `json:"solvent"`
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

	add := func(a, b float64) (interface{}, error) {
		return a + b, nil
	}

	rl, err := NewRulesLibrary(RulesLibrarySettings{BasePath: "./test"})
	assert.NoError(t, err)
	runner, err := NewRulesRunnerFromLibrary(rl, "go-rules", &context,
		WithDebugCallback[TestContext](
			func(data ...interface{}) {
				if len(data) > 0 {
					debugMessage = fmt.Sprintf("%v", data[0])
				}
			}),
		WithGoFunction[TestContext]("add", add))
	assert.NoError(t, err)

	_, err = runner.RunRules(&context, nil)
	assert.NoError(t, err)

	assert.Equal(t, "Go function result: 5.2", debugMessage)
}
func TestRunnerGoFunctionsVariadicArgs(t *testing.T) {
	type TestContext struct{}
	var debugMessage string
	context := TestContext{}

	add := func(args ...interface{}) (interface{}, error) {
		a := args[0].(float64) // 2.2
		b := args[1].(int64)   // 3

		return a + float64(b), nil
	}

	rl, err := NewRulesLibrary(RulesLibrarySettings{BasePath: "./test"})
	assert.NoError(t, err)
	runner, err := NewRulesRunnerFromLibrary(rl, "go-rules", &context,
		WithDebugCallback[TestContext](
			func(data ...interface{}) {
				if len(data) > 0 {
					debugMessage = fmt.Sprintf("%v", data[0])
				}
			}),
		WithGoFunction[TestContext]("add", add))
	assert.NoError(t, err)

	_, err = runner.RunRules(&context, nil)
	assert.NoError(t, err)

	assert.Equal(t, "Go function result: 5.2", debugMessage)
}

//go:embed test
var testFs embed.FS

func TestRunnerUpdateContextEmbedded(t *testing.T) {
	type TestContext struct{ Value string }
	context := TestContext{Value: "Initial"}

	rl, err := NewRulesLibrary(RulesLibrarySettings{BasePath: "./test", FileSystem: testFs})
	assert.NoError(t, err)
	runner, err := NewRulesRunnerFromLibrary(rl, "update-context", &context)
	assert.NoError(t, err)

	updatedContext, err := runner.RunRules(&context, nil)
	assert.NoError(t, err)

	assert.Equal(t, "Updated", updatedContext.Value)
}

func TestRunnerAliquotingEmbedded(t *testing.T) {
	// Create a sample context
	context := RecipeContext{
		Container: Container{Amount: 2100},
		OrderItems: []OrderItem{
			{Ref: 1, Rank: 1, ProductType: "powder", Amount: 300, Concentration: 10, Solvent: "water", OrderType: "primary", State: "pending"},
			{Ref: 2, Rank: 2, ProductType: "solution", Amount: 500, Concentration: 10, Solvent: "water", OrderType: "primary", State: "pending"},
			{Ref: 3, Rank: 3, ProductType: "solution", Amount: 300, Concentration: 10, Solvent: "water", OrderType: "early", State: "pending"},
		},
		Products: []Product{},
	}

	var debugData interface{}

	decisions := []string{}

	rl, err := NewRulesLibrary(RulesLibrarySettings{BasePath: "test", FileSystem: testFs})

	assert.NoError(t, err)

	runner, err := NewRulesRunnerFromLibrary(
		rl,
		"aliquoting-rules",
		&context,
		WithDebugCallback[RecipeContext](
			func(data ...interface{}) {
				if len(data) > 0 {
					debugData = data[0]
				}
			}),
		WithDecisionCallback[RecipeContext](func(msg string, args ...interface{}) {
			msg = fmt.Sprintf(msg, args...)
			//fmt.Print(msg)
			decisions = append(decisions, strings.Trim(strings.TrimLeft(msg, "\t"), " "))
		}))
	assert.NoError(t, err)

	// Run the rules
	updatedContext, err := runner.RunRules(&context, nil)

	assert.NoError(t, err)
	assert.NotNil(t, updatedContext)

	// check debug string
	debugString, ok := debugData.(string)

	assert.True(t, ok)
	assert.Equal(t, "create_products check", debugString)

	// Check the updated context
	assert.Equal(t, 5, len(updatedContext.Products))

	assert.Equal(t, 300, updatedContext.Products[0].Amount)
	assert.Equal(t, "fail", updatedContext.Products[0].State)

	assert.Equal(t, 500, updatedContext.Products[1].Amount)
	assert.Equal(t, "pending", updatedContext.Products[1].State)

	assert.Equal(t, 300, updatedContext.Products[2].Amount)
	assert.Equal(t, "pending", updatedContext.Products[2].State)

	assert.Equal(t, 900, updatedContext.Products[3].Amount)
	assert.Equal(t, "pending", updatedContext.Products[3].State)

	assert.Equal(t, 400, updatedContext.Products[4].Amount)
	assert.Equal(t, "pending", updatedContext.Products[4].State)

	//p, _ := json.MarshalIndent(updatedContext.Products, "", "  ")
	//fmt.Printf("Products: %v\n", string(p))

	expectedDecisions := []string{
		"Evaluating condition: [create_products] Create products for order items in state \"pending\"",
		"Condition [create_products] evaluated to [true]",
		"Running action: [create_products_true]",
		"Moving to next condition:[check_powder_protocols]",
		"Evaluating condition: [check_powder_protocols] Check if there are any powder protocols among the products.",
		"Condition [check_powder_protocols] evaluated to [true]",
		"Running action: [check_powder_protocols_true] Fail all powder products and their corresponding order items.",
		"Moving to next condition:[check_mixed_solvents]",
		"Evaluating condition: [check_mixed_solvents] Check if there are mixed solvents or concentrations among the solution order items.",
		"Condition [check_mixed_solvents] evaluated to [false]",
		"Moving to next condition:[check_overflow]",
		"Evaluating condition: [check_overflow] Check if the total required amount exceeds the container amount.",
		"Condition [check_overflow] evaluated to [false]",
		"Moving to next condition:[check_amount_less_than_required]",
		"Evaluating condition: [check_amount_less_than_required] Check if the actual amount is less than the required amount.",
		"Condition [check_amount_less_than_required] evaluated to [false]",
		"Moving to next condition:[check_amount_more_than_required]",
		"Evaluating condition: [check_amount_more_than_required] Check if the actual amount is more than the required amount.",
		"Condition [check_amount_more_than_required] evaluated to [true]",
		"Moving to next condition:[check_remainder_less_than_50]",
		"Evaluating condition: [check_remainder_less_than_50] Check if the remainder is less than 50 μl.",
		"Condition [check_remainder_less_than_50] evaluated to [false]",
		"Moving to next condition:[check_remainder_between_50_and_950]",
		"Evaluating condition: [check_remainder_between_50_and_950] Check if the remainder is between 50 μl and 950 μl.",
		"Condition [check_remainder_between_50_and_950] evaluated to [false]",
		"Moving to next condition:[check_remainder_between_950_and_1800]",
		"Evaluating condition: [check_remainder_between_950_and_1800] Check if the remainder is between 950 μl and 1800 μl.",
		"Condition [check_remainder_between_950_and_1800] evaluated to [true]",
		"Running action: [check_remainder_between_950_and_1800_true] Create two spare tubes, one with 900 μl and another with the remaining amount.",
		"Terminating",
	}
	assert.Equal(t, expectedDecisions, decisions)
}

func TestLoanApproval(t *testing.T) {
	// Load the YAML rules
	rl, err := NewRulesLibrary(RulesLibrarySettings{BasePath: "test", FileSystem: testFs})
	assert.NoError(t, err)

	// Test case for the happy path
	t.Run("HappyPath", func(t *testing.T) {
		context := struct {
			Applicants []Applicant
			LoanAmount int
			Decision   string
			Reason     string
		}{
			Applicants: []Applicant{
				{Type: "primary", Age: 25, Income: 5000, Debt: 1000, CreditScore: 750},
				{Type: "co-applicant", Age: 30, Income: 4000, Debt: 500, CreditScore: 700},
			},
			LoanAmount: 40000,
		}

		runner, err := NewRulesRunnerFromLibrary(rl, "loan-approval", &context)
		assert.NoError(t, err)

		_, err = runner.RunRules(&context, nil)
		assert.NoError(t, err)
		assert.Equal(t, "approved", context.Decision)
		assert.Empty(t, context.Reason)
	})

	// Test cases for failed decisions
	testCases := []struct {
		name     string
		context  LoanContext
		expected struct {
			decision string
			reason   string
		}
	}{
		{
			name: "MissingPrimaryApplicant",
			context: LoanContext{
				Applicants: []Applicant{
					{Type: "co-applicant", Age: 30, Income: 4000, Debt: 500, CreditScore: 700},
				},
			},
			expected: struct {
				decision string
				reason   string
			}{
				decision: "rejected",
				reason:   "No primary applicant",
			},
		},
		{
			name: "UnderagePrimaryApplicant",
			context: LoanContext{
				Applicants: []Applicant{
					{Type: "primary", Age: 17, Income: 5000, Debt: 1000, CreditScore: 750},
				},
			},
			expected: struct {
				decision string
				reason   string
			}{
				decision: "rejected",
				reason:   "Primary applicant is underage",
			},
		},
		{
			name: "InsufficientIncome",
			context: LoanContext{
				Applicants: []Applicant{
					{Type: "primary", Age: 25, Income: 500, Debt: 1000, CreditScore: 750},
				},
			},
			expected: struct {
				decision string
				reason   string
			}{
				decision: "rejected",
				reason:   "Insufficient income",
			},
		},
		{
			name: "LowCreditScore",
			context: LoanContext{
				Applicants: []Applicant{
					{Type: "primary", Age: 25, Income: 5000, Debt: 1000, CreditScore: 550},
				},
			},
			expected: struct {
				decision string
				reason   string
			}{
				decision: "rejected",
				reason:   "Low credit score",
			},
		},
		{
			name: "UnderageCoApplicant",
			context: LoanContext{
				Applicants: []Applicant{
					{Type: "primary", Age: 25, Income: 5000, Debt: 1000, CreditScore: 750},
					{Type: "co-applicant", Age: 17, Income: 4000, Debt: 500, CreditScore: 700},
				},
			},
			expected: struct {
				decision string
				reason   string
			}{
				decision: "rejected",
				reason:   "Co-applicant is underage",
			},
		},
		{
			name: "LowCoApplicantCreditScore",
			context: LoanContext{
				Applicants: []Applicant{
					{Type: "primary", Age: 25, Income: 5000, Debt: 1000, CreditScore: 750},
					{Type: "co-applicant", Age: 30, Income: 4000, Debt: 500, CreditScore: 550},
				},
			},
			expected: struct {
				decision string
				reason   string
			}{
				decision: "rejected",
				reason:   "Co-applicant has low credit score",
			},
		},
		{
			name: "HighDebtToIncomeRatio",
			context: LoanContext{
				Applicants: []Applicant{
					{Type: "primary", Age: 25, Income: 5000, Debt: 4000, CreditScore: 750},
				},
			},
			expected: struct {
				decision string
				reason   string
			}{
				decision: "rejected",
				reason:   "High debt-to-income ratio",
			},
		},
		{
			name: "ExcessiveLoanAmount",
			context: LoanContext{
				Applicants: []Applicant{
					{Type: "primary", Age: 25, Income: 5000, Debt: 1000, CreditScore: 750},
				},
				LoanAmount: 100000,
			},
			expected: struct {
				decision string
				reason   string
			}{
				decision: "rejected",
				reason:   "Excessive loan amount",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			runner, err := NewRulesRunnerFromLibrary(rl, "loan-approval", &tc.context)

			assert.NoError(t, err)

			_, err = runner.RunRules(&tc.context, nil)
			assert.NoError(t, err)

			assert.Equal(t, tc.expected.decision, runner.Context.Decision)
			assert.Equal(t, tc.expected.reason, runner.Context.Reason)
		})
	}
}

type LoanContext struct {
	Applicants []Applicant
	LoanAmount int
	Decision   string
	Reason     string
}

type Applicant struct {
	Type        string
	Age         int
	Income      int
	Debt        int
	CreditScore int
}

func loadYaml(fileName string) ([]byte, error) {
	yamlFile, err := os.ReadFile(fileName)
	if err != nil {
		return nil, fmt.Errorf("error reading YAML file: %v", err)
	}

	return yamlFile, nil
}
