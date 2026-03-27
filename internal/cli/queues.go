package cli

import (
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

			if rt.Options.JSON {
				return rt.PrintJSON(resp)
			}

			for _, queue := range resp {
				if err := rt.Printf("%s/%s messages=%d consumers=%d durable=%s\n",
					queue.VHost,
					queue.Name,
					queue.Messages,
					queue.Consumers,
					formatBool(queue.Durable),
				); err != nil {
					return err
				}
			}
			return nil
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

			if rt.Options.JSON {
				return rt.PrintJSON(resp)
			}

			return rt.Printf(
				"Queue: %s/%s\nMessages: %d\nConsumers: %d\nDurable: %s\nAuto Delete: %s\nDLX: %s\nMessage TTL: %s\nMax Length: %s\n",
				resp.VHost,
				resp.Name,
				resp.Messages,
				resp.Consumers,
				formatBool(resp.Durable),
				formatBool(resp.AutoDelete),
				formatMaybeString(resp.DeadLetterExchange),
				formatMaybeInt64(resp.MessageTTL),
				formatMaybeInt32(resp.MaxLength),
			)
		},
	}
}

