package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRuntimeWriteOutput_UsesTextRendererWhenJSONDisabled(t *testing.T) {
	var stdout bytes.Buffer
	rt := &Runtime{
		Options: &RootOptions{},
		Stdout:  &stdout,
	}

	err := rt.WriteOutput(map[string]string{"message": "ignored"}, func() error {
		return rt.Println("plain text")
	})

	require.NoError(t, err)
	assert.Equal(t, "plain text\n", stdout.String())
}

func TestRuntimeWriteOutput_UsesJSONWhenEnabled(t *testing.T) {
	var stdout bytes.Buffer
	rt := &Runtime{
		Options: &RootOptions{JSON: true},
		Stdout:  &stdout,
	}

	err := rt.WriteOutput(map[string]string{"message": "hello"}, func() error {
		t.Fatal("text renderer should not be called")
		return nil
	})

	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"message": "hello"`)
}

func TestRuntimePrintFields(t *testing.T) {
	var stdout bytes.Buffer
	rt := &Runtime{
		Options: &RootOptions{},
		Stdout:  &stdout,
	}

	err := rt.PrintFields("Queue: //jobs", []Field{
		{Label: "Messages", Value: "3"},
		{Label: "Durable", Value: "true"},
	})

	require.NoError(t, err)
	assert.Equal(t, "Queue: //jobs\nMessages: 3\nDurable: true\n", stdout.String())
}

func TestRuntimePrintSummaryList_ShowsEmptyMessage(t *testing.T) {
	var stdout bytes.Buffer
	rt := &Runtime{
		Options: &RootOptions{},
		Stdout:  &stdout,
	}

	err := rt.PrintSummaryList(nil, "No queues found")

	require.NoError(t, err)
	assert.Equal(t, "No queues found\n", stdout.String())
}

func TestFormatSummaryLine(t *testing.T) {
	line := formatSummaryLine("//jobs", []Field{
		{Label: "messages", Value: "3"},
		{Label: "consumers", Value: "1"},
	})

	assert.Equal(t, "//jobs messages=3 consumers=1", line)
}
