package cli

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRuntime_UsesProcessStreamsByDefault(t *testing.T) {
	rt := NewRuntime(&RootOptions{})

	assert.Equal(t, os.Stdout, rt.Stdout)
	assert.Equal(t, os.Stderr, rt.Stderr)
}
