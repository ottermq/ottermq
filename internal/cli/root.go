package cli

import "github.com/spf13/cobra"

const (
	DefaultBaseURL = "http://localhost:3000"
)

type RootOptions struct {
	BaseURL  string
	Username string
	Password string
	Token    string
	JSON     bool
}

func NewRootCmd(opts *RootOptions) *cobra.Command {
	if opts == nil {
		opts = &RootOptions{}
	}
	rt := NewRuntime(opts)

	cmd := &cobra.Command{
		Use:           "ottermqadmin",
		Short:         "Admin CLI for OtterMQ",
		Long:          "ottermqadmin is a command-line administration tool for OtterMQ.",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	flags := cmd.PersistentFlags()
	flags.StringVar(&opts.BaseURL, "url", DefaultBaseURL, "Base URL for the OtterMQ management API")
	flags.StringVar(&opts.Username, "username", "", "Username for login-based authentication")
	flags.StringVar(&opts.Password, "password", "", "Password for login-based authentication")
	flags.StringVar(&opts.Token, "token", "", "JWT token for authenticated requests")
	flags.BoolVar(&opts.JSON, "json", false, "Output results as JSON")

	cmd.AddCommand(NewLoginCmd(rt))
	cmd.AddCommand(NewOverviewCmd(rt))
	cmd.AddCommand(NewQueuesCmd(rt))
	cmd.AddCommand(NewExchangesCmd(rt))
	cmd.AddCommand(NewBindingsCmd(rt))
	cmd.AddCommand(NewPublishCmd(rt))
	cmd.AddCommand(NewConnectionsCmd(rt))
	cmd.AddCommand(NewChannelsCmd(rt))
	cmd.AddCommand(NewConsumersCmd(rt))
	cmd.AddCommand(NewHealthCmd(rt))
	cmd.AddCommand(NewNodesCmd(rt))
	cmd.AddCommand(NewDefinitionsCmd(rt))

	return cmd
}
