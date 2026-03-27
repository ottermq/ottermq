package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

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

			return rt.WriteOutput(resp, func() error {
				lines := make([]string, 0, len(resp))
				for _, exchange := range resp {
					lines = append(lines, formatSummaryLine(
						fmt.Sprintf("%s/%s", exchange.VHost, exchange.Name),
						[]Field{
							{Label: "type", Value: exchange.Type},
							{Label: "durable", Value: formatBool(exchange.Durable)},
							{Label: "internal", Value: formatBool(exchange.Internal)},
						},
					))
				}
				return rt.PrintSummaryList(lines, "No exchanges found")
			})
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

			return rt.WriteOutput(resp, func() error {
				return rt.PrintFields(
					fmt.Sprintf("Exchange: %s/%s", resp.VHost, resp.Name),
					[]Field{
						{Label: "Type", Value: resp.Type},
						{Label: "Durable", Value: formatBool(resp.Durable)},
						{Label: "Auto Delete", Value: formatBool(resp.AutoDelete)},
						{Label: "Internal", Value: formatBool(resp.Internal)},
					},
				)
			})
		},
	}
}
