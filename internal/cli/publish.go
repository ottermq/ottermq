package cli

import (
	"fmt"

	"github.com/ottermq/ottermq/internal/core/models"
	"github.com/spf13/cobra"
)

func NewPublishCmd(rt *Runtime) *cobra.Command {
	var routingKey string
	var body string
	var contentType string
	var deliveryMode uint8
	var priority uint8

	cmd := &cobra.Command{
		Use:   "publish <vhost> <exchange>",
		Short: "Publish a message to an exchange",
		Long:  "Publish a message to an exchange. Use an empty string or '(AMQP default)' for the default exchange.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if body == "" {
				return fmt.Errorf("body is required")
			}

			client, err := rt.AuthenticatedClient(rt.Context())
			if err != nil {
				return err
			}

			resp, err := client.PublishMessage(rt.Context(), args[0], args[1], models.PublishMessageRequest{
				RoutingKey:   routingKey,
				Payload:      body,
				ContentType:  contentType,
				DeliveryMode: deliveryMode,
				Priority:     priority,
			})
			if err != nil {
				return err
			}

			return rt.WriteOutput(resp, func() error {
				return rt.Println(resp.Message)
			})
		},
	}

	cmd.Flags().StringVar(&routingKey, "routing-key", "", "Routing key used for publishing")
	cmd.Flags().StringVar(&body, "body", "", "Message body to publish")
	cmd.Flags().StringVar(&contentType, "content-type", "", "Content type property")
	cmd.Flags().Uint8Var(&deliveryMode, "delivery-mode", 0, "Delivery mode property")
	cmd.Flags().Uint8Var(&priority, "priority", 0, "Priority property")
	return cmd
}
