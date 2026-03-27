package cli

import "github.com/spf13/cobra"

func NewBindingsCmd(rt *Runtime) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bindings",
		Short: "Inspect bindings",
	}

	cmd.AddCommand(newBindingsListCmd(rt))
	return cmd
}

func newBindingsListCmd(rt *Runtime) *cobra.Command {
	var vhost string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List bindings",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := rt.AuthenticatedClient(rt.Context())
			if err != nil {
				return err
			}

			resp, err := client.ListBindings(rt.Context(), vhost)
			if err != nil {
				return err
			}

			return rt.WriteOutput(resp, func() error {
				lines := make([]string, 0, len(resp))
				for _, binding := range resp {
					lines = append(lines, formatSummaryLine(
						binding.VHost,
						[]Field{
							{Label: "source", Value: binding.Source},
							{Label: "destination", Value: binding.Destination},
							{Label: "routing_key", Value: binding.RoutingKey},
						},
					))
				}
				return rt.PrintSummaryList(lines, "No bindings found")
			})
		},
	}

	cmd.Flags().StringVar(&vhost, "vhost", "", "Filter bindings by vhost")
	return cmd
}
