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

			if rt.Options.JSON {
				return rt.PrintJSON(resp)
			}

			for _, binding := range resp {
				if err := rt.Printf("%s %s -> %s routing_key=%s\n",
					binding.VHost,
					binding.Source,
					binding.Destination,
					binding.RoutingKey,
				); err != nil {
					return err
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&vhost, "vhost", "", "Filter bindings by vhost")
	return cmd
}

