package cli

import (
	"fmt"
	"strings"

	"github.com/ottermq/ottermq/internal/core/models"
	"github.com/spf13/cobra"
)

func NewQueuesCmd(rt *Runtime) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "queues",
		Short: "Inspect queues",
	}

	cmd.AddCommand(newQueuesListCmd(rt))
	cmd.AddCommand(newQueuesGetCmd(rt))
	cmd.AddCommand(newQueuesCreateCmd(rt))
	cmd.AddCommand(newQueuesDeleteCmd(rt))
	cmd.AddCommand(newQueuesPurgeCmd(rt))
	cmd.AddCommand(newQueuesGetMessagesCmd(rt))
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

func newQueuesCreateCmd(rt *Runtime) *cobra.Command {
	var durable bool
	var autoDelete bool
	var passive bool
	var maxLength int32
	var maxLengthSet bool
	var messageTTL int64
	var messageTTLSet bool
	var deadLetterExchange string
	var deadLetterRoutingKey string

	cmd := &cobra.Command{
		Use:   "create <vhost> [queue]",
		Short: "Create a queue",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := rt.AuthenticatedClient(rt.Context())
			if err != nil {
				return err
			}

			queueName := ""
			if len(args) == 2 {
				queueName = args[1]
			}

			req := models.CreateQueueRequest{
				Passive:    passive,
				Durable:    durable,
				AutoDelete: autoDelete,
			}
			if maxLengthSet {
				req.MaxLength = &maxLength
			}
			if messageTTLSet {
				req.MessageTTL = &messageTTL
			}
			if strings.TrimSpace(deadLetterExchange) != "" {
				req.DeadLetterExchange = &deadLetterExchange
			}
			if strings.TrimSpace(deadLetterRoutingKey) != "" {
				req.DeadLetterRoutingKey = &deadLetterRoutingKey
			}

			resp, err := client.CreateQueue(rt.Context(), args[0], queueName, req)
			if err != nil {
				return err
			}

			return rt.WriteOutput(resp, func() error {
				return rt.Println(resp.Message)
			})
		},
	}

	cmd.Flags().BoolVar(&durable, "durable", false, "Create the queue as durable")
	cmd.Flags().BoolVar(&autoDelete, "auto-delete", false, "Delete the queue automatically when no longer used")
	cmd.Flags().BoolVar(&passive, "passive", false, "Validate that the queue exists without changing it")
	cmd.Flags().Int32Var(&maxLength, "max-length", 0, "Set x-max-length on the queue")
	cmd.Flags().Int64Var(&messageTTL, "message-ttl", 0, "Set x-message-ttl in milliseconds")
	cmd.Flags().StringVar(&deadLetterExchange, "dead-letter-exchange", "", "Set x-dead-letter-exchange")
	cmd.Flags().StringVar(&deadLetterRoutingKey, "dead-letter-routing-key", "", "Set x-dead-letter-routing-key")
	cmd.Flags().Lookup("max-length").NoOptDefVal = "0"
	cmd.Flags().Lookup("message-ttl").NoOptDefVal = "0"
	cmd.PreRun = func(cmd *cobra.Command, args []string) {
		maxLengthSet = cmd.Flags().Changed("max-length")
		messageTTLSet = cmd.Flags().Changed("message-ttl")
	}

	return cmd
}

func newQueuesDeleteCmd(rt *Runtime) *cobra.Command {
	var ifUnused bool
	var ifEmpty bool

	cmd := &cobra.Command{
		Use:   "delete <vhost> <queue>",
		Short: "Delete a queue",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := rt.AuthenticatedClient(rt.Context())
			if err != nil {
				return err
			}

			if err := client.DeleteQueue(rt.Context(), args[0], args[1], ifUnused, ifEmpty); err != nil {
				return err
			}

			return rt.Println("Queue deleted")
		},
	}

	cmd.Flags().BoolVar(&ifUnused, "if-unused", false, "Delete only if the queue has no consumers")
	cmd.Flags().BoolVar(&ifEmpty, "if-empty", false, "Delete only if the queue is empty")
	return cmd
}

func newQueuesPurgeCmd(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "purge <vhost> <queue>",
		Short: "Purge all messages from a queue",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := rt.AuthenticatedClient(rt.Context())
			if err != nil {
				return err
			}

			if err := client.PurgeQueue(rt.Context(), args[0], args[1]); err != nil {
				return err
			}

			return rt.Println("Queue purged")
		},
	}
}

func newQueuesGetMessagesCmd(rt *Runtime) *cobra.Command {
	var count int
	var ackMode string

	cmd := &cobra.Command{
		Use:   "get-messages <vhost> <queue>",
		Short: "Get messages from a queue",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := rt.AuthenticatedClient(rt.Context())
			if err != nil {
				return err
			}

			req := models.GetMessageRequest{
				AckMode:      models.AckType(ackMode),
				MessageCount: count,
			}

			resp, err := client.GetMessages(rt.Context(), args[0], args[1], req)
			if err != nil {
				return err
			}

			return rt.WriteOutput(resp, func() error {
				lines := make([]string, 0, len(resp))
				for idx, message := range resp {
					payload := message.PayloadText
					if payload == "" {
						payload = string(message.Payload)
					}
					lines = append(lines, formatSummaryLine(
						fmt.Sprintf("message[%d]", idx+1),
						[]Field{
							{Label: "delivery_tag", Value: fmt.Sprintf("%d", message.DeliveryTag)},
							{Label: "redelivered", Value: formatBool(message.Redelivered)},
							{Label: "payload", Value: payload},
						},
					))
				}
				return rt.PrintSummaryList(lines, "No messages found")
			})
		},
	}

	cmd.Flags().IntVar(&count, "count", 1, "Number of messages to fetch")
	cmd.Flags().StringVar(&ackMode, "ack-mode", string(models.Ack), "Ack mode: ack, no_ack, reject, reject_requeue")
	return cmd
}
