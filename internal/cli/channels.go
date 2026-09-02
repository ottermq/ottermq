package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

func NewChannelsCmd(rt *Runtime) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "channels",
		Short: "Inspect channels",
	}

	cmd.AddCommand(newChannelsListCmd(rt))
	cmd.AddCommand(newChannelsGetCmd(rt))
	return cmd
}

func newChannelsListCmd(rt *Runtime) *cobra.Command {
	var vhost string
	var connection string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List channels",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := rt.AuthenticatedClient(rt.Context())
			if err != nil {
				return err
			}

			resp, err := client.ListChannels(rt.Context(), vhost, connection)
			if err != nil {
				return err
			}

			return rt.WriteOutput(resp, func() error {
				lines := make([]string, 0, len(resp))
				for _, channel := range resp {
					lines = append(lines, formatSummaryLine(
						channel.ConnectionName+"#"+strconv.FormatUint(uint64(channel.Number), 10),
						[]Field{
							{Label: "vhost", Value: channel.VHost},
							{Label: "user", Value: channel.User},
							{Label: "state", Value: channel.State},
							{Label: "unacked", Value: fmt.Sprintf("%d", channel.UnackedCount)},
						},
					))
				}
				return rt.PrintSummaryList(lines, "No channels found")
			})
		},
	}

	cmd.Flags().StringVar(&vhost, "vhost", "", "Filter channels by vhost")
	cmd.Flags().StringVar(&connection, "connection", "", "Filter channels by connection name")
	return cmd
}

func newChannelsGetCmd(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "get <connection> <channel>",
		Short: "Get channel details",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			number, err := strconv.ParseUint(args[1], 10, 16)
			if err != nil {
				return fmt.Errorf("channel must be a valid integer")
			}

			client, err := rt.AuthenticatedClient(rt.Context())
			if err != nil {
				return err
			}

			resp, err := client.GetChannel(rt.Context(), args[0], uint16(number))
			if err != nil {
				return err
			}

			return rt.WriteOutput(resp, func() error {
				return rt.PrintFields(
					fmt.Sprintf("Channel: %s#%d", resp.ConnectionName, resp.Number),
					[]Field{
						{Label: "VHost", Value: resp.VHost},
						{Label: "User", Value: resp.User},
						{Label: "State", Value: resp.State},
						{Label: "Unconfirmed", Value: fmt.Sprintf("%d", resp.UnconfirmedCount)},
						{Label: "Prefetch", Value: fmt.Sprintf("%d", resp.PrefetchCount)},
						{Label: "Unacked", Value: fmt.Sprintf("%d", resp.UnackedCount)},
						{Label: "Publish Rate", Value: fmt.Sprintf("%.2f", resp.PublishRate)},
						{Label: "Deliver Rate", Value: fmt.Sprintf("%.2f", resp.DeliverRate)},
						{Label: "Ack Rate", Value: fmt.Sprintf("%.2f", resp.AckRate)},
					},
				)
			})
		},
	}
}
