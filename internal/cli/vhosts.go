package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewVHostsCmd(rt *Runtime) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vhosts",
		Short: "Manage virtual hosts",
	}

	cmd.AddCommand(newVHostsListCmd(rt))
	cmd.AddCommand(newVHostsGetCmd(rt))
	cmd.AddCommand(newVHostsCreateCmd(rt))
	cmd.AddCommand(newVHostsDeleteCmd(rt))

	return cmd
}

func newVHostsListCmd(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List virtual hosts",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := rt.AuthenticatedClient(rt.Context())
			if err != nil {
				return err
			}
			vhosts, err := client.ListVHosts(rt.Context())
			if err != nil {
				return err
			}
			return rt.WriteOutput(vhosts, func() error {
				lines := make([]string, 0, len(vhosts))
				for _, v := range vhosts {
					lines = append(lines, formatSummaryLine(v.Name, []Field{
						{Label: "state", Value: v.State},
						{Label: "unacked", Value: fmt.Sprintf("%d", v.UnackedCount)},
					}))
				}
				return rt.PrintSummaryList(lines, "No virtual hosts found")
			})
		},
	}
}

func newVHostsGetCmd(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "get <vhost>",
		Short: "Get virtual host details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := rt.AuthenticatedClient(rt.Context())
			if err != nil {
				return err
			}
			v, err := client.GetVHost(rt.Context(), args[0])
			if err != nil {
				return err
			}
			return rt.WriteOutput(v, func() error {
				return rt.PrintFields(
					fmt.Sprintf("VHost: %s", v.Name),
					[]Field{
						{Label: "State", Value: v.State},
						{Label: "Users", Value: formatStringSlice(v.Users)},
						{Label: "Unacked", Value: fmt.Sprintf("%d", v.UnackedCount)},
						{Label: "Prefetch", Value: fmt.Sprintf("%d", v.PrefetchCount)},
						{Label: "Unconfirmed", Value: fmt.Sprintf("%d", v.UnconfirmedCount)},
					},
				)
			})
		},
	}
}

func newVHostsCreateCmd(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "create <vhost>",
		Short: "Create a virtual host",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := rt.AuthenticatedClient(rt.Context())
			if err != nil {
				return err
			}
			v, err := client.CreateVHost(rt.Context(), args[0])
			if err != nil {
				return err
			}
			return rt.WriteOutput(v, func() error {
				return rt.Println(fmt.Sprintf("VHost '%s' created", v.Name))
			})
		},
	}
}

func newVHostsDeleteCmd(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <vhost>",
		Short: "Delete a virtual host",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := rt.AuthenticatedClient(rt.Context())
			if err != nil {
				return err
			}
			if err := client.DeleteVHost(rt.Context(), args[0]); err != nil {
				return err
			}
			return rt.Println(fmt.Sprintf("VHost '%s' deleted", args[0]))
		},
	}
}
