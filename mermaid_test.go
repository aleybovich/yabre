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

func TestExportMermaidFromLibrary(t *testing.T) {
	// Create a rules library from test data
	rl, err := NewRulesLibrary(RulesLibrarySettings{
		BasePath:   "test/bre",
		FileSystem: testFs,
	})
	assert.NoError(t, err)

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
