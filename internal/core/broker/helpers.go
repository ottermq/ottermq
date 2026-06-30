package broker

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"syscall"

	"github.com/ottermq/ottermq/internal/core/models"
	"github.com/rs/zerolog/log"
)

func getHostname() string {
	host, err := os.Hostname()
	if err != nil {
		return "localhost"
	}
	return host
}

// getCommitInfo extracts commit information from the version string (from broker.config.VersionInfo).
func getCommitInfo(verInfo string) models.CommitInfo {
	var commit models.CommitInfo
	// split verInfo into version, commit, buildNum (assuming format "version-buildNum-commit")
	parts := strings.Split(verInfo, "-")
	if len(parts) >= 3 {
		commit.Version = parts[0]    // "v0.14.0"
		commit.CommitNum = parts[1]  // "92"
		commit.CommitHash = parts[2] // "g3bb9ca6"
		return commit
	}
	commit.Version = verInfo
	return commit
}

func getFileDescriptors() uint32 {
	pid := os.Getpid()
	fdDir := fmt.Sprintf("/proc/%d/fd", pid)

	entries, err := os.ReadDir(fdDir)
	if err != nil {
		log.Error().Err(err).Msg("Error reading fd dir")
		return 0
	}
	return uint32(len(entries))
}

func getFileDescriptorLimit() uint32 {
	var rLimit syscall.Rlimit
	err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	if err != nil {
		log.Error().Err(err).Msg("Error getting rlimit")
		return 0
	}
	return uint32(rLimit.Cur)
}

func getMemoryUsage() uint32 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return uint32(m.Alloc)
}

type Sysinfo struct {
	TotalRam  uint64
	Uptime    int64
	TotalDisk uint64
	AvailDisk uint64
}
