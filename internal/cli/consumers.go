package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

func NewConsumersCmd(rt *Runtime) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "consumers",
		Short: "Inspect consumers",
	}

	cmd.AddCommand(newConsumersListCmd(rt))
	return cmd
}

func newConsumersListCmd(rt *Runtime) *cobra.Command {
	var vhost string
	var queue string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List consumers",
		RunE: func(cmd *cobra.Command, args []string) error {
			if queue != "" && vhost == "" {
				return fmt.Errorf("vhost is required when filtering by queue")
			}

			client, err := rt.AuthenticatedClient(rt.Context())
			if err != nil {
				return err
			}

			resp, err := client.ListConsumers(rt.Context(), vhost, queue)
			if err != nil {
				return err
			}

			return rt.WriteOutput(resp, func() error {
				lines := make([]string, 0, len(resp))
				for _, consumer := range resp {
					lines = append(lines, formatSummaryLine(
						consumer.ConsumerTag,
						[]Field{
							{Label: "queue", Value: consumer.QueueName},
							{Label: "channel", Value: consumer.ChannelDetails.ConnectionName + "#" + strconv.FormatUint(uint64(consumer.ChannelDetails.Number), 10)},
							{Label: "ack_required", Value: formatBool(consumer.AckRequired)},
							{Label: "active", Value: formatBool(consumer.Active)},
						},
					))
				}
				return rt.PrintSummaryList(lines, "No consumers found")
			})
		},
	}

	cmd.Flags().StringVar(&vhost, "vhost", "", "Filter consumers by vhost")
	cmd.Flags().StringVar(&queue, "queue", "", "Filter consumers by queue name")
	return cmd
}
