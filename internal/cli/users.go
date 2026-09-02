package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var roleNameToID = map[string]int{
	"admin": 1,
	"user":  2,
	"guest": 3,
}

func NewUsersCmd(rt *Runtime) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "users",
		Short: "Manage users",
	}

	cmd.AddCommand(newUsersListCmd(rt))
	cmd.AddCommand(newUsersGetCmd(rt))
	cmd.AddCommand(newUsersCreateCmd(rt))
	cmd.AddCommand(newUsersDeleteCmd(rt))
	cmd.AddCommand(newUsersChangePasswordCmd(rt))

	return cmd
}

func newUsersListCmd(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List users",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := rt.AuthenticatedClient(rt.Context())
			if err != nil {
				return err
			}
			users, err := client.ListUsers(rt.Context())
			if err != nil {
				return err
			}
			return rt.WriteOutput(users, func() error {
				lines := make([]string, 0, len(users))
				for _, u := range users {
					lines = append(lines, formatSummaryLine(u.Username, []Field{
						{Label: "role", Value: u.Role},
						{Label: "password", Value: formatBool(u.HasPassword)},
					}))
				}
				return rt.PrintSummaryList(lines, "No users found")
			})
		},
	}
}

func newUsersGetCmd(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "get <username>",
		Short: "Get user details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := rt.AuthenticatedClient(rt.Context())
			if err != nil {
				return err
			}
			u, err := client.GetUser(rt.Context(), args[0])
			if err != nil {
				return err
			}
			return rt.WriteOutput(u, func() error {
				return rt.PrintFields(
					fmt.Sprintf("User: %s", u.Username),
					[]Field{
						{Label: "ID", Value: fmt.Sprintf("%d", u.ID)},
						{Label: "Role", Value: u.Role},
						{Label: "Has password", Value: formatBool(u.HasPassword)},
					},
				)
			})
		},
	}
}

func newUsersCreateCmd(rt *Runtime) *cobra.Command {
	var password string
	var role string

	cmd := &cobra.Command{
		Use:   "create <username>",
		Short: "Create a user",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			roleID, ok := roleNameToID[strings.ToLower(role)]
			if !ok {
				return fmt.Errorf("invalid role %q — must be one of: admin, user, guest", role)
			}

			client, err := rt.AuthenticatedClient(rt.Context())
			if err != nil {
				return err
			}
			resp, err := client.CreateUser(rt.Context(), args[0], password, roleID)
			if err != nil {
				return err
			}
			return rt.WriteOutput(resp, func() error {
				return rt.Println(resp.Message)
			})
		},
	}

	cmd.Flags().StringVar(&password, "password", "", "Password for the new user (required)")
	cmd.Flags().StringVar(&role, "role", "user", "Role for the new user: admin, user, guest")
	_ = cmd.MarkFlagRequired("password")

	return cmd
}

func newUsersDeleteCmd(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <username>",
		Short: "Delete a user",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := rt.AuthenticatedClient(rt.Context())
			if err != nil {
				return err
			}
			if err := client.DeleteUser(rt.Context(), args[0]); err != nil {
				return err
			}
			return rt.Println(fmt.Sprintf("User '%s' deleted", args[0]))
		},
	}
}

func newUsersChangePasswordCmd(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "change-password <username>",
		Short: "Change a user's password",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			password, err := promptPassword("New password: ")
			if err != nil {
				return err
			}
			confirm, err := promptPassword("Confirm password: ")
			if err != nil {
				return err
			}
			if password != confirm {
				return fmt.Errorf("passwords do not match")
			}

			client, err := rt.AuthenticatedClient(rt.Context())
			if err != nil {
				return err
			}
			resp, err := client.ChangePassword(rt.Context(), args[0], password)
			if err != nil {
				return err
			}
			return rt.WriteOutput(resp, func() error {
				return rt.Println(resp.Message)
			})
		},
	}
}

func promptPassword(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	raw, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr) // move to next line after hidden input
	if err != nil {
		return "", fmt.Errorf("reading password: %w", err)
	}
	return strings.TrimSpace(string(raw)), nil
}
