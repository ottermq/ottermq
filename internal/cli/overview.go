package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewOverviewCmd(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "overview",
		Short: "Show broker overview information",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := rt.AuthenticatedClient(rt.Context())
			if err != nil {
				return err
			}

			resp, err := client.GetOverview(rt.Context())
			if err != nil {
				return err
			}

			return rt.WriteOutput(resp, func() error {
				return rt.PrintFields(
					fmt.Sprintf("Broker: %s %s", resp.BrokerDetails.Product, resp.BrokerDetails.Version),
					[]Field{
						{Label: "Connections", Value: fmt.Sprintf("%d", resp.ObjectTotals.Connections)},
						{Label: "Channels", Value: fmt.Sprintf("%d", resp.ObjectTotals.Channels)},
						{Label: "Queues", Value: fmt.Sprintf("%d", resp.ObjectTotals.Queues)},
						{Label: "Consumers", Value: fmt.Sprintf("%d", resp.ObjectTotals.Consumers)},
						{Label: "Messages Ready", Value: fmt.Sprintf("%d", resp.MessageStats.MessagesReady)},
						{Label: "Messages Unacked", Value: fmt.Sprintf("%d", resp.MessageStats.MessagesUnacked)},
					},
				)
			})
		},
	}
}

func formatBool(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func formatMaybeString(value *string) string {
	if value == nil {
		return "-"
	}
	return *value
}

func formatMaybeInt32(value *int32) string {
	if value == nil {
		return "-"
	}
	return fmt.Sprintf("%d", *value)
}

func formatMaybeInt64(value *int64) string {
	if value == nil {
		return "-"
	}
	return fmt.Sprintf("%d", *value)
}
