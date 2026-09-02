package cli

import (
	"fmt"
	"strings"
)

type Field struct {
	Label string
	Value string
}

func (rt *Runtime) WriteOutput(jsonValue any, textRenderer func() error) error {
	if rt.Options.JSON {
		return rt.PrintJSON(jsonValue)
	}
	return textRenderer()
}

func (rt *Runtime) PrintFields(title string, fields []Field) error {
	if title != "" {
		if err := rt.Printf("%s\n", title); err != nil {
			return err
		}
	}

	for _, field := range fields {
		if err := rt.Printf("%s: %s\n", field.Label, field.Value); err != nil {
			return err
		}
	}

	return nil
}

func (rt *Runtime) PrintSummaryList(lines []string, emptyMessage string) error {
	if len(lines) == 0 && emptyMessage != "" {
		return rt.Println(emptyMessage)
	}

	for _, line := range lines {
		if err := rt.Println(line); err != nil {
			return err
		}
	}
	return nil
}

func formatSummaryLine(head string, fields []Field) string {
	if len(fields) == 0 {
		return head
	}

	parts := make([]string, 0, len(fields))
	for _, field := range fields {
		parts = append(parts, fmt.Sprintf("%s=%s", strings.ToLower(field.Label), field.Value))
	}

	return fmt.Sprintf("%s %s", head, strings.Join(parts, " "))
}
