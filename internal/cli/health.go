package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewHealthCmd(rt *Runtime) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "health",
		Short: "Run broker health checks",
	}

	cmd.AddCommand(newHealthCheckAlarmsCmd(rt))
	cmd.AddCommand(newHealthCheckLocalAlarmsCmd(rt))
	cmd.AddCommand(newHealthCheckPortListenerCmd(rt))
	cmd.AddCommand(newHealthCheckVirtualHostsCmd(rt))
	cmd.AddCommand(newHealthCheckReadyCmd(rt))

	return cmd
}

func newHealthCheckAlarmsCmd(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "check-alarms",
		Short: "Check for broker-wide alarms",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := rt.AuthenticatedClient(rt.Context())
			if err != nil {
				return err
			}
			resp, err := client.CheckAlarms(rt.Context())
			if err != nil {
				return err
			}
			return rt.WriteOutput(resp, func() error {
				return printHealthResult(rt, resp.Status, resp.Reason)
			})
		},
	}
}

func newHealthCheckLocalAlarmsCmd(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "check-local-alarms",
		Short: "Check for local node alarms",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := rt.AuthenticatedClient(rt.Context())
			if err != nil {
				return err
			}
			resp, err := client.CheckLocalAlarms(rt.Context())
			if err != nil {
				return err
			}
			return rt.WriteOutput(resp, func() error {
				return printHealthResult(rt, resp.Status, resp.Reason)
			})
		},
	}
}

func newHealthCheckPortListenerCmd(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "check-port-listener <port>",
		Short: "Check if the broker is listening on a given port",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := rt.AuthenticatedClient(rt.Context())
			if err != nil {
				return err
			}
			resp, err := client.CheckPortListener(rt.Context(), args[0])
			if err != nil {
				return err
			}
			return rt.WriteOutput(resp, func() error {
				return printHealthResult(rt, resp.Status, resp.Reason)
			})
		},
	}
}

func newHealthCheckVirtualHostsCmd(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "check-virtual-hosts",
		Short: "Check that all virtual hosts are running",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := rt.AuthenticatedClient(rt.Context())
			if err != nil {
				return err
			}
			resp, err := client.CheckVirtualHosts(rt.Context())
			if err != nil {
				return err
			}
			return rt.WriteOutput(resp, func() error {
				return printHealthResult(rt, resp.Status, resp.Reason)
			})
		},
	}
}

func newHealthCheckReadyCmd(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "check-ready",
		Short: "Check if the broker is ready to serve clients",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := rt.AuthenticatedClient(rt.Context())
			if err != nil {
				return err
			}
			resp, err := client.CheckReady(rt.Context())
			if err != nil {
				return err
			}
			return rt.WriteOutput(resp, func() error {
				return printHealthResult(rt, resp.Status, resp.Reason)
			})
		},
	}
}

func printHealthResult(rt *Runtime, status, reason string) error {
	if reason != "" {
		return rt.Println(fmt.Sprintf("%s: %s", status, reason))
	}
	return rt.Println(status)
}
