package yabre

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInitRulesLibrary(t *testing.T) {
	rl, err := NewRulesLibrary(RulesLibrarySettings{BasePath: "./test"})
	assert.NoError(t, err, "failed to initialize rules library")
	assert.NotNil(t, rl, "rules library is nil")

	assert.Contains(t, rl.rulePaths, "aliquoting-rules")
	assert.Contains(t, rl.rulePaths, "aliquoting-rules-scripts")

	deps, exists := rl.dependencies["aliquoting-rules"]
	assert.True(t, exists)
	assert.Equal(t, []string{"aliquoting-rules-scripts"}, deps)
}

func TestLoadRules(t *testing.T) {
	rl, err := NewRulesLibrary(RulesLibrarySettings{BasePath: "./test"})
	assert.NoError(t, err, "failed to initialize rules library")
	assert.NotNil(t, rl, "rules library is nil")

	rules, err := rl.LoadRules("aliquoting-rules")

	assert.NoError(t, err, "failed to load rules")
	assert.NotNil(t, rules, "rules is nil")
}
