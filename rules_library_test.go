package yabre

import (
	"embed"
	"io/fs"
	"os"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:embed test/*.yaml
var embeddedTestData embed.FS

func assertNoValidationIssues(t *testing.T, valResult ValidationResult) {
	t.Helper()
	assert.Empty(t, valResult.Errors, "validation errors found in rules library")
	assert.Empty(t, valResult.Warnings, "validation warnings found in rules library")
}

func assertBREValidationWarnings(t *testing.T, valResult ValidationResult) {
	t.Helper()
	assert.Empty(t, valResult.Errors, "validation errors found in rules library")
	assert.Len(t, valResult.Warnings, 3, "validation warnings found in rules library")

	conditionNames := make([]string, 0, len(valResult.Warnings))
	for _, issue := range valResult.Warnings {
		assert.Equal(t, SeverityWarning, issue.Severity)
		assert.Contains(t, issue.Message, "condition is unreachable")
		conditionNames = append(conditionNames, issue.ConditionName)
	}

	assert.ElementsMatch(t, []string{
		"ruleset1.execute_ruleset1",
		"ruleset2.execute_ruleset2",
		"ruleset3.execute_ruleset3",
	}, conditionNames)
}

func assertValidationIssueContains(t *testing.T, issues ValidationIssues, want string) {
	t.Helper()
	for _, issue := range issues {
		if assert.Contains(t, issue.Message, want) {
			return
		}
	}
	t.Fatalf("expected validation issue containing %q, got %s", want, issues.String())
}

func testInitRulesLibrary(t *testing.T, fileSystem fs.FS, basePath string, expectBREWarnings bool) {
	rl, valResult, err := NewRulesLibrary(RulesLibrarySettings{FileSystem: fileSystem, BasePath: basePath})
	assert.NoError(t, err, "failed to initialize rules library")
	if expectBREWarnings {
		assertBREValidationWarnings(t, valResult)
	} else {
		assertNoValidationIssues(t, valResult)
	}

	assert.NotNil(t, rl, "rules library is nil")

	assert.Contains(t, rl.rulePaths, "aliquoting-rules")
	assert.Contains(t, rl.rulePaths, "aliquoting-rules-scripts")

	deps, exists := rl.dependencies["aliquoting-rules"]
	assert.True(t, exists)
	assert.Equal(t, []string{"aliquoting-rules-scripts"}, deps)
}

func testLoadRules(t *testing.T, fileSystem fs.FS, basePath string, expectBREWarnings bool) {
	rl, valResult, err := NewRulesLibrary(RulesLibrarySettings{FileSystem: fileSystem, BasePath: basePath})
	assert.NoError(t, err, "failed to initialize rules library")
	if expectBREWarnings {
		assertBREValidationWarnings(t, valResult)
	} else {
		assertNoValidationIssues(t, valResult)
	}
	assert.NotNil(t, rl, "rules library is nil")

	rules, err := rl.LoadRules("aliquoting-rules")

	assert.NoError(t, err, "failed to load rules")
	assert.NotNil(t, rules, "rules is nil")
}

func TestInitRulesLibraryFromFileSystem(t *testing.T) {
	testInitRulesLibrary(t, os.DirFS("./test"), "", true)
	testInitRulesLibrary(t, nil, "./test", true)
}

func TestLoadRulesFromFileSystem(t *testing.T) {
	testLoadRules(t, os.DirFS("./test"), "", true)
	testLoadRules(t, nil, "./test", true)
}

func TestInitRulesLibraryFromEmbeddedFS(t *testing.T) {
	testInitRulesLibrary(t, embeddedTestData, "", false)
}

func TestLoadRulesFromEmbeddedFS(t *testing.T) {
	testLoadRules(t, embeddedTestData, "", false)
}

func TestInitRulesLibraryWithWrongFSPath(t *testing.T) {
	rl, valResult, err := NewRulesLibrary(RulesLibrarySettings{FileSystem: os.DirFS("./wrong")})
	assert.Error(t, err)
	assert.Equal(t, "failed to scan files: stat .: no such file or directory", err.Error())
	assert.Nil(t, rl, "rules library is not nil")
	assert.Len(t, valResult.Errors, 0, "validation errors found in rules library")
	assert.Len(t, valResult.Warnings, 0, "validation warnings found in rules library")
}

func TestRulesAreCachedAtInit(t *testing.T) {
	rl, valResult, err := NewRulesLibrary(RulesLibrarySettings{FileSystem: os.DirFS("./test")})
	assert.NoError(t, err)
	assertBREValidationWarnings(t, valResult)

	// All rule sets should be cached after init
	for name := range rl.rulePaths {
		assert.Contains(t, rl.rules, name, "rule set %s should be cached", name)
		assert.NotNil(t, rl.rules[name], "cached rule set %s should not be nil", name)
	}
}

func TestLoadRulesDoesNotMutateCache(t *testing.T) {
	rl, valResult, err := NewRulesLibrary(RulesLibrarySettings{FileSystem: os.DirFS("./test")})
	assert.NoError(t, err)
	assertBREValidationWarnings(t, valResult)

	// Get the cached conditions count before loading
	cachedRules := rl.rules["aliquoting-rules"]
	originalConditionCount := len(cachedRules.Conditions)
	originalScripts := cachedRules.Scripts

	// Load rules (which merges dependencies)
	loaded, err := rl.LoadRules("aliquoting-rules")
	assert.NoError(t, err)
	assert.NotNil(t, loaded)

	// The loaded result should have more content (merged deps)
	// but the cache should remain unchanged
	assert.Equal(t, originalConditionCount, len(cachedRules.Conditions),
		"cache was mutated: condition count changed")
	assert.Equal(t, originalScripts, cachedRules.Scripts,
		"cache was mutated: scripts changed")
}

func TestLoadRulesMultipleCallsReturnIndependentCopies(t *testing.T) {
	rl, valResult, err := NewRulesLibrary(RulesLibrarySettings{FileSystem: os.DirFS("./test")})
	assert.NoError(t, err)
	assertBREValidationWarnings(t, valResult)

	rules1, err := rl.LoadRules("aliquoting-rules")
	assert.NoError(t, err)

	rules2, err := rl.LoadRules("aliquoting-rules")
	assert.NoError(t, err)

	// They should be equal in content
	assert.Equal(t, len(rules1.Conditions), len(rules2.Conditions))

	// But they should be different pointers (independent copies)
	assert.NotSame(t, rules1, rules2)
}

func TestLoadRulesNonExistentReturnsError(t *testing.T) {
	rl, valResult, err := NewRulesLibrary(RulesLibrarySettings{FileSystem: os.DirFS("./test")})
	assert.NoError(t, err)
	assertBREValidationWarnings(t, valResult)

	_, err = rl.LoadRules("does-not-exist")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestGetRuleNamesAndPaths(t *testing.T) {
	rl, valResult, err := NewRulesLibrary(RulesLibrarySettings{FileSystem: os.DirFS("./test")})
	assert.NoError(t, err)
	assertBREValidationWarnings(t, valResult)
	assert.NotNil(t, rl, "rules library is nil")

	m := rl.GetRuleNamesAndPaths()

	// test a few
	path, ok := m["go-rules"]
	assert.True(t, ok, "rule name seems missing")
	assert.Equal(t, "go_rules.yaml", path)

	path, ok = m["loan-approval"]
	assert.True(t, ok, "rule name seems missing")
	assert.Equal(t, "loan_approval.yaml", path)
}

// --- ValidateOnLoad tests ---

func TestValidateOnLoad_Disabled_NoValidation(t *testing.T) {
	// A rule set with circular deps should load fine when validation is off
	circularFS := fstest.MapFS{
		"circular.yaml": &fstest.MapFile{
			Data: []byte(`
name: circular-rules
conditions:
  cond_a:
    default: true
    check: "function() { return true; }"
    true:
      next: cond_b
  cond_b:
    check: "function() { return true; }"
    true:
      next: cond_a
`),
		},
	}

	rl, valResult, err := NewRulesLibrary(RulesLibrarySettings{
		FileSystem: circularFS,
	})
	assert.NoError(t, err)
	assert.NotNil(t, rl)
	assert.NotEmpty(t, valResult.Errors, "validation errors should be reported")
	assert.Contains(t, valResult.Errors[0].Message, "circular dependency detected")
	assert.Empty(t, valResult.Warnings)
}

func TestValidateOnLoad_CircularConditions_Fatal(t *testing.T) {
	circularFS := fstest.MapFS{
		"circular.yaml": &fstest.MapFile{
			Data: []byte(`
name: circular-rules
conditions:
  cond_a:
    default: true
    check: "function() { return true; }"
    true:
      next: cond_b
  cond_b:
    check: "function() { return true; }"
    true:
      next: cond_a
`),
		},
	}

	rl, valResult, err := NewRulesLibrary(RulesLibrarySettings{
		FileSystem: circularFS,
	})
	require.NoError(t, err)
	assert.NotNil(t, rl)
	assert.NotEmpty(t, valResult.Errors, "validation errors should be reported")
	assertValidationIssueContains(t, valResult.Errors, "circular dependency detected")
}

func TestValidateOnLoad_SelfReference_Fatal(t *testing.T) {
	selfRefFS := fstest.MapFS{
		"selfref.yaml": &fstest.MapFile{
			Data: []byte(`
name: self-ref-rules
conditions:
  loop:
    default: true
    check: "function() { return true; }"
    true:
      next: loop
`),
		},
	}

	rl, valResult, err := NewRulesLibrary(RulesLibrarySettings{
		FileSystem: selfRefFS,
	})
	require.NoError(t, err)
	assert.NotNil(t, rl)
	assert.NotEmpty(t, valResult.Errors, "validation errors should be reported")
	assertValidationIssueContains(t, valResult.Errors, "circular dependency detected")
}

func TestValidateOnLoad_DanglingReference_Fatal(t *testing.T) {
	danglingFS := fstest.MapFS{
		"dangling.yaml": &fstest.MapFile{
			Data: []byte(`
name: dangling-rules
conditions:
  start:
    default: true
    check: "function() { return true; }"
    true:
      next: nonexistent_condition
`),
		},
	}

	rl, valResult, err := NewRulesLibrary(RulesLibrarySettings{
		FileSystem: danglingFS,
	})
	require.NoError(t, err)
	assert.NotNil(t, rl)
	assert.NotEmpty(t, valResult.Errors, "validation errors should be reported")
	assertValidationIssueContains(t, valResult.Errors, "non-existent condition")
}

func TestValidateOnLoad_UnreachableCondition_Warning(t *testing.T) {
	unreachableFS := fstest.MapFS{
		"unreachable.yaml": &fstest.MapFile{
			Data: []byte(`
name: unreachable-rules
conditions:
  start:
    default: true
    check: "function() { return true; }"
    true:
      terminate: true
    false:
      terminate: true
  orphan:
    check: "function() { return true; }"
    true:
      terminate: true
`),
		},
	}

	rl, valResult, err := NewRulesLibrary(RulesLibrarySettings{
		FileSystem: unreachableFS,
	})
	require.NoError(t, err, "unreachable conditions should not cause fatal error")
	require.NotNil(t, rl)
	require.NotEmpty(t, valResult.Warnings)
	assert.Equal(t, SeverityWarning, valResult.Warnings[0].Severity)
	assert.Contains(t, valResult.Warnings[0].Message, "unreachable")
}

func TestValidateOnLoad_CircularRequireDeps_Fatal(t *testing.T) {
	circularRequireFS := fstest.MapFS{
		"a.yaml": &fstest.MapFile{
			Data: []byte(`
name: rule-a
require:
  - rule-b
conditions:
  check_a:
    default: true
    check: "function() { return true; }"
    true:
      terminate: true
`),
		},
		"b.yaml": &fstest.MapFile{
			Data: []byte(`
name: rule-b
require:
  - rule-a
conditions:
  check_b:
    default: true
    check: "function() { return true; }"
    true:
      terminate: true
`),
		},
	}

	rl, valResult, err := NewRulesLibrary(RulesLibrarySettings{
		FileSystem: circularRequireFS,
	})
	require.NoError(t, err)
	assert.NotNil(t, rl)
	assert.NotEmpty(t, valResult.Errors, "validation errors should be reported")
	assertValidationIssueContains(t, valResult.Errors, "circular require dependency")
	assert.Len(t, valResult.Warnings, 2)
	assert.Contains(t, valResult.Warnings[0].Message, "condition is unreachable")
	assert.Contains(t, valResult.Warnings[1].Message, "condition is unreachable")
	assert.ElementsMatch(t, []string{"rule-a.check_b", "rule-b.check_a"}, []string{
		valResult.Warnings[0].ConditionName,
		valResult.Warnings[1].ConditionName,
	})
}

func TestValidateOnLoad_ValidRules_NoWarnings(t *testing.T) {
	validFS := fstest.MapFS{
		"valid.yaml": &fstest.MapFile{
			Data: []byte(`
name: valid-rules
conditions:
  start:
    default: true
    check: "function() { return true; }"
    true:
      next: step2
    false:
      terminate: true
  step2:
    check: "function() { return true; }"
    true:
      terminate: true
    false:
      terminate: true
`),
		},
	}

	rl, valResult, err := NewRulesLibrary(RulesLibrarySettings{
		FileSystem: validFS,
	})
	require.NoError(t, err)
	require.NotNil(t, rl)
	assert.Empty(t, valResult.Warnings)
}

func TestValidateOnLoad_ExistingTestRules_Pass(t *testing.T) {
	// The existing test fixtures should pass validation
	rl, valResult, err := NewRulesLibrary(RulesLibrarySettings{
		FileSystem: os.DirFS("./test"),
	})
	require.NoError(t, err)
	require.NotNil(t, rl)
	assertBREValidationWarnings(t, valResult)
}
