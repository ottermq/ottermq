// go: linux
package broker

import (
	"syscall"

	"github.com/rs/zerolog/log"
)

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
