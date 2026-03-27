package cli

import "github.com/spf13/cobra"

func NewExchangesCmd(rt *Runtime) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exchanges",
		Short: "Inspect exchanges",
	}

	cmd.AddCommand(newExchangesListCmd(rt))
	cmd.AddCommand(newExchangesGetCmd(rt))
	return cmd
}

func newExchangesListCmd(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List exchanges",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := rt.AuthenticatedClient(rt.Context())
			if err != nil {
				return err
			}

			resp, err := client.ListExchanges(rt.Context())
			if err != nil {
				return err
			}

			if rt.Options.JSON {
				return rt.PrintJSON(resp)
			}

			for _, exchange := range resp {
				if err := rt.Printf("%s/%s type=%s durable=%s internal=%s\n",
					exchange.VHost,
					exchange.Name,
					exchange.Type,
					formatBool(exchange.Durable),
					formatBool(exchange.Internal),
				); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func newExchangesGetCmd(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "get <vhost> <exchange>",
		Short: "Get exchange details",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := rt.AuthenticatedClient(rt.Context())
			if err != nil {
				return err
			}

			resp, err := client.GetExchange(rt.Context(), args[0], args[1])
			if err != nil {
				return err
			}

			if rt.Options.JSON {
				return rt.PrintJSON(resp)
			}

			return rt.Printf(
				"Exchange: %s/%s\nType: %s\nDurable: %s\nAuto Delete: %s\nInternal: %s\n",
				resp.VHost,
				resp.Name,
				resp.Type,
				formatBool(resp.Durable),
				formatBool(resp.AutoDelete),
				formatBool(resp.Internal),
			)
		},
	}
}

