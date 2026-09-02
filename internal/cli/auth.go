package cli

import (
	"errors"
	"strings"

	"github.com/spf13/cobra"
)

func NewLoginCmd(rt *Runtime) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate against the OtterMQ management API",
		RunE: func(cmd *cobra.Command, args []string) error {
			if rt == nil || rt.Options == nil {
				return errors.New("cli runtime not configured")
			}
			if strings.TrimSpace(rt.Options.Username) == "" {
				return errors.New("username is required")
			}
			if strings.TrimSpace(rt.Options.Password) == "" {
				return errors.New("password is required")
			}

			resp, err := rt.Client().Login(rt.Context(), rt.Options.Username, rt.Options.Password)
			if err != nil {
				return err
			}

			rt.Options.Token = resp.Token

			if rt.Options.JSON {
				return rt.PrintJSON(resp)
			}

			return rt.Println("Login successful")
		},
	}

	return cmd
}

