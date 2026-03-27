package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRootCmd_Help(t *testing.T) {
	opts := &RootOptions{}
	cmd := NewRootCmd(opts)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "ottermqadmin")
	assert.Contains(t, output, "command-line administration tool for OtterMQ")
	assert.Contains(t, output, "--url")
	assert.Contains(t, output, "--username")
	assert.Contains(t, output, "--password")
	assert.Contains(t, output, "--token")
	assert.Contains(t, output, "--json")
	assert.Contains(t, output, "login")
	assert.Empty(t, stderr.String())
}

func TestNewRootCmd_BindsPersistentFlagsIntoOptions(t *testing.T) {
	opts := &RootOptions{}
	cmd := NewRootCmd(opts)
	cmd.SetArgs([]string{
		"--url", "http://broker.example:8080",
		"--username", "admin",
		"--password", "secret",
		"--token", "jwt-token",
		"--json",
	})

	err := cmd.Execute()
	require.NoError(t, err)

	assert.Equal(t, "http://broker.example:8080", opts.BaseURL)
	assert.Equal(t, "admin", opts.Username)
	assert.Equal(t, "secret", opts.Password)
	assert.Equal(t, "jwt-token", opts.Token)
	assert.True(t, opts.JSON)
}

func TestNewRootCmd_UsesDefaultBaseURL(t *testing.T) {
	opts := &RootOptions{}
	cmd := NewRootCmd(opts)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	require.NoError(t, err)

	assert.Equal(t, DefaultBaseURL, opts.BaseURL)
	assert.Empty(t, opts.Username)
	assert.Empty(t, opts.Password)
	assert.Empty(t, opts.Token)
	assert.False(t, opts.JSON)
}

func TestNewRootCmd_RegistersLoginCommand(t *testing.T) {
	opts := &RootOptions{}
	cmd := NewRootCmd(opts)

	loginCmd, _, err := cmd.Find([]string{"login"})
	require.NoError(t, err)
	require.NotNil(t, loginCmd)
	assert.Equal(t, "login", loginCmd.Name())
}

func TestNewRootCmd_RegistersPublishCommand(t *testing.T) {
	opts := &RootOptions{}
	cmd := NewRootCmd(opts)

	publishCmd, _, err := cmd.Find([]string{"publish"})
	require.NoError(t, err)
	require.NotNil(t, publishCmd)
	assert.Equal(t, "publish", publishCmd.Name())
}
