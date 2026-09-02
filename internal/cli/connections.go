package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewConnectionsCmd(rt *Runtime) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "connections",
		Short: "Inspect and manage connections",
	}

	cmd.AddCommand(newConnectionsListCmd(rt))
	cmd.AddCommand(newConnectionsGetCmd(rt))
	cmd.AddCommand(newConnectionsCloseCmd(rt))
	return cmd
}

func newConnectionsListCmd(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List connections",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := rt.AuthenticatedClient(rt.Context())
			if err != nil {
				return err
			}

			resp, err := client.ListConnections(rt.Context())
			if err != nil {
				return err
			}

			return rt.WriteOutput(resp, func() error {
				lines := make([]string, 0, len(resp))
				for _, conn := range resp {
					lines = append(lines, formatSummaryLine(
						conn.Name,
						[]Field{
							{Label: "vhost", Value: conn.VHostName},
							{Label: "user", Value: conn.Username},
							{Label: "state", Value: conn.State},
							{Label: "channels", Value: fmt.Sprintf("%d", conn.Channels)},
						},
					))
				}
				return rt.PrintSummaryList(lines, "No connections found")
			})
		},
	}
}

func newConnectionsGetCmd(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "get <name>",
		Short: "Get connection details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := rt.AuthenticatedClient(rt.Context())
			if err != nil {
				return err
			}

			resp, err := client.GetConnection(rt.Context(), args[0])
			if err != nil {
				return err
			}

			return rt.WriteOutput(resp, func() error {
				return rt.PrintFields(
					fmt.Sprintf("Connection: %s", resp.Name),
					[]Field{
						{Label: "VHost", Value: resp.VHostName},
						{Label: "User", Value: resp.Username},
						{Label: "State", Value: resp.State},
						{Label: "Protocol", Value: resp.Protocol},
						{Label: "SSL", Value: formatBool(resp.SSL)},
						{Label: "Channels", Value: fmt.Sprintf("%d", resp.Channels)},
					},
				)
			})
		},
	}
}

func newConnectionsCloseCmd(rt *Runtime) *cobra.Command {
	var reason string

	cmd := &cobra.Command{
		Use:   "close <name>",
		Short: "Close a connection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := rt.AuthenticatedClient(rt.Context())
			if err != nil {
				return err
			}

			resp, err := client.CloseConnection(rt.Context(), args[0], reason)
			if err != nil {
				return err
			}

			return rt.WriteOutput(resp, func() error {
				return rt.Println(resp.Message)
			})
		},
	}

	cmd.Flags().StringVar(&reason, "reason", "Closed by admin via CLI", "Reason for closing the connection")
	return cmd
}
