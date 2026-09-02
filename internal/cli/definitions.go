package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ottermq/ottermq/internal/core/models"
	"github.com/spf13/cobra"
)


func NewDefinitionsCmd(rt *Runtime) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "definitions",
		Short: "Export and import broker definitions",
	}

	cmd.AddCommand(newDefinitionsExportCmd(rt))
	cmd.AddCommand(newDefinitionsImportCmd(rt))

	return cmd
}

func newDefinitionsExportCmd(rt *Runtime) *cobra.Command {
	var vhost string
	var output string

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export broker definitions to JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := rt.AuthenticatedClient(rt.Context())
			if err != nil {
				return err
			}

			var defs *models.DefinitionsDTO
			if vhost != "" {
				defs, err = client.ExportVHostDefinitions(rt.Context(), vhost)
			} else {
				defs, err = client.ExportDefinitions(rt.Context())
			}
			if err != nil {
				return err
			}

			data, err := json.MarshalIndent(defs, "", "  ")
			if err != nil {
				return fmt.Errorf("marshal definitions: %w", err)
			}

			if output != "" {
				if err := os.WriteFile(output, data, 0644); err != nil {
					return fmt.Errorf("write file: %w", err)
				}
				return rt.Println(fmt.Sprintf("Definitions written to %s", output))
			}

			return rt.Println(string(data))
		},
	}

	cmd.Flags().StringVar(&vhost, "vhost", "", "Scope export to a single virtual host")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Write output to file instead of stdout")

	return cmd
}

func newDefinitionsImportCmd(rt *Runtime) *cobra.Command {
	var vhost string

	cmd := &cobra.Command{
		Use:   "import <file>",
		Short: "Import broker definitions from a JSON file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("read file: %w", err)
			}

			var defs models.DefinitionsDTO
			if err := json.Unmarshal(data, &defs); err != nil {
				return fmt.Errorf("parse definitions: %w", err)
			}

			client, err := rt.AuthenticatedClient(rt.Context())
			if err != nil {
				return err
			}

			var result *models.DefinitionsImportResponse
			if vhost != "" {
				result, err = client.ImportVHostDefinitions(rt.Context(), vhost, &defs)
			} else {
				result, err = client.ImportDefinitions(rt.Context(), &defs)
			}
			if err != nil {
				return err
			}

			return rt.WriteOutput(result, func() error {
				return rt.PrintFields("Import complete", []Field{
					{Label: "VHosts created", Value: fmt.Sprintf("%d", result.VHosts)},
					{Label: "Users created", Value: fmt.Sprintf("%d", result.Users)},
					{Label: "Permissions granted", Value: fmt.Sprintf("%d", result.Permissions)},
					{Label: "Exchanges created", Value: fmt.Sprintf("%d", result.Exchanges)},
					{Label: "Queues created", Value: fmt.Sprintf("%d", result.Queues)},
					{Label: "Bindings created", Value: fmt.Sprintf("%d", result.Bindings)},
				})
			})
		},
	}

	cmd.Flags().StringVar(&vhost, "vhost", "", "Scope import to a single virtual host")

	return cmd
}
