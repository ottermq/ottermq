package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewQueuesCmd(rt *Runtime) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "queues",
		Short: "Inspect queues",
	}

	cmd.AddCommand(newQueuesListCmd(rt))
	cmd.AddCommand(newQueuesGetCmd(rt))
	return cmd
}

func newQueuesListCmd(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List queues",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := rt.AuthenticatedClient(rt.Context())
			if err != nil {
				return err
			}

			resp, err := client.ListQueues(rt.Context())
			if err != nil {
				return err
			}

			return rt.WriteOutput(resp, func() error {
				lines := make([]string, 0, len(resp))
				for _, queue := range resp {
					lines = append(lines, formatSummaryLine(
						fmt.Sprintf("%s/%s", queue.VHost, queue.Name),
						[]Field{
							{Label: "messages", Value: fmt.Sprintf("%d", queue.Messages)},
							{Label: "consumers", Value: fmt.Sprintf("%d", queue.Consumers)},
							{Label: "durable", Value: formatBool(queue.Durable)},
						},
					))
				}
				return rt.PrintSummaryList(lines, "No queues found")
			})
		},
	}
}

func newQueuesGetCmd(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "get <vhost> <queue>",
		Short: "Get queue details",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := rt.AuthenticatedClient(rt.Context())
			if err != nil {
				return err
			}

			resp, err := client.GetQueue(rt.Context(), args[0], args[1])
			if err != nil {
				return err
			}

			return rt.WriteOutput(resp, func() error {
				return rt.PrintFields(
					fmt.Sprintf("Queue: %s/%s", resp.VHost, resp.Name),
					[]Field{
						{Label: "Messages", Value: fmt.Sprintf("%d", resp.Messages)},
						{Label: "Consumers", Value: fmt.Sprintf("%d", resp.Consumers)},
						{Label: "Durable", Value: formatBool(resp.Durable)},
						{Label: "Auto Delete", Value: formatBool(resp.AutoDelete)},
						{Label: "DLX", Value: formatMaybeString(resp.DeadLetterExchange)},
						{Label: "Message TTL", Value: formatMaybeInt64(resp.MessageTTL)},
						{Label: "Max Length", Value: formatMaybeInt32(resp.MaxLength)},
					},
				)
			})
		},
	}
}
