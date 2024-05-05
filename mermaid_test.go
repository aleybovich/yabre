package main

import (
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
