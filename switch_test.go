package yabre

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// ─── SwitchCase UnmarshalYAML ────────────────────────────────────────────────

func TestSwitchCase_UnmarshalYAML_ValidNext(t *testing.T) {
	yamlData := `
description: "Medium risk"
action: "function() { context.Rate = 0.06; }"
next: "standard_review"
`
	var sc SwitchCase
	err := yaml.Unmarshal([]byte(yamlData), &sc)
	require.NoError(t, err)
	assert.Equal(t, "Medium risk", sc.Description)
	assert.Equal(t, "standard_review", sc.Next)
	assert.False(t, sc.Terminate)
}

func TestSwitchCase_UnmarshalYAML_ValidTerminate(t *testing.T) {
	yamlData := `
description: "Low risk - approve"
action: "function() { context.Decision = 'approved'; }"
terminate: true
`
	var sc SwitchCase
	err := yaml.Unmarshal([]byte(yamlData), &sc)
	require.NoError(t, err)
	assert.True(t, sc.Terminate)
	assert.Empty(t, sc.Next)
}

func TestSwitchCase_UnmarshalYAML_ErrorBothNextAndTerminate(t *testing.T) {
	yamlData := `
description: "Invalid"
next: "somewhere"
terminate: true
`
	var sc SwitchCase
	err := yaml.Unmarshal([]byte(yamlData), &sc)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "next and terminate cannot be used together")
}

// ─── Switch Condition Execution ──────────────────────────────────────────────

type switchTestCtx struct {
	ProductType string `json:"productType"`
	Route       string `json:"route"`
	Score       int    `json:"score"`
	Decision    string `json:"decision"`
	Rate        float64 `json:"rate"`
}

func TestSwitch_MatchedCase(t *testing.T) {
	yamlData := `
name: switch-test
conditions:
  classify:
    type: switch
    default: true
    description: Classify by product type
    check: |
      function() { return context.ProductType; }
    cases:
      powder:
        description: Powder route
        action: |
          function() { context.Route = "powder-line"; }
        terminate: true
      solution:
        description: Solution route
        action: |
          function() { context.Route = "solution-line"; }
        terminate: true
      default:
        description: Manual review
        action: |
          function() { context.Route = "manual-review"; }
        terminate: true
`
	ctx := switchTestCtx{ProductType: "powder"}
	runner := buildSwitchRunner(t, yamlData, &ctx)

	result, err := runner.RunRules(&ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, "powder-line", result.Route)
}

func TestSwitch_FallsToDefault(t *testing.T) {
	yamlData := `
name: switch-test
conditions:
  classify:
    type: switch
    default: true
    description: Classify by product type
    check: |
      function() { return context.ProductType; }
    cases:
      powder:
        description: Powder route
        action: |
          function() { context.Route = "powder-line"; }
        terminate: true
      default:
        description: Manual review
        action: |
          function() { context.Route = "manual-review"; }
        terminate: true
`
	ctx := switchTestCtx{ProductType: "unknown_type"}
	runner := buildSwitchRunner(t, yamlData, &ctx)

	result, err := runner.RunRules(&ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, "manual-review", result.Route)
}

func TestSwitch_NoMatchNoDefault_TerminatesSilently(t *testing.T) {
	yamlData := `
name: switch-test
conditions:
  classify:
    type: switch
    default: true
    description: Classify
    check: |
      function() { return context.ProductType; }
    cases:
      powder:
        description: Powder
        action: |
          function() { context.Route = "powder-line"; }
        terminate: true
`
	ctx := switchTestCtx{ProductType: "unknown_type"}
	runner := buildSwitchRunner(t, yamlData, &ctx)

	result, err := runner.RunRules(&ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, "", result.Route) // No route set
}

func TestSwitch_CaseChainToNext(t *testing.T) {
	yamlData := `
name: switch-test
conditions:
  classify:
    type: switch
    default: true
    description: Classify risk
    check: |
      function() {
        if (context.Score >= 750) return "low";
        return "high";
      }
    cases:
      low:
        description: Low risk - approve
        action: |
          function() { context.Decision = "approved"; context.Rate = 0.04; }
        terminate: true
      high:
        description: High risk - review
        next: detailed_review
  detailed_review:
    description: Detailed review
    check: |
      function() { return context.Score >= 550; }
    true:
      action: |
        function() { context.Decision = "conditional"; }
      terminate: true
    false:
      action: |
        function() { context.Decision = "rejected"; }
      terminate: true
`
	ctx := switchTestCtx{Score: 600}
	runner := buildSwitchRunner(t, yamlData, &ctx)

	result, err := runner.RunRules(&ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, "conditional", result.Decision)
}

func TestSwitch_CaseWithActionAndNext(t *testing.T) {
	yamlData := `
name: switch-test
conditions:
  classify:
    type: switch
    default: true
    description: Classify risk
    check: |
      function() { return "medium"; }
    cases:
      medium:
        description: Medium - set rate then review
        action: |
          function() { context.Rate = 0.06; }
        next: final_check
  final_check:
    description: Final check
    check: |
      function() { return true; }
    true:
      action: |
        function() { context.Decision = "approved"; }
      terminate: true
`
	ctx := switchTestCtx{}
	runner := buildSwitchRunner(t, yamlData, &ctx)

	result, err := runner.RunRules(&ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, 0.06, result.Rate)
	assert.Equal(t, "approved", result.Decision)
}

// ─── Switch Condition Trace ──────────────────────────────────────────────────

func TestSwitch_Trace_MatchedCase(t *testing.T) {
	yamlData := `
name: switch-test
conditions:
  classify:
    type: switch
    default: true
    description: Classify by type
    check: |
      function() { return context.ProductType; }
    cases:
      powder:
        description: Powder
        action: |
          function() { context.Route = "powder-line"; }
        terminate: true
      default:
        description: Default
        action: |
          function() { context.Route = "manual"; }
        terminate: true
`
	ctx := switchTestCtx{ProductType: "powder"}
	runner := buildSwitchRunner(t, yamlData, &ctx)

	_, trace, err := runner.RunRulesWithTrace(&ctx, nil)
	require.NoError(t, err)
	require.Len(t, trace, 1)

	assert.Equal(t, "classify", trace[0].ConditionName)
	assert.Equal(t, "powder", trace[0].SwitchResult)
	assert.False(t, trace[0].Result) // Result is unused for switch
	assert.True(t, trace[0].HasAction)
	assert.True(t, trace[0].Terminated)
	assert.Empty(t, trace[0].NextCondition)
	assert.NoError(t, trace[0].Error)
}

func TestSwitch_Trace_DefaultFallthrough(t *testing.T) {
	yamlData := `
name: switch-test
conditions:
  classify:
    type: switch
    default: true
    description: Classify
    check: |
      function() { return context.ProductType; }
    cases:
      powder:
        description: Powder
        terminate: true
      default:
        description: Default
        action: |
          function() { context.Route = "manual"; }
        terminate: true
`
	ctx := switchTestCtx{ProductType: "unknown"}
	runner := buildSwitchRunner(t, yamlData, &ctx)

	_, trace, err := runner.RunRulesWithTrace(&ctx, nil)
	require.NoError(t, err)
	require.Len(t, trace, 1)

	assert.Equal(t, "unknown", trace[0].SwitchResult)
	assert.True(t, trace[0].HasAction)
	assert.True(t, trace[0].Terminated)
}

func TestSwitch_Trace_NoMatch(t *testing.T) {
	yamlData := `
name: switch-test
conditions:
  classify:
    type: switch
    default: true
    description: Classify
    check: |
      function() { return context.ProductType; }
    cases:
      powder:
        terminate: true
`
	ctx := switchTestCtx{ProductType: "vapor"}
	runner := buildSwitchRunner(t, yamlData, &ctx)

	_, trace, err := runner.RunRulesWithTrace(&ctx, nil)
	require.NoError(t, err)
	require.Len(t, trace, 1)

	assert.Equal(t, "vapor", trace[0].SwitchResult)
	assert.True(t, trace[0].Terminated)
	assert.False(t, trace[0].HasAction)
}

func TestSwitch_Trace_ChainToBinaryCondition(t *testing.T) {
	yamlData := `
name: switch-test
conditions:
  classify:
    type: switch
    default: true
    description: Classify
    check: |
      function() { return "high"; }
    cases:
      high:
        description: High risk
        next: review
  review:
    description: Review
    check: |
      function() { return true; }
    true:
      action: |
        function() { context.Decision = "reviewed"; }
      terminate: true
`
	ctx := switchTestCtx{}
	runner := buildSwitchRunner(t, yamlData, &ctx)

	_, trace, err := runner.RunRulesWithTrace(&ctx, nil)
	require.NoError(t, err)
	require.Len(t, trace, 2)

	assert.Equal(t, "classify", trace[0].ConditionName)
	assert.Equal(t, "high", trace[0].SwitchResult)
	assert.Equal(t, "review", trace[0].NextCondition)
	assert.False(t, trace[0].Terminated)

	assert.Equal(t, "review", trace[1].ConditionName)
	assert.True(t, trace[1].Result)
	assert.Empty(t, trace[1].SwitchResult)
	assert.True(t, trace[1].Terminated)
}

// ─── Switch Condition Validation ─────────────────────────────────────────────

func TestValidate_SwitchWithTrueBranch(t *testing.T) {
	rules := &Rules{
		Conditions: map[string]Condition{
			"cond": {
				Name:  "cond",
				Type:  ConditionTypeSwitch,
				Check: "function() { return 'a'; }",
				True:  &Decision{Terminate: true},
				Cases: map[string]*SwitchCase{
					"a": {Name: "cond_case_a", CaseKey: "a", Terminate: true},
				},
			},
		},
	}

	issues := ValidateRules(rules)
	found := false
	for _, issue := range issues {
		if issue.Severity == SeverityError && issue.ConditionName == "cond" {
			if assert.Contains(t, issue.Message, "must not have 'true' or 'false' branches") {
				found = true
			}
		}
	}
	assert.True(t, found, "expected validation error for switch with true branch")
}

func TestValidate_SwitchNoCases(t *testing.T) {
	rules := &Rules{
		Conditions: map[string]Condition{
			"cond": {
				Name:  "cond",
				Type:  ConditionTypeSwitch,
				Check: "function() { return 'a'; }",
				Cases: map[string]*SwitchCase{},
			},
		},
	}

	issues := ValidateRules(rules)
	found := false
	for _, issue := range issues {
		if issue.Severity == SeverityError && issue.ConditionName == "cond" {
			if assert.Contains(t, issue.Message, "must have at least one case") {
				found = true
			}
		}
	}
	assert.True(t, found, "expected validation error for switch with no cases")
}

func TestValidate_SwitchNoDefaultCase_Warning(t *testing.T) {
	rules := &Rules{
		Conditions: map[string]Condition{
			"cond": {
				Name:    "cond",
				Type:    ConditionTypeSwitch,
				Default: true,
				Check:   "function() { return 'a'; }",
				Cases: map[string]*SwitchCase{
					"a": {Name: "cond_case_a", CaseKey: "a", Terminate: true},
				},
			},
		},
		DefaultCondition: &Condition{Name: "cond"},
	}

	issues := ValidateRules(rules)
	found := false
	for _, issue := range issues {
		if issue.Severity == SeverityWarning && issue.ConditionName == "cond" {
			if assert.Contains(t, issue.Message, "no 'default' case") {
				found = true
			}
		}
	}
	assert.True(t, found, "expected warning for switch with no default case")
}

func TestValidate_SwitchDanglingCaseNext(t *testing.T) {
	rules := &Rules{
		Conditions: map[string]Condition{
			"cond": {
				Name:    "cond",
				Type:    ConditionTypeSwitch,
				Default: true,
				Check:   "function() { return 'a'; }",
				Cases: map[string]*SwitchCase{
					"a":       {Name: "cond_case_a", CaseKey: "a", Next: "nonexistent"},
					"default": {Name: "cond_case_default", CaseKey: "default", Terminate: true},
				},
			},
		},
		DefaultCondition: &Condition{Name: "cond"},
	}

	issues := ValidateRules(rules)
	found := false
	for _, issue := range issues {
		if issue.Severity == SeverityError && issue.ConditionName == "cond" {
			if assert.Contains(t, issue.Message, "references non-existent condition 'nonexistent'") {
				found = true
			}
		}
	}
	assert.True(t, found, "expected error for dangling switch case next reference")
}

func TestValidate_SwitchCycleThroughCaseNext(t *testing.T) {
	rules := &Rules{
		Conditions: map[string]Condition{
			"cond": {
				Name:    "cond",
				Type:    ConditionTypeSwitch,
				Default: true,
				Check:   "function() { return 'a'; }",
				Cases: map[string]*SwitchCase{
					"a":       {Name: "cond_case_a", CaseKey: "a", Next: "cond"},
					"default": {Name: "cond_case_default", CaseKey: "default", Terminate: true},
				},
			},
		},
		DefaultCondition: &Condition{Name: "cond"},
	}

	issues := ValidateRules(rules)
	found := false
	for _, issue := range issues {
		if issue.Severity == SeverityError {
			if assert.Contains(t, issue.Message, "circular dependency") {
				found = true
			}
		}
	}
	assert.True(t, found, "expected cycle detection through switch case next")
}

func TestValidate_NonSwitchWithCases(t *testing.T) {
	rules := &Rules{
		Conditions: map[string]Condition{
			"cond": {
				Name:    "cond",
				Default: true,
				Check:   "function() { return true; }",
				True:    &Decision{Terminate: true},
				False:   &Decision{Terminate: true},
				Cases: map[string]*SwitchCase{
					"a": {Name: "cond_case_a", CaseKey: "a", Terminate: true},
				},
			},
		},
		DefaultCondition: &Condition{Name: "cond"},
	}

	issues := ValidateRules(rules)
	found := false
	for _, issue := range issues {
		if issue.Severity == SeverityError && issue.ConditionName == "cond" {
			if assert.Contains(t, issue.Message, "'cases' is only valid on switch conditions") {
				found = true
			}
		}
	}
	assert.True(t, found, "expected error for non-switch condition with cases")
}

func TestValidate_UnknownConditionType(t *testing.T) {
	rules := &Rules{
		Conditions: map[string]Condition{
			"cond": {
				Name:    "cond",
				Default: true,
				Type:    ConditionType("invalid"),
				Check:   "function() { return true; }",
				True:    &Decision{Terminate: true},
			},
		},
		DefaultCondition: &Condition{Name: "cond"},
	}

	issues := ValidateRules(rules)
	found := false
	for _, issue := range issues {
		if issue.Severity == SeverityError && issue.ConditionName == "cond" {
			if assert.Contains(t, issue.Message, "unknown condition type 'invalid'") {
				found = true
			}
		}
	}
	assert.True(t, found, "expected error for unknown condition type")
}

func TestValidate_SwitchCaseMakesConditionReachable(t *testing.T) {
	rules := &Rules{
		Conditions: map[string]Condition{
			"entry": {
				Name:    "entry",
				Type:    ConditionTypeSwitch,
				Default: true,
				Check:   "function() { return 'a'; }",
				Cases: map[string]*SwitchCase{
					"a":       {Name: "entry_case_a", CaseKey: "a", Next: "target"},
					"default": {Name: "entry_case_default", CaseKey: "default", Terminate: true},
				},
			},
			"target": {
				Name:  "target",
				Check: "function() { return true; }",
				True:  &Decision{Terminate: true},
				False: &Decision{Terminate: true},
			},
		},
		DefaultCondition: &Condition{Name: "entry"},
	}

	issues := ValidateRules(rules)
	// "target" should NOT be flagged as unreachable
	for _, issue := range issues {
		if issue.ConditionName == "target" && issue.Severity == SeverityWarning {
			t.Errorf("target should be reachable via switch case next, but got warning: %s", issue.Message)
		}
	}
}

func TestValidate_SwitchCollidingSanitizedKeys(t *testing.T) {
	rules := &Rules{
		Conditions: map[string]Condition{
			"cond": {
				Name:    "cond",
				Type:    ConditionTypeSwitch,
				Default: true,
				Check:   "function() { return 'a-b'; }",
				Cases: map[string]*SwitchCase{
					"a-b":     {Name: "cond_case_a_b", CaseKey: "a-b", Terminate: true},
					"a/b":     {Name: "cond_case_a_b", CaseKey: "a/b", Terminate: true},
					"default": {Name: "cond_case_default", CaseKey: "default", Terminate: true},
				},
			},
		},
		DefaultCondition: &Condition{Name: "cond"},
	}

	issues := ValidateRules(rules)
	found := false
	for _, issue := range issues {
		if issue.Severity == SeverityError && issue.ConditionName == "cond" {
			if assert.Contains(t, issue.Message, "collide after sanitization") {
				found = true
			}
		}
	}
	assert.True(t, found, "expected error for colliding sanitized case keys")
}

func TestValidate_SwitchNonCollidingSpecialKeys(t *testing.T) {
	rules := &Rules{
		Conditions: map[string]Condition{
			"cond": {
				Name:    "cond",
				Type:    ConditionTypeSwitch,
				Default: true,
				Check:   "function() { return 'a-b'; }",
				Cases: map[string]*SwitchCase{
					"a-b":     {Name: "cond_case_a_b", CaseKey: "a-b", Terminate: true},
					"a-c":     {Name: "cond_case_a_c", CaseKey: "a-c", Terminate: true},
					"default": {Name: "cond_case_default", CaseKey: "default", Terminate: true},
				},
			},
		},
		DefaultCondition: &Condition{Name: "cond"},
	}

	issues := ValidateRules(rules)
	for _, issue := range issues {
		if issue.Severity == SeverityError && issue.ConditionName == "cond" {
			assert.NotContains(t, issue.Message, "collide after sanitization",
				"should not report collision for distinct sanitized keys")
		}
	}
}

// ─── Switch YAML Parsing Integration ─────────────────────────────────────────

func TestSwitch_YAMLParsing(t *testing.T) {
	yamlData := `
name: switch-rules
conditions:
  classify:
    type: switch
    default: true
    description: Classify risk
    check: |
      function() { return "low"; }
    cases:
      low:
        description: Low risk
        action: |
          function() { context.Decision = "approved"; }
        terminate: true
      high:
        description: High risk
        next: review
      default:
        description: Default
        terminate: true
  review:
    description: Review step
    check: |
      function() { return true; }
    true:
      terminate: true
`
	var rules Rules
	err := yaml.Unmarshal([]byte(yamlData), &rules)
	require.NoError(t, err)

	classify := rules.Conditions["classify"]
	assert.Equal(t, ConditionTypeSwitch, classify.Type)
	assert.True(t, classify.IsSwitch())
	assert.Len(t, classify.Cases, 3)
	assert.Equal(t, "classify_case_low", classify.Cases["low"].Name)
	assert.Equal(t, "low", classify.Cases["low"].CaseKey)
	assert.Equal(t, "classify_case_high", classify.Cases["high"].Name)
	assert.Equal(t, "review", classify.Cases["high"].Next)
	assert.True(t, classify.Cases["default"].Terminate)
}

// ─── Unmarshal-time rejection ────────────────────────────────────────────────

func TestSwitch_UnmarshalRejects_NilCase(t *testing.T) {
	yamlData := `
name: bad
conditions:
  cond:
    type: switch
    default: true
    check: |
      function() { return "a"; }
    cases:
      a:
        terminate: true
      default:
`
	var rules Rules
	err := yaml.Unmarshal([]byte(yamlData), &rules)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "switch case 'default' is empty (null)")
}

func TestSwitch_UnmarshalRejects_ExplicitNullCase(t *testing.T) {
	yamlData := `
name: bad
conditions:
  cond:
    type: switch
    default: true
    check: |
      function() { return "a"; }
    cases:
      a:
        terminate: true
      fallback: ~
`
	var rules Rules
	err := yaml.Unmarshal([]byte(yamlData), &rules)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "switch case 'fallback' is empty (null)")
}

func TestSwitch_UnmarshalRejects_SwitchWithTrueBranch(t *testing.T) {
	yamlData := `
name: bad
conditions:
  cond:
    type: switch
    default: true
    check: |
      function() { return "a"; }
    true:
      terminate: true
    cases:
      a:
        terminate: true
`
	var rules Rules
	err := yaml.Unmarshal([]byte(yamlData), &rules)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "switch conditions must not have 'true' or 'false' branches")
}

func TestSwitch_UnmarshalRejects_BoolWithCases(t *testing.T) {
	yamlData := `
name: bad
conditions:
  cond:
    default: true
    check: |
      function() { return true; }
    true:
      terminate: true
    cases:
      a:
        terminate: true
`
	var rules Rules
	err := yaml.Unmarshal([]byte(yamlData), &rules)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "'cases' is only valid on switch conditions")
}

func TestSwitch_UnmarshalRejects_UnknownType(t *testing.T) {
	yamlData := `
name: bad
conditions:
  cond:
    type: fancy
    default: true
    check: |
      function() { return true; }
    true:
      terminate: true
`
	var rules Rules
	err := yaml.Unmarshal([]byte(yamlData), &rules)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown condition type 'fancy'")
}

// ─── Switch Case Key Sanitization ────────────────────────────────────────────

func TestSwitch_HyphenatedCaseKey(t *testing.T) {
	yamlData := `
name: switch-test
conditions:
  classify:
    type: switch
    default: true
    description: Classify by product type
    check: |
      function() { return context.ProductType; }
    cases:
      two-spares:
        description: Two spares route
        action: |
          function() { context.Route = "two-spares-line"; }
        terminate: true
      manual review:
        description: Manual review route
        action: |
          function() { context.Route = "manual-review-line"; }
        terminate: true
      default:
        description: Default
        action: |
          function() { context.Route = "default-line"; }
        terminate: true
`
	ctx := switchTestCtx{ProductType: "two-spares"}
	runner := buildSwitchRunner(t, yamlData, &ctx)

	result, err := runner.RunRules(&ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, "two-spares-line", result.Route)
}

func TestSwitch_SpacedCaseKey(t *testing.T) {
	yamlData := `
name: switch-test
conditions:
  classify:
    type: switch
    default: true
    description: Classify by product type
    check: |
      function() { return context.ProductType; }
    cases:
      manual review:
        description: Manual review route
        action: |
          function() { context.Route = "manual-review-line"; }
        terminate: true
      default:
        description: Default
        action: |
          function() { context.Route = "default-line"; }
        terminate: true
`
	ctx := switchTestCtx{ProductType: "manual review"}
	runner := buildSwitchRunner(t, yamlData, &ctx)

	result, err := runner.RunRules(&ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, "manual-review-line", result.Route)
}

func TestSwitch_SpecialCharsCaseKey(t *testing.T) {
	yamlData := `
name: switch-test
conditions:
  classify:
    type: switch
    default: true
    description: Classify
    check: |
      function() { return context.ProductType; }
    cases:
      "type.A/1":
        description: Type A
        action: |
          function() { context.Route = "type-a"; }
        terminate: true
      default:
        terminate: true
`
	ctx := switchTestCtx{ProductType: "type.A/1"}
	runner := buildSwitchRunner(t, yamlData, &ctx)

	result, err := runner.RunRules(&ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, "type-a", result.Route)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func buildSwitchRunner(t *testing.T, yamlData string, ctx *switchTestCtx) *RulesRunner[switchTestCtx] {
	t.Helper()
	var rules Rules
	err := yaml.Unmarshal([]byte(yamlData), &rules)
	require.NoError(t, err)

	runner := &RulesRunner[switchTestCtx]{
		Context:          ctx,
		Rules:            &rules,
		decisionCallback: func(msg string, args ...any) {},
	}
	return runner
}
