package cli

import (
	"fmt"

	"github.com/ottermq/ottermq/internal/core/models"
	"github.com/spf13/cobra"
)

func NewExchangesCmd(rt *Runtime) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exchanges",
		Short: "Inspect exchanges",
	}

	cmd.AddCommand(newExchangesListCmd(rt))
	cmd.AddCommand(newExchangesGetCmd(rt))
	cmd.AddCommand(newExchangesCreateCmd(rt))
	cmd.AddCommand(newExchangesDeleteCmd(rt))
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

func newExchangesCreateCmd(rt *Runtime) *cobra.Command {
	var exchangeType string
	var durable bool
	var autoDelete bool
	var passive bool

	cmd := &cobra.Command{
		Use:   "create <vhost> <exchange>",
		Short: "Create an exchange",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if exchangeType == "" {
				return fmt.Errorf("exchange type is required")
			}

			client, err := rt.AuthenticatedClient(rt.Context())
			if err != nil {
				return err
			}

			resp, err := client.CreateExchange(rt.Context(), args[0], args[1], models.CreateExchangeRequest{
				ExchangeType: exchangeType,
				Passive:      passive,
				Durable:      durable,
				AutoDelete:   autoDelete,
			})
			if err != nil {
				return err
			}

			return rt.WriteOutput(resp, func() error {
				return rt.Println(resp.Message)
			})
		},
	}

	cmd.Flags().StringVar(&exchangeType, "type", "", "Exchange type: direct, fanout, topic, headers")
	cmd.Flags().BoolVar(&durable, "durable", false, "Create the exchange as durable")
	cmd.Flags().BoolVar(&autoDelete, "auto-delete", false, "Delete the exchange automatically when unused")
	cmd.Flags().BoolVar(&passive, "passive", false, "Validate that the exchange exists without changing it")
	return cmd
}

func newExchangesDeleteCmd(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <vhost> <exchange>",
		Short: "Delete an exchange",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := rt.AuthenticatedClient(rt.Context())
			if err != nil {
				return err
			}

			if err := client.DeleteExchange(rt.Context(), args[0], args[1]); err != nil {
				return err
			}

			return rt.Println("Exchange deleted")
		},
	}
}
