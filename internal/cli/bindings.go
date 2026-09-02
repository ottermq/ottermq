package cli

import (
	"fmt"

	"github.com/ottermq/ottermq/internal/core/models"
	"github.com/spf13/cobra"
)

func NewBindingsCmd(rt *Runtime) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bindings",
		Short: "Inspect bindings",
	}

	cmd.AddCommand(newBindingsListCmd(rt))
	cmd.AddCommand(newBindingsCreateCmd(rt))
	cmd.AddCommand(newBindingsDeleteCmd(rt))
	return cmd
}

func newBindingsListCmd(rt *Runtime) *cobra.Command {
	var vhost string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List bindings",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := rt.AuthenticatedClient(rt.Context())
			if err != nil {
				return err
			}

			resp, err := client.ListBindings(rt.Context(), vhost)
			if err != nil {
				return err
			}

			return rt.WriteOutput(resp, func() error {
				lines := make([]string, 0, len(resp))
				for _, binding := range resp {
					lines = append(lines, formatSummaryLine(
						binding.VHost,
						[]Field{
							{Label: "source", Value: binding.Source},
							{Label: "destination", Value: binding.Destination},
							{Label: "routing_key", Value: binding.RoutingKey},
						},
					))
				}
				return rt.PrintSummaryList(lines, "No bindings found")
			})
		},
	}

	cmd.Flags().StringVar(&vhost, "vhost", "", "Filter bindings by vhost")
	return cmd
}

func newBindingsCreateCmd(rt *Runtime) *cobra.Command {
	var vhost string
	var source string
	var destination string
	var routingKey string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a binding",
		RunE: func(cmd *cobra.Command, args []string) error {
			if source == "" {
				return fmt.Errorf("source is required")
			}
			if destination == "" {
				return fmt.Errorf("destination is required")
			}

			client, err := rt.AuthenticatedClient(rt.Context())
			if err != nil {
				return err
			}

			resp, err := client.CreateBinding(rt.Context(), models.CreateBindingRequest{
				VHost:       vhost,
				Source:      source,
				Destination: destination,
				RoutingKey:  routingKey,
			})
			if err != nil {
				return err
			}

			return rt.WriteOutput(resp, func() error {
				return rt.Println(resp.Message)
			})
		},
	}

	cmd.Flags().StringVar(&vhost, "vhost", "/", "VHost for the binding")
	cmd.Flags().StringVar(&source, "source", "", "Source exchange")
	cmd.Flags().StringVar(&destination, "destination", "", "Destination queue or exchange")
	cmd.Flags().StringVar(&routingKey, "routing-key", "", "Routing key for the binding")
	return cmd
}

func newBindingsDeleteCmd(rt *Runtime) *cobra.Command {
	var vhost string
	var source string
	var destination string
	var routingKey string

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a binding",
		RunE: func(cmd *cobra.Command, args []string) error {
			if source == "" {
				return fmt.Errorf("source is required")
			}
			if destination == "" {
				return fmt.Errorf("destination is required")
			}

			client, err := rt.AuthenticatedClient(rt.Context())
			if err != nil {
				return err
			}

			if err := client.DeleteBinding(rt.Context(), models.DeleteBindingRequest{
				VHost:       vhost,
				Source:      source,
				Destination: destination,
				RoutingKey:  routingKey,
			}); err != nil {
				return err
			}

			return rt.Println("Binding deleted")
		},
	}

	cmd.Flags().StringVar(&vhost, "vhost", "/", "VHost for the binding")
	cmd.Flags().StringVar(&source, "source", "", "Source exchange")
	cmd.Flags().StringVar(&destination, "destination", "", "Destination queue or exchange")
	cmd.Flags().StringVar(&routingKey, "routing-key", "", "Routing key for the binding")
	return cmd
}
