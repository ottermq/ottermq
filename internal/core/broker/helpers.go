package broker

import (
	"fmt"
	"os"
	"runtime"
	"syscall"

	"github.com/andrelcunha/ottermq/internal/core/models"
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
	if verInfo != "" {
		var version, commitNum, commitHash string
		n, _ := fmt.Sscanf(verInfo, "%s-%s-%s", &version, &commitNum, &commitHash)
		if n == 3 {
			commit = models.CommitInfo{
				Version:    version,
				CommitNum:  commitNum,
				CommitHash: commitHash,
			}
		} else {
			commit = models.CommitInfo{
				Version: verInfo,
			}
		}
	}
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

func getSysInfo() (Sysinfo, error) {
	// get total system memory
	var sysInfo syscall.Sysinfo_t
	err := syscall.Sysinfo(&sysInfo)
	if err != nil {
		log.Error().Err(err).Msg("Error getting sysinfo")
		return Sysinfo{}, err
	}

	var stat syscall.Statfs_t
	err = syscall.Statfs("/", &stat)
	if err != nil {
		log.Error().Err(err).Msg("Error getting disk stats")
		return Sysinfo{}, err
	}

	return Sysinfo{
		TotalRam:  uint64(sysInfo.Totalram) * uint64(syscall.Getpagesize()),
		Uptime:    int64(sysInfo.Uptime),
		TotalDisk: stat.Blocks * uint64(stat.Bsize),
		AvailDisk: stat.Bavail * uint64(stat.Bsize),
	}, nil
}

type Sysinfo struct {
	TotalRam  uint64
	Uptime    int64
	TotalDisk uint64
	AvailDisk uint64
}
