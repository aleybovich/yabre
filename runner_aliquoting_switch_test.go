package yabre

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Mermaid Export ─────────────────────────────────────────────────────────────

func TestMermaidAliquotingSwitch(t *testing.T) {
	yamlString, err := os.ReadFile("test/aliquoting_rules_switch.yaml")
	require.NoError(t, err)

	mmd, err := ExportMermaid(yamlString, "")
	require.NoError(t, err)

	// Verify switch conditions are represented
	assert.Contains(t, mmd, "check_weight")
	assert.Contains(t, mmd, "check_remainder")

	// Verify standard conditions are present
	assert.Contains(t, mmd, "create_products")
	assert.Contains(t, mmd, "check_powder_protocols")
	assert.Contains(t, mmd, "check_mixed_solvents")
	assert.Contains(t, mmd, "check_overflow")
	assert.Contains(t, mmd, "check_leftovers")

	fmt.Println(mmd)
}

func TestMermaidAliquotingSwitchFromLibrary(t *testing.T) {
	rl, valResult, err := NewRulesLibrary(RulesLibrarySettings{BasePath: "test", FileSystem: testFs})
	require.NoError(t, err)
	assert.Empty(t, valResult.Errors)

	mmd, err := ExportMermaidFromLibrary(rl, "aliquoting-rules-switch", "")
	require.NoError(t, err)

	assert.Contains(t, mmd, "check_weight")
	assert.Contains(t, mmd, "check_remainder")
}

// ─── Execution Tests ────────────────────────────────────────────────────────────

func newAliquotingSwitchRunner(t *testing.T, ctx *RecipeContext) *RulesRunner[RecipeContext] {
	t.Helper()

	rl, valResult, err := NewRulesLibrary(RulesLibrarySettings{BasePath: "test", FileSystem: testFs})
	require.NoError(t, err)
	assert.Empty(t, valResult.Errors)

	runner, err := NewRulesRunnerFromLibrary(
		rl,
		"aliquoting-rules-switch",
		ctx,
		WithDecisionCallback[RecipeContext](func(msg string, args ...any) {
			// silent
		}),
	)
	require.NoError(t, err)
	return runner
}

func TestAliquotingSwitch_TwoSpares(t *testing.T) {
	// Same scenario as TestRunnerAliquotingEmbedded:
	// Container=2100, powder(300) + solution(500) + solution(300)
	// Powder is failed, leaving 500+300=800 pending, remainder=1300 → two spares (900+400)
	ctx := &RecipeContext{
		Container: Container{Amount: 2100},
		OrderItems: []OrderItem{
			{Ref: 1, Rank: 1, ProductType: "powder", Amount: 300, Concentration: 10, Solvent: "water", OrderType: "primary", State: "pending"},
			{Ref: 2, Rank: 2, ProductType: "solution", Amount: 500, Concentration: 10, Solvent: "water", OrderType: "primary", State: "pending"},
			{Ref: 3, Rank: 3, ProductType: "solution", Amount: 300, Concentration: 10, Solvent: "water", OrderType: "early", State: "pending"},
		},
		Products: []Product{},
	}

	runner := newAliquotingSwitchRunner(t, ctx)
	result, err := runner.RunRules(ctx, nil)
	require.NoError(t, err)

	assert.Equal(t, 5, len(result.Products))

	// Powder product failed
	assert.Equal(t, "fail", result.Products[0].State)
	assert.Equal(t, 300, result.Products[0].Amount)

	// Two pending solution products
	assert.Equal(t, "pending", result.Products[1].State)
	assert.Equal(t, 500, result.Products[1].Amount)
	assert.Equal(t, "pending", result.Products[2].State)
	assert.Equal(t, 300, result.Products[2].Amount)

	// Two spare tubes: 900 + 400
	assert.Equal(t, "pending", result.Products[3].State)
	assert.Equal(t, 900, result.Products[3].Amount)
	assert.Equal(t, "pending", result.Products[4].State)
	assert.Equal(t, 400, result.Products[4].Amount)
}

func TestAliquotingSwitch_ExactAmount(t *testing.T) {
	// Container exactly matches required amount → terminates at "equal" case
	ctx := &RecipeContext{
		Container: Container{Amount: 800},
		OrderItems: []OrderItem{
			{Ref: 1, Rank: 1, ProductType: "solution", Amount: 500, Concentration: 10, Solvent: "water", OrderType: "primary", State: "pending"},
			{Ref: 2, Rank: 2, ProductType: "solution", Amount: 300, Concentration: 10, Solvent: "water", OrderType: "primary", State: "pending"},
		},
		Products: []Product{},
	}

	runner := newAliquotingSwitchRunner(t, ctx)
	result, err := runner.RunRules(ctx, nil)
	require.NoError(t, err)

	// Only the two original products, no spares
	assert.Equal(t, 2, len(result.Products))
	assert.Equal(t, "pending", result.Products[0].State)
	assert.Equal(t, "pending", result.Products[1].State)
}

func TestAliquotingSwitch_Overflow(t *testing.T) {
	// Total required exceeds container → all products fail
	ctx := &RecipeContext{
		Container: Container{Amount: 100},
		OrderItems: []OrderItem{
			{Ref: 1, Rank: 1, ProductType: "solution", Amount: 500, Concentration: 10, Solvent: "water", OrderType: "primary", State: "pending"},
			{Ref: 2, Rank: 2, ProductType: "solution", Amount: 300, Concentration: 10, Solvent: "water", OrderType: "primary", State: "pending"},
		},
		Products: []Product{},
	}

	runner := newAliquotingSwitchRunner(t, ctx)
	result, err := runner.RunRules(ctx, nil)
	require.NoError(t, err)

	for _, p := range result.Products {
		assert.Equal(t, "fail", p.State)
	}
}

func TestAliquotingSwitch_LessThanRequired_FailsLowestRank(t *testing.T) {
	// Container=400, required=200+300+500=1000 → diff=600
	// JS sorts by Ranking (undefined → all equal, but iteration order preserved)
	// Products are failed until diff <= 0:
	//   Product 0 (200): diff=600>0, fail, diff=400
	//   Product 1 (300): diff=400>0, fail, diff=100
	//   Product 2 (500): diff=100>0, fail, diff=-400
	// All failed, no leftovers → terminate
	ctx := &RecipeContext{
		Container: Container{Amount: 400},
		OrderItems: []OrderItem{
			{Ref: 1, Rank: 1, ProductType: "solution", Amount: 200, Concentration: 10, Solvent: "water", OrderType: "primary", State: "pending"},
			{Ref: 2, Rank: 2, ProductType: "solution", Amount: 300, Concentration: 10, Solvent: "water", OrderType: "primary", State: "pending"},
			{Ref: 3, Rank: 3, ProductType: "solution", Amount: 500, Concentration: 10, Solvent: "water", OrderType: "primary", State: "pending"},
		},
		Products: []Product{},
	}

	runner := newAliquotingSwitchRunner(t, ctx)
	result, err := runner.RunRules(ctx, nil)
	require.NoError(t, err)

	// All products should be failed (diff consumes them all)
	for _, p := range result.Products {
		assert.Equal(t, "fail", p.State)
	}
}

func TestAliquotingSwitch_SmallRemainder_Discard(t *testing.T) {
	// Container=830, required=800 → remainder=30 (<50) → discard (terminate, no spare)
	ctx := &RecipeContext{
		Container: Container{Amount: 830},
		OrderItems: []OrderItem{
			{Ref: 1, Rank: 1, ProductType: "solution", Amount: 500, Concentration: 10, Solvent: "water", OrderType: "primary", State: "pending"},
			{Ref: 2, Rank: 2, ProductType: "solution", Amount: 300, Concentration: 10, Solvent: "water", OrderType: "primary", State: "pending"},
		},
		Products: []Product{},
	}

	runner := newAliquotingSwitchRunner(t, ctx)
	result, err := runner.RunRules(ctx, nil)
	require.NoError(t, err)

	// Only original products, no spares
	assert.Equal(t, 2, len(result.Products))
	assert.Equal(t, "pending", result.Products[0].State)
	assert.Equal(t, "pending", result.Products[1].State)
}

func TestAliquotingSwitch_OneSpare(t *testing.T) {
	// Container=1100, required=800 → remainder=300 (>=50 and <950) → one spare
	ctx := &RecipeContext{
		Container: Container{Amount: 1100},
		OrderItems: []OrderItem{
			{Ref: 1, Rank: 1, ProductType: "solution", Amount: 500, Concentration: 10, Solvent: "water", OrderType: "primary", State: "pending"},
			{Ref: 2, Rank: 2, ProductType: "solution", Amount: 300, Concentration: 10, Solvent: "water", OrderType: "primary", State: "pending"},
		},
		Products: []Product{},
	}

	runner := newAliquotingSwitchRunner(t, ctx)
	result, err := runner.RunRules(ctx, nil)
	require.NoError(t, err)

	assert.Equal(t, 3, len(result.Products))
	assert.Equal(t, "pending", result.Products[0].State)
	assert.Equal(t, "pending", result.Products[1].State)
	// Spare with 300
	assert.Equal(t, 300, result.Products[2].Amount)
	assert.Equal(t, "pending", result.Products[2].State)
}

func TestAliquotingSwitch_ExcessiveRemainder_Fail(t *testing.T) {
	// Container=3000, required=800 → remainder=2200 (>1800) → fail all pending
	ctx := &RecipeContext{
		Container: Container{Amount: 3000},
		OrderItems: []OrderItem{
			{Ref: 1, Rank: 1, ProductType: "solution", Amount: 500, Concentration: 10, Solvent: "water", OrderType: "primary", State: "pending"},
			{Ref: 2, Rank: 2, ProductType: "solution", Amount: 300, Concentration: 10, Solvent: "water", OrderType: "primary", State: "pending"},
		},
		Products: []Product{},
	}

	runner := newAliquotingSwitchRunner(t, ctx)
	result, err := runner.RunRules(ctx, nil)
	require.NoError(t, err)

	for _, p := range result.Products {
		assert.Equal(t, "fail", p.State)
	}
}

func TestAliquotingSwitch_PowderProducts(t *testing.T) {
	// All powder products get failed, solutions remain pending
	ctx := &RecipeContext{
		Container: Container{Amount: 1000},
		OrderItems: []OrderItem{
			{Ref: 1, Rank: 1, ProductType: "powder", Amount: 200, Concentration: 10, Solvent: "water", OrderType: "primary", State: "pending"},
			{Ref: 2, Rank: 2, ProductType: "solution", Amount: 500, Concentration: 10, Solvent: "water", OrderType: "primary", State: "pending"},
		},
		Products: []Product{},
	}

	runner := newAliquotingSwitchRunner(t, ctx)
	result, err := runner.RunRules(ctx, nil)
	require.NoError(t, err)

	// Powder product failed
	assert.Equal(t, "fail", result.Products[0].State)
	assert.Equal(t, "powder", result.Products[0].ProductType)
	// Solution product stays pending
	assert.Equal(t, "pending", result.Products[1].State)
	// Remainder = 1000-500 = 500, >=50 and <950 → one spare
	require.Equal(t, 3, len(result.Products))
	assert.Equal(t, 500, result.Products[2].Amount)
}

// ─── Trace Test ─────────────────────────────────────────────────────────────────

func TestAliquotingSwitch_Trace_TwoSpares(t *testing.T) {
	ctx := &RecipeContext{
		Container: Container{Amount: 2100},
		OrderItems: []OrderItem{
			{Ref: 1, Rank: 1, ProductType: "powder", Amount: 300, Concentration: 10, Solvent: "water", OrderType: "primary", State: "pending"},
			{Ref: 2, Rank: 2, ProductType: "solution", Amount: 500, Concentration: 10, Solvent: "water", OrderType: "primary", State: "pending"},
			{Ref: 3, Rank: 3, ProductType: "solution", Amount: 300, Concentration: 10, Solvent: "water", OrderType: "early", State: "pending"},
		},
		Products: []Product{},
	}

	rl, valResult, err := NewRulesLibrary(RulesLibrarySettings{BasePath: "test", FileSystem: testFs})
	require.NoError(t, err)
	assert.Empty(t, valResult.Errors)

	var decisions []string
	runner, err := NewRulesRunnerFromLibrary(
		rl,
		"aliquoting-rules-switch",
		ctx,
		WithDecisionCallback[RecipeContext](func(msg string, args ...any) {
			msg = fmt.Sprintf(msg, args...)
			decisions = append(decisions, strings.Trim(strings.TrimLeft(msg, "\t"), " "))
		}),
	)
	require.NoError(t, err)

	_, trace, err := runner.RunRulesWithTrace(ctx, nil)
	require.NoError(t, err)

	// Verify trace includes switch condition results
	var switchTraces []TraceEntry
	for _, tr := range trace {
		if tr.SwitchResult != "" {
			switchTraces = append(switchTraces, tr)
		}
	}

	// We should have two switch evaluations: check_weight and check_remainder
	require.Equal(t, 2, len(switchTraces))
	assert.Equal(t, "check_weight", switchTraces[0].ConditionName)
	assert.Equal(t, "more", switchTraces[0].SwitchResult)
	assert.Equal(t, "check_remainder", switchTraces[1].ConditionName)
	assert.Equal(t, "two_spares", switchTraces[1].SwitchResult)
	assert.True(t, switchTraces[1].Terminated)

	// Verify decisions mention the switch conditions
	found := false
	for _, d := range decisions {
		if strings.Contains(d, "check_weight") {
			found = true
			break
		}
	}
	assert.True(t, found, "decisions should mention check_weight")
}
