package yabre

import (
	_ "embed"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMermaid(t *testing.T) {
	yamlString, err := os.ReadFile("test/aliquoting_rules.yaml")
	assert.NoError(t, err, "error reading YAML file")

	mmd, err := ExportMermaid(yamlString, "")
	assert.NoError(t, err, "error converting to mermaid")

	fmt.Println(mmd)
}

func TestMermaid_SwitchCaseKeyEscaping(t *testing.T) {
	yamlData := []byte(`
name: escape-test
conditions:
  route:
    type: switch
    default: true
    description: Route by type
    check: |
      function() { return "a|b"; }
    cases:
      'a|b':
        description: Pipe case
        action: |
          function() { context.Route = "piped"; }
        terminate: true
      'say "hello"':
        description: Quote case
        terminate: true
      default:
        terminate: true
`)
	mmd, err := ExportMermaid(yamlData, "")
	assert.NoError(t, err)

	// Pipes should be escaped as HTML entities
	assert.Contains(t, mmd, "&#124;")
	assert.NotContains(t, mmd, "|a|b|")

	// Quotes should be escaped
	assert.Contains(t, mmd, "&quot;")
}

func TestExportMermaidFromLibrary(t *testing.T) {
	// Create a rules library from test data
	rl, valResult, err := NewRulesLibrary(RulesLibrarySettings{
		BasePath:   "test/bre",
		FileSystem: testFs,
	})
	assert.NoError(t, err)
	assertBREValidationWarnings(t, valResult)

	// Generate Mermaid diagram from the "main" ruleset
	mermaidCode, err := ExportMermaidFromLibrary(rl, "main", "check_for_ruleset1")
	assert.NoError(t, err)

	// Verify the mermaid code contains elements from all dependent rulesets
	assert.Contains(t, mermaidCode, "check_for_ruleset1")
	assert.Contains(t, mermaidCode, "check_for_ruleset2")
	assert.Contains(t, mermaidCode, "execute_ruleset1")
	assert.Contains(t, mermaidCode, "execute_ruleset2")
	assert.Contains(t, mermaidCode, "execute_ruleset3")

	// Check connections between conditions
	assert.Contains(t, mermaidCode, "check_for_ruleset1 --> |true| execute_ruleset1")
	assert.Contains(t, mermaidCode, "check_for_ruleset1 --> |false| check_for_ruleset2")
	assert.Contains(t, mermaidCode, "check_for_ruleset2 --> |true| execute_ruleset2")
	assert.Contains(t, mermaidCode, "check_for_ruleset2 --> |false| execute_ruleset3")
}
