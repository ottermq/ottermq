package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewPermissionsCmd(rt *Runtime) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "permissions",
		Short: "Manage vhost permissions",
	}

	cmd.AddCommand(newPermissionsListCmd(rt))
	cmd.AddCommand(newPermissionsGetCmd(rt))
	cmd.AddCommand(newPermissionsGrantCmd(rt))
	cmd.AddCommand(newPermissionsRevokeCmd(rt))

	return cmd
}

func newPermissionsListCmd(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all vhost permissions",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := rt.AuthenticatedClient(rt.Context())
			if err != nil {
				return err
			}
			perms, err := client.ListPermissions(rt.Context())
			if err != nil {
				return err
			}
			return rt.WriteOutput(perms, func() error {
				lines := make([]string, 0, len(perms))
				for _, p := range perms {
					lines = append(lines, formatSummaryLine(p.Username, []Field{
						{Label: "vhost", Value: p.VHost},
					}))
				}
				return rt.PrintSummaryList(lines, "No permissions found")
			})
		},
	}
}

func newPermissionsGetCmd(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "get <vhost> <username>",
		Short: "Get vhost permission for a user",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := rt.AuthenticatedClient(rt.Context())
			if err != nil {
				return err
			}
			p, err := client.GetPermission(rt.Context(), args[0], args[1])
			if err != nil {
				return err
			}
			return rt.WriteOutput(p, func() error {
				return rt.PrintFields(
					fmt.Sprintf("Permission: %s on %s", p.Username, p.VHost),
					[]Field{
						{Label: "Username", Value: p.Username},
						{Label: "VHost", Value: p.VHost},
					},
				)
			})
		},
	}
}

func newPermissionsGrantCmd(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "grant <vhost> <username>",
		Short: "Grant a user access to a vhost",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := rt.AuthenticatedClient(rt.Context())
			if err != nil {
				return err
			}
			if _, err := client.GrantPermission(rt.Context(), args[0], args[1]); err != nil {
				return err
			}
			return rt.Println(fmt.Sprintf("Granted '%s' access to vhost '%s'", args[1], args[0]))
		},
	}
}

func newPermissionsRevokeCmd(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "revoke <vhost> <username>",
		Short: "Revoke a user's access to a vhost",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := rt.AuthenticatedClient(rt.Context())
			if err != nil {
				return err
			}
			if err := client.RevokePermission(rt.Context(), args[0], args[1]); err != nil {
				return err
			}
			return rt.Println(fmt.Sprintf("Revoked '%s' access to vhost '%s'", args[1], args[0]))
		},
	}
}
