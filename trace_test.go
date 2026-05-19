package yabre

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// assertDurationsNonNegative verifies that every TraceEntry has a non-negative
// Duration. Duration == 0 is acceptable (very fast check on modern hardware).
func assertDurationsNonNegative(t *testing.T, trace []TraceEntry) {
	t.Helper()
	for i, e := range trace {
		assert.GreaterOrEqual(t, e.Duration, time.Duration(0),
			"entry %d (%s) has negative Duration", i, e.ConditionName)
	}
}

// ---------------------------------------------------------------------------
// Basic single-condition tests
// ---------------------------------------------------------------------------

// TestTrace_SingleConditionTerminatesTrue verifies that a condition that
// evaluates to true and immediately terminates produces exactly one entry
// with all fields correctly populated.
func TestTrace_SingleConditionTerminatesTrue(t *testing.T) {
	yamlRules := `
name: "single-true"
conditions:
  start:
    description: "Evaluate something trivially true"
    default: true
    check: "function() { return true; }"
    true:
      action: "function() { context.ran = true; }"
      terminate: true
`
	library, valResult := createLibraryFromYAML(t, yamlRules, "single-true.yaml")
	assert.Empty(t, valResult.Errors)
	assert.Empty(t, valResult.Warnings)

	ctx := map[string]any{}
	runner, err := NewRulesRunnerFromLibrary(library, "single-true", &ctx)
	require.NoError(t, err)

	_, trace, err := runner.RunRulesWithTrace(&ctx, nil)
	require.NoError(t, err)

	require.Len(t, trace, 1)
	e := trace[0]
	assert.Equal(t, "start", e.ConditionName)
	assert.Equal(t, "Evaluate something trivially true", e.Description)
	assert.True(t, e.Result)
	assert.True(t, e.HasAction)
	assert.Empty(t, e.NextCondition)
	assert.True(t, e.Terminated)
	assert.Nil(t, e.Error)
	assert.GreaterOrEqual(t, e.Duration, time.Duration(0))
}

// TestTrace_SingleConditionTerminatesFalse verifies that a condition that
// evaluates to false and immediately terminates produces one entry with
// Result==false.
func TestTrace_SingleConditionTerminatesFalse(t *testing.T) {
	yamlRules := `
name: "single-false"
conditions:
  start:
    description: "Evaluate something trivially false"
    default: true
    check: "function() { return false; }"
    true:
      terminate: true
    false:
      action: "function() { context.ran = true; }"
      terminate: true
`
	library, valResult := createLibraryFromYAML(t, yamlRules, "single-false.yaml")
	assert.Empty(t, valResult.Errors)
	assert.Empty(t, valResult.Warnings)

	ctx := map[string]any{}
	runner, err := NewRulesRunnerFromLibrary(library, "single-false", &ctx)
	require.NoError(t, err)

	_, trace, err := runner.RunRulesWithTrace(&ctx, nil)
	require.NoError(t, err)

	require.Len(t, trace, 1)
	e := trace[0]
	assert.Equal(t, "start", e.ConditionName)
	assert.False(t, e.Result)
	assert.True(t, e.HasAction)
	assert.True(t, e.Terminated)
	assert.Nil(t, e.Error)
}

// TestTrace_NilDecisionBranch verifies that when a condition has no matching
// branch (the evaluated branch is nil), execution implicitly terminates and
// the trace records Terminated==true.
func TestTrace_NilDecisionBranch(t *testing.T) {
	// The condition only defines a true branch; the check returns false,
	// so decision is nil -> implicit termination.
	yamlRules := `
name: "nil-branch"
conditions:
  start:
    description: "Nil branch test"
    default: true
    check: "function() { return false; }"
    true:
      terminate: true
`
	library, valResult := createLibraryFromYAML(t, yamlRules, "nil-branch.yaml")
	assert.Empty(t, valResult.Errors)
	assert.Empty(t, valResult.Warnings)

	ctx := map[string]any{}
	runner, err := NewRulesRunnerFromLibrary(library, "nil-branch", &ctx)
	require.NoError(t, err)

	_, trace, err := runner.RunRulesWithTrace(&ctx, nil)
	require.NoError(t, err)

	require.Len(t, trace, 1)
	e := trace[0]
	assert.Equal(t, "start", e.ConditionName)
	assert.False(t, e.Result)
	assert.False(t, e.HasAction)
	assert.Empty(t, e.NextCondition)
	assert.True(t, e.Terminated, "implicit termination should set Terminated=true")
	assert.Nil(t, e.Error)
}

// ---------------------------------------------------------------------------
// Chained conditions - order and field verification
// ---------------------------------------------------------------------------

// TestTrace_ChainedConditionsOrder verifies that three chained conditions
// appear in the trace in chronological execution order (step1, step2, step3).
func TestTrace_ChainedConditionsOrder(t *testing.T) {
	yamlRules := `
name: "chain-order"
conditions:
  step1:
    description: "Step 1"
    default: true
    check: "function() { return true; }"
    true:
      next: "step2"
  step2:
    description: "Step 2"
    check: "function() { return true; }"
    true:
      next: "step3"
  step3:
    description: "Step 3"
    check: "function() { return true; }"
    true:
      action: "function() { context.done = true; }"
      terminate: true
    false:
      terminate: true
`
	library, valResult := createLibraryFromYAML(t, yamlRules, "chain-order.yaml")
	assert.Empty(t, valResult.Errors)
	assert.Empty(t, valResult.Warnings)

	ctx := map[string]any{}
	runner, err := NewRulesRunnerFromLibrary(library, "chain-order", &ctx)
	require.NoError(t, err)

	_, trace, err := runner.RunRulesWithTrace(&ctx, nil)
	require.NoError(t, err)

	require.Len(t, trace, 3)

	assert.Equal(t, "step1", trace[0].ConditionName)
	assert.Equal(t, "Step 1", trace[0].Description)
	assert.True(t, trace[0].Result)
	assert.False(t, trace[0].HasAction)
	assert.Equal(t, "step2", trace[0].NextCondition)
	assert.False(t, trace[0].Terminated)
	assert.Nil(t, trace[0].Error)

	assert.Equal(t, "step2", trace[1].ConditionName)
	assert.True(t, trace[1].Result)
	assert.False(t, trace[1].HasAction)
	assert.Equal(t, "step3", trace[1].NextCondition)
	assert.False(t, trace[1].Terminated)
	assert.Nil(t, trace[1].Error)

	assert.Equal(t, "step3", trace[2].ConditionName)
	assert.True(t, trace[2].Result)
	assert.True(t, trace[2].HasAction)
	assert.Empty(t, trace[2].NextCondition)
	assert.True(t, trace[2].Terminated)
	assert.Nil(t, trace[2].Error)

	assertDurationsNonNegative(t, trace)
}

// ---------------------------------------------------------------------------
// Loan approval - file-based, real-world rule set
// ---------------------------------------------------------------------------

// TestTrace_LoanApprovalHappyPath exercises the loan-approval rule set with
// a context that passes all checks. The expected path traverses 9 conditions.
func TestTrace_LoanApprovalHappyPath(t *testing.T) {
	rl, valResult, err := NewRulesLibrary(RulesLibrarySettings{BasePath: "test", FileSystem: testFs})
	require.NoError(t, err)
	assertBREValidationWarnings(t, valResult)

	ctx := LoanContext{
		Applicants: []Applicant{
			{Type: "primary", Age: 25, Income: 5000, Debt: 1000, CreditScore: 750},
			{Type: "co-applicant", Age: 30, Income: 4000, Debt: 500, CreditScore: 700},
		},
		LoanAmount: 40000,
	}
	runner, err := NewRulesRunnerFromLibrary(rl, "loan-approval", &ctx)
	require.NoError(t, err)

	_, trace, err := runner.RunRulesWithTrace(&ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, "approved", ctx.Decision)

	// The happy-path chain in order:
	expectedConditions := []string{
		"check_primary_applicant",
		"check_applicant_age",
		"check_applicant_income",
		"check_applicant_credit_score",
		"check_co_applicant",
		"check_co_applicant_age",
		"check_co_applicant_credit_score",
		"check_debt_to_income_ratio",
		"check_loan_amount",
	}
	require.Len(t, trace, len(expectedConditions))

	for i, name := range expectedConditions {
		assert.Equal(t, name, trace[i].ConditionName, "entry %d", i)
		assert.True(t, trace[i].Result, "entry %d (%s) should be true", i, name)
		assert.Nil(t, trace[i].Error, "entry %d (%s) should have no error", i, name)
	}

	// Verify the final condition (check_loan_amount) ran an action and terminated.
	last := trace[len(trace)-1]
	assert.True(t, last.HasAction)
	assert.True(t, last.Terminated)
	assert.Empty(t, last.NextCondition)

	// Verify intermediate conditions did not run actions and did not terminate.
	for i := 0; i < len(trace)-1; i++ {
		assert.False(t, trace[i].HasAction, "intermediate entry %d (%s) should not have run action", i, trace[i].ConditionName)
		assert.False(t, trace[i].Terminated, "intermediate entry %d (%s) should not be terminated", i, trace[i].ConditionName)
	}

	// Verify the NextCondition chain links correctly.
	for i := 0; i < len(expectedConditions)-1; i++ {
		assert.Equal(t, expectedConditions[i+1], trace[i].NextCondition,
			"entry %d (%s) NextCondition mismatch", i, trace[i].ConditionName)
	}

	assertDurationsNonNegative(t, trace)
}

// TestTrace_LoanApprovalUnderagePrimary verifies that the trace correctly
// captures an early-exit path: only 2 conditions are evaluated.
func TestTrace_LoanApprovalUnderagePrimary(t *testing.T) {
	rl, valResult, err := NewRulesLibrary(RulesLibrarySettings{BasePath: "test", FileSystem: testFs})
	require.NoError(t, err)
	assertBREValidationWarnings(t, valResult)

	ctx := LoanContext{
		Applicants: []Applicant{
			{Type: "primary", Age: 17, Income: 5000, Debt: 1000, CreditScore: 750},
		},
	}
	runner, err := NewRulesRunnerFromLibrary(rl, "loan-approval", &ctx)
	require.NoError(t, err)

	_, trace, err := runner.RunRulesWithTrace(&ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, "rejected", ctx.Decision)
	assert.Equal(t, "Primary applicant is underage", ctx.Reason)

	require.Len(t, trace, 2)

	// First condition: primary applicant exists -> true branch -> next check_applicant_age.
	assert.Equal(t, "check_primary_applicant", trace[0].ConditionName)
	assert.True(t, trace[0].Result)
	assert.False(t, trace[0].HasAction)
	assert.Equal(t, "check_applicant_age", trace[0].NextCondition)
	assert.False(t, trace[0].Terminated)
	assert.Nil(t, trace[0].Error)

	// Second condition: age < 18 -> false branch -> action sets rejection + terminates.
	assert.Equal(t, "check_applicant_age", trace[1].ConditionName)
	assert.False(t, trace[1].Result)
	assert.True(t, trace[1].HasAction)
	assert.Empty(t, trace[1].NextCondition)
	assert.True(t, trace[1].Terminated)
	assert.Nil(t, trace[1].Error)

	assertDurationsNonNegative(t, trace)
}

// ---------------------------------------------------------------------------
// Error handling
// ---------------------------------------------------------------------------

// TestTrace_ErrorInCheckFunction verifies that a JavaScript runtime error
// thrown inside a check function is captured in the trace entry and also
// returned as the RunRulesWithTrace error.
func TestTrace_ErrorInCheckFunction(t *testing.T) {
	yamlRules := `
name: "check-error"
conditions:
  errorCond:
    description: "Check that throws"
    default: true
    check: |
      function() {
        throw new Error("intentional check error");
      }
    true:
      terminate: true
`
	library, valResult := createLibraryFromYAML(t, yamlRules, "check-error.yaml")
	assert.Empty(t, valResult.Errors)
	assert.Empty(t, valResult.Warnings)

	ctx := map[string]any{}
	runner, err := NewRulesRunnerFromLibrary(library, "check-error", &ctx)
	require.NoError(t, err)

	_, trace, runErr := runner.RunRulesWithTrace(&ctx, nil)
	require.Error(t, runErr)
	assert.Contains(t, runErr.Error(), "intentional check error")

	// A slot should have been reserved before the check was evaluated.
	require.Len(t, trace, 1)
	e := trace[0]
	assert.Equal(t, "errorCond", e.ConditionName)
	assert.Equal(t, "Check that throws", e.Description)

	// Result, HasAction, NextCondition, Terminated are all zero-valued because
	// we never reached the branch-selection logic.
	assert.False(t, e.Result)
	assert.False(t, e.HasAction)
	assert.Empty(t, e.NextCondition)
	assert.False(t, e.Terminated)

	// Error must be set and Duration must be non-negative.
	require.Error(t, e.Error)
	assert.Contains(t, e.Error.Error(), "intentional check error")
	assert.GreaterOrEqual(t, e.Duration, time.Duration(0))
}

// TestTrace_ErrorInActionFunction verifies that a JavaScript runtime error
// thrown inside an action function is captured in the trace entry. Because
// the error happens after the check succeeds, Result is true and HasAction
// is true in the trace.
func TestTrace_ErrorInActionFunction(t *testing.T) {
	yamlRules := `
name: "action-error"
conditions:
  normalCond:
    description: "Check passes, action throws"
    default: true
    check: "function() { return true; }"
    true:
      action: |
        function() {
          throw new Error("intentional action error");
        }
      terminate: true
`
	library, valResult := createLibraryFromYAML(t, yamlRules, "action-error.yaml")
	assert.Empty(t, valResult.Errors)
	assert.Empty(t, valResult.Warnings)

	ctx := map[string]any{}
	runner, err := NewRulesRunnerFromLibrary(library, "action-error", &ctx)
	require.NoError(t, err)

	_, trace, runErr := runner.RunRulesWithTrace(&ctx, nil)
	require.Error(t, runErr)
	assert.Contains(t, runErr.Error(), "intentional action error")

	require.Len(t, trace, 1)
	e := trace[0]
	assert.Equal(t, "normalCond", e.ConditionName)
	assert.True(t, e.Result)    // Check succeeded before the action failed.
	assert.True(t, e.HasAction) // Pre-filled from Decision before action was invoked.
	assert.True(t, e.Terminated)

	require.Error(t, e.Error)
	assert.Contains(t, e.Error.Error(), "intentional action error")
	assert.GreaterOrEqual(t, e.Duration, time.Duration(0))
}

// ---------------------------------------------------------------------------
// Aliquoting rules - deep chain (9 conditions)
// ---------------------------------------------------------------------------

// TestTrace_AliquotingRules exercises the aliquoting-rules file-based rule set
// and verifies that all 9 traversed conditions appear in the trace in order.
func TestTrace_AliquotingRules(t *testing.T) {
	rl, valResult, err := NewRulesLibrary(RulesLibrarySettings{BasePath: "test", FileSystem: testFs})
	require.NoError(t, err)
	assertBREValidationWarnings(t, valResult)

	ctx := RecipeContext{
		Container: Container{Amount: 2100},
		OrderItems: []OrderItem{
			{Ref: 1, Rank: 1, ProductType: "powder", Amount: 300, Concentration: 10, Solvent: "water", OrderType: "primary", State: "pending"},
			{Ref: 2, Rank: 2, ProductType: "solution", Amount: 500, Concentration: 10, Solvent: "water", OrderType: "primary", State: "pending"},
			{Ref: 3, Rank: 3, ProductType: "solution", Amount: 300, Concentration: 10, Solvent: "water", OrderType: "early", State: "pending"},
		},
		Products: []Product{},
	}
	runner, err := NewRulesRunnerFromLibrary(rl, "aliquoting-rules", &ctx,
		WithDebugCallback[RecipeContext](func(data ...any) {}))
	require.NoError(t, err)

	_, trace, err := runner.RunRulesWithTrace(&ctx, nil)
	require.NoError(t, err)

	// The expected execution path mirrors the conditions from the existing
	// TestRunnerAliquotingEmbedded decision log.
	expectedPath := []struct {
		name          string
		result        bool
		actionRan     bool
		nextCondition string
		terminated    bool
	}{
		{"create_products", true, true, "check_powder_protocols", false},
		{"check_powder_protocols", true, true, "check_mixed_solvents", false},
		{"check_mixed_solvents", false, false, "check_overflow", false},
		{"check_overflow", false, false, "check_amount_less_than_required", false},
		{"check_amount_less_than_required", false, false, "check_amount_more_than_required", false},
		{"check_amount_more_than_required", true, false, "check_remainder_less_than_50", false},
		{"check_remainder_less_than_50", false, false, "check_remainder_between_50_and_950", false},
		{"check_remainder_between_50_and_950", false, false, "check_remainder_between_950_and_1800", false},
		{"check_remainder_between_950_and_1800", true, true, "", true},
	}

	require.Len(t, trace, len(expectedPath))

	for i, exp := range expectedPath {
		e := trace[i]
		assert.Equal(t, exp.name, e.ConditionName, "entry %d name", i)
		assert.Equal(t, exp.result, e.Result, "entry %d (%s) result", i, exp.name)
		assert.Equal(t, exp.actionRan, e.HasAction, "entry %d (%s) actionRan", i, exp.name)
		assert.Equal(t, exp.nextCondition, e.NextCondition, "entry %d (%s) nextCondition", i, exp.name)
		assert.Equal(t, exp.terminated, e.Terminated, "entry %d (%s) terminated", i, exp.name)
		assert.Nil(t, e.Error, "entry %d (%s) error", i, exp.name)
	}

	assertDurationsNonNegative(t, trace)
}

// ---------------------------------------------------------------------------
// Concurrency - independent traces per call
// ---------------------------------------------------------------------------

// TestTrace_ConcurrentRunRulesWithTrace verifies that 10 concurrent
// RunRulesWithTrace calls on the same runner produce independent, non-shared
// trace slices. goja allocates a new VM per call, so this is goroutine-safe.
func TestTrace_ConcurrentRunRulesWithTrace(t *testing.T) {
	yamlRules := `
name: "concurrent-trace"
conditions:
  checkA:
    description: "Check A"
    default: true
    check: "function() { return true; }"
    true:
      next: "checkB"
  checkB:
    description: "Check B"
    check: "function() { return true; }"
    true:
      action: "function() { context.done = true; }"
      terminate: true
    false:
      terminate: true
`
	library, valResult := createLibraryFromYAML(t, yamlRules, "concurrent-trace.yaml")
	assert.Empty(t, valResult.Errors)
	assert.Empty(t, valResult.Warnings)

	initCtx := map[string]any{}
	runner, err := NewRulesRunnerFromLibrary(library, "concurrent-trace", &initCtx)
	require.NoError(t, err)

	const numGoroutines = 10
	traces := make([][]TraceEntry, numGoroutines)
	errs := make([]error, numGoroutines)

	var wg sync.WaitGroup
	for i := range numGoroutines {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ctx := map[string]any{}
			_, traces[idx], errs[idx] = runner.RunRulesWithTrace(&ctx, nil)
		}(i)
	}
	wg.Wait()

	for i := range numGoroutines {
		assert.NoError(t, errs[i], "goroutine %d returned an error", i)
		require.Len(t, traces[i], 2, "goroutine %d trace length", i)
		assert.Equal(t, "checkA", traces[i][0].ConditionName, "goroutine %d entry 0", i)
		assert.Equal(t, "checkB", traces[i][1].ConditionName, "goroutine %d entry 1", i)
		assert.True(t, traces[i][1].HasAction, "goroutine %d entry 1 actionRan", i)
		assert.True(t, traces[i][1].Terminated, "goroutine %d entry 1 terminated", i)
	}
}

// ---------------------------------------------------------------------------
// Coexistence with WithDecisionCallback
// ---------------------------------------------------------------------------

// TestTrace_CoexistenceWithDecisionCallback verifies that RunRulesWithTrace
// and WithDecisionCallback can be used together: both the decision log and
// the trace are populated independently.
func TestTrace_CoexistenceWithDecisionCallback(t *testing.T) {
	yamlRules := `
name: "coexistence"
conditions:
  start:
    description: "Start condition"
    default: true
    check: "function() { return true; }"
    true:
      action: "function() { context.ok = true; }"
      terminate: true
`
	library, valResult := createLibraryFromYAML(t, yamlRules, "coexistence.yaml")
	assert.Empty(t, valResult.Errors)
	assert.Empty(t, valResult.Warnings)

	decisions := []string{}
	ctx := map[string]any{}
	runner, err := NewRulesRunnerFromLibrary(library, "coexistence", &ctx,
		WithDecisionCallback[map[string]any](func(msg string, args ...any) {
			decisions = append(decisions, fmt.Sprintf(msg, args...))
		}),
	)
	require.NoError(t, err)

	_, trace, err := runner.RunRulesWithTrace(&ctx, nil)
	require.NoError(t, err)

	// Trace must have been populated.
	require.Len(t, trace, 1)
	assert.Equal(t, "start", trace[0].ConditionName)
	assert.True(t, trace[0].Result)
	assert.True(t, trace[0].HasAction)
	assert.True(t, trace[0].Terminated)
	assert.Nil(t, trace[0].Error)

	// Decision callback must also have been called.
	require.NotEmpty(t, decisions)
	assert.Contains(t, decisions[0], "Evaluating condition: [start]")
}

// ---------------------------------------------------------------------------
// BRE multi-level ruleset
// ---------------------------------------------------------------------------

// TestTrace_BREMultiLevelRuleset exercises the multi-file BRE ruleset
// (test/bre) and confirms that the trace correctly records the two-condition
// dispatch path for ruleset1.
func TestTrace_BREMultiLevelRuleset(t *testing.T) {
	ruleLibrary, valResult, err := NewRulesLibrary(RulesLibrarySettings{
		BasePath:   "test/bre",
		FileSystem: testFs,
	})
	require.NoError(t, err)
	assertBREValidationWarnings(t, valResult)

	breCtx := &BreContext{RuleSet: "ruleset1"}
	runner, err := NewRulesRunnerFromLibrary(ruleLibrary, "main", breCtx)
	require.NoError(t, err)

	_, trace, err := runner.RunRulesWithTrace(breCtx, nil)
	require.NoError(t, err)

	// Dispatch path: check_for_ruleset1 (true) -> execute_ruleset1 (true, terminates).
	require.Len(t, trace, 2)

	assert.Equal(t, "check_for_ruleset1", trace[0].ConditionName)
	assert.True(t, trace[0].Result)
	assert.False(t, trace[0].HasAction)
	assert.Equal(t, "execute_ruleset1", trace[0].NextCondition)
	assert.False(t, trace[0].Terminated)
	assert.Nil(t, trace[0].Error)

	assert.Equal(t, "execute_ruleset1", trace[1].ConditionName)
	assert.True(t, trace[1].Result)
	assert.True(t, trace[1].HasAction)
	assert.Empty(t, trace[1].NextCondition)
	assert.True(t, trace[1].Terminated)
	assert.Nil(t, trace[1].Error)

	// Verify that ruleset2 and ruleset3 dispatch paths also produce correct traces.
	t.Run("Ruleset2", func(t *testing.T) {
		ctx2 := &BreContext{RuleSet: "ruleset2"}
		runner2, err := NewRulesRunnerFromLibrary(ruleLibrary, "main", ctx2)
		require.NoError(t, err)

		_, trace2, err := runner2.RunRulesWithTrace(ctx2, nil)
		require.NoError(t, err)

		// check_for_ruleset1 (false) -> check_for_ruleset2 (true) -> execute_ruleset2.
		require.Len(t, trace2, 3)
		assert.Equal(t, "check_for_ruleset1", trace2[0].ConditionName)
		assert.False(t, trace2[0].Result)
		assert.Equal(t, "check_for_ruleset2", trace2[0].NextCondition)

		assert.Equal(t, "check_for_ruleset2", trace2[1].ConditionName)
		assert.True(t, trace2[1].Result)
		assert.Equal(t, "execute_ruleset2", trace2[1].NextCondition)

		assert.Equal(t, "execute_ruleset2", trace2[2].ConditionName)
		assert.True(t, trace2[2].Result)
		assert.True(t, trace2[2].HasAction)
		assert.True(t, trace2[2].Terminated)
		assert.Nil(t, trace2[2].Error)
	})

	t.Run("Ruleset3DefaultFallthrough", func(t *testing.T) {
		ctx3 := &BreContext{RuleSet: ""}
		runner3, err := NewRulesRunnerFromLibrary(ruleLibrary, "main", ctx3)
		require.NoError(t, err)

		_, trace3, err := runner3.RunRulesWithTrace(ctx3, nil)
		require.NoError(t, err)

		// check_for_ruleset1 (false) -> check_for_ruleset2 (false) -> execute_ruleset3.
		require.Len(t, trace3, 3)
		assert.Equal(t, "check_for_ruleset1", trace3[0].ConditionName)
		assert.False(t, trace3[0].Result)

		assert.Equal(t, "check_for_ruleset2", trace3[1].ConditionName)
		assert.False(t, trace3[1].Result)
		assert.Equal(t, "execute_ruleset3", trace3[1].NextCondition)

		assert.Equal(t, "execute_ruleset3", trace3[2].ConditionName)
		assert.True(t, trace3[2].HasAction)
		assert.True(t, trace3[2].Terminated)
		assert.Nil(t, trace3[2].Error)
	})

	assertDurationsNonNegative(t, trace)
}

// ---------------------------------------------------------------------------
// Duration non-negative - dedicated assertion test
// ---------------------------------------------------------------------------

// TestTrace_DurationNonNegative explicitly verifies that every Duration in
// a multi-condition loan-approval trace is >= 0. This acts as a focused
// regression guard for the timing logic.
func TestTrace_DurationNonNegative(t *testing.T) {
	rl, valResult, err := NewRulesLibrary(RulesLibrarySettings{BasePath: "test", FileSystem: testFs})
	require.NoError(t, err)
	assertBREValidationWarnings(t, valResult)

	ctx := LoanContext{
		Applicants: []Applicant{
			{Type: "primary", Age: 25, Income: 5000, Debt: 1000, CreditScore: 750},
		},
		LoanAmount: 10000,
	}
	runner, err := NewRulesRunnerFromLibrary(rl, "loan-approval", &ctx)
	require.NoError(t, err)

	_, trace, err := runner.RunRulesWithTrace(&ctx, nil)
	require.NoError(t, err)
	require.NotEmpty(t, trace)

	for i, e := range trace {
		assert.GreaterOrEqual(t, e.Duration, time.Duration(0),
			"condition %d (%s) has negative duration %v", i, e.ConditionName, e.Duration)
	}
}

// ---------------------------------------------------------------------------
// RunRules backward-compatibility - unchanged behavior without trace
// ---------------------------------------------------------------------------

// TestTrace_RunRulesUnchangedBehavior confirms that RunRules (non-tracing
// variant) produces identical context output to RunRulesWithTrace. This is
// the backward-compatibility guard to ensure the nil-tc fast path is correct.
func TestTrace_RunRulesUnchangedBehavior(t *testing.T) {
	rl, valResult, err := NewRulesLibrary(RulesLibrarySettings{BasePath: "test", FileSystem: testFs})
	require.NoError(t, err)
	assertBREValidationWarnings(t, valResult)

	makeCtx := func() LoanContext {
		return LoanContext{
			Applicants: []Applicant{
				{Type: "primary", Age: 25, Income: 5000, Debt: 1000, CreditScore: 750},
				{Type: "co-applicant", Age: 30, Income: 4000, Debt: 500, CreditScore: 700},
			},
			LoanAmount: 40000,
		}
	}

	// RunRules path.
	ctx1 := makeCtx()
	runner1, err := NewRulesRunnerFromLibrary(rl, "loan-approval", &ctx1)
	require.NoError(t, err)
	resultCtx1, err := runner1.RunRules(&ctx1, nil)
	require.NoError(t, err)

	// RunRulesWithTrace path.
	ctx2 := makeCtx()
	runner2, err := NewRulesRunnerFromLibrary(rl, "loan-approval", &ctx2)
	require.NoError(t, err)
	resultCtx2, _, err := runner2.RunRulesWithTrace(&ctx2, nil)
	require.NoError(t, err)

	// The resulting contexts must be identical.
	assert.Equal(t, resultCtx1.Decision, resultCtx2.Decision)
	assert.Equal(t, resultCtx1.Reason, resultCtx2.Reason)
}
