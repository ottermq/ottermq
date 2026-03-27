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

			if rt.Options.JSON {
				return rt.PrintJSON(resp)
			}

			return rt.Printf(
				"Broker: %s %s\nConnections: %d\nChannels: %d\nQueues: %d\nConsumers: %d\nMessages Ready: %d\nMessages Unacked: %d\n",
				resp.BrokerDetails.Product,
				resp.BrokerDetails.Version,
				resp.ObjectTotals.Connections,
				resp.ObjectTotals.Channels,
				resp.ObjectTotals.Queues,
				resp.ObjectTotals.Consumers,
				resp.MessageStats.MessagesReady,
				resp.MessageStats.MessagesUnacked,
			)
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

