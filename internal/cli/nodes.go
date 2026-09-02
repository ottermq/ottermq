package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewNodesCmd(rt *Runtime) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "nodes",
		Short: "Inspect cluster nodes",
	}

	cmd.AddCommand(newNodesListCmd(rt))
	cmd.AddCommand(newNodesGetCmd(rt))
	cmd.AddCommand(newNodesMemoryCmd(rt))

	return cmd
}

func newNodesListCmd(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List cluster nodes",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := rt.AuthenticatedClient(rt.Context())
			if err != nil {
				return err
			}
			nodes, err := client.ListNodes(rt.Context())
			if err != nil {
				return err
			}
			return rt.WriteOutput(nodes, func() error {
				lines := make([]string, 0, len(nodes))
				for _, n := range nodes {
					lines = append(lines, formatSummaryLine(n.Name, []Field{
						{Label: "goroutines", Value: fmt.Sprintf("%d", n.Goroutines)},
						{Label: "mem", Value: formatBytes(n.MemoryUsage)},
						{Label: "uptime", Value: formatUptime(n.UptimeSecs)},
					}))
				}
				return rt.PrintSummaryList(lines, "No nodes found")
			})
		},
	}
}

func newNodesGetCmd(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "get <name>",
		Short: "Get node details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := rt.AuthenticatedClient(rt.Context())
			if err != nil {
				return err
			}
			n, err := client.GetNode(rt.Context(), args[0])
			if err != nil {
				return err
			}
			return rt.WriteOutput(n, func() error {
				return rt.PrintFields(
					fmt.Sprintf("Node: %s", n.Name),
					[]Field{
						{Label: "Goroutines", Value: fmt.Sprintf("%d", n.Goroutines)},
						{Label: "File descriptors", Value: fmt.Sprintf("%d / %d", n.FDUsed, n.FDLimit)},
						{Label: "Memory used", Value: formatBytes(n.MemoryUsage)},
						{Label: "Memory limit", Value: formatBytes(n.MemoryLimit)},
						{Label: "Disk free", Value: formatBytes(n.DiskAvailable)},
						{Label: "Disk total", Value: formatBytes(n.DiskTotal)},
						{Label: "CPU cores", Value: fmt.Sprintf("%d", n.Cores)},
						{Label: "Uptime", Value: formatUptime(n.UptimeSecs)},
						{Label: "Enabled plugins", Value: formatStringSlice(n.Info.EnabledPlugins)},
					},
				)
			})
		},
	}
}

func newNodesMemoryCmd(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "memory <name>",
		Short: "Show memory breakdown for a node",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := rt.AuthenticatedClient(rt.Context())
			if err != nil {
				return err
			}
			mem, err := client.GetNodeMemory(rt.Context(), args[0])
			if err != nil {
				return err
			}
			return rt.WriteOutput(mem, func() error {
				return rt.PrintFields(
					fmt.Sprintf("Memory breakdown: %s", args[0]),
					[]Field{
						{Label: "System total", Value: formatBytes(mem.Memory.Total)},
						{Label: "Process used", Value: formatBytes(mem.Memory.Used)},
						{Label: "Heap allocated", Value: formatBytes(mem.Memory.HeapAlloc)},
						{Label: "Heap from OS", Value: formatBytes(mem.Memory.HeapSys)},
						{Label: "Heap in-use", Value: formatBytes(mem.Memory.HeapInUse)},
						{Label: "Heap idle", Value: formatBytes(mem.Memory.HeapIdle)},
						{Label: "Stack in-use", Value: formatBytes(mem.Memory.StackInUse)},
						{Label: "GC metadata", Value: formatBytes(mem.Memory.GCSys)},
						{Label: "Other runtime", Value: formatBytes(mem.Memory.OtherSys)},
					},
				)
			})
		},
	}
}

func formatBytes(b int) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case b >= GB:
		return fmt.Sprintf("%.2f GB", float64(b)/GB)
	case b >= MB:
		return fmt.Sprintf("%.2f MB", float64(b)/MB)
	case b >= KB:
		return fmt.Sprintf("%.2f KB", float64(b)/KB)
	default:
		return fmt.Sprintf("%d B", b)
	}
}

func formatUptime(secs int) string {
	if secs < 60 {
		return fmt.Sprintf("%ds", secs)
	}
	if secs < 3600 {
		return fmt.Sprintf("%dm%ds", secs/60, secs%60)
	}
	h := secs / 3600
	m := (secs % 3600) / 60
	return fmt.Sprintf("%dh%dm", h, m)
}

func formatStringSlice(ss []string) string {
	if len(ss) == 0 {
		return "(none)"
	}
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += ", "
		}
		result += s
	}
	return result
}
