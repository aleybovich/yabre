package yabre

import (
	"embed"
	"io/fs"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

//go:embed test/*.yaml
var embeddedTestData embed.FS

func testInitRulesLibrary(t *testing.T, fileSystem fs.FS, basePath string) {
	rl, err := NewRulesLibrary(RulesLibrarySettings{FileSystem: fileSystem, BasePath: basePath})
	assert.NoError(t, err, "failed to initialize rules library")
	assert.NotNil(t, rl, "rules library is nil")

	assert.Contains(t, rl.rulePaths, "aliquoting-rules")
	assert.Contains(t, rl.rulePaths, "aliquoting-rules-scripts")

	deps, exists := rl.dependencies["aliquoting-rules"]
	assert.True(t, exists)
	assert.Equal(t, []string{"aliquoting-rules-scripts"}, deps)
}

func testLoadRules(t *testing.T, fileSystem fs.FS, basePath string) {
	rl, err := NewRulesLibrary(RulesLibrarySettings{FileSystem: fileSystem, BasePath: basePath})
	assert.NoError(t, err, "failed to initialize rules library")
	assert.NotNil(t, rl, "rules library is nil")

	rules, err := rl.LoadRules("aliquoting-rules")

	assert.NoError(t, err, "failed to load rules")
	assert.NotNil(t, rules, "rules is nil")
}

func TestInitRulesLibraryFromFileSystem(t *testing.T) {
	testInitRulesLibrary(t, os.DirFS("./test"), "")
	testInitRulesLibrary(t, nil, "./test")
}

func TestLoadRulesFromFileSystem(t *testing.T) {
	testLoadRules(t, os.DirFS("./test"), "")
	testLoadRules(t, nil, "./test")
}

func TestInitRulesLibraryFromEmbeddedFS(t *testing.T) {
	testInitRulesLibrary(t, embeddedTestData, "")
}

func TestLoadRulesFromEmbeddedFS(t *testing.T) {
	testLoadRules(t, embeddedTestData, "")
}

func TestInitRulesLibraryWithWrongFSPath(t *testing.T) {
	rl, err := NewRulesLibrary(RulesLibrarySettings{FileSystem: os.DirFS("./wrong")})
	assert.Error(t, err)
	assert.Equal(t, "failed to scan files: stat .: no such file or directory", err.Error())
	assert.Nil(t, rl, "rules library is not nil")
}
