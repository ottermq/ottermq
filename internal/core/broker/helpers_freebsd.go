// go: build freebsd
package broker

import (
	"syscall"
	"time"

	"github.com/rs/zerolog/log"
	"golang.org/x/sys/unix"
)

func getSysInfo() (Sysinfo, error) {
	// get total system memory
	totalram, err := unix.SysctlUint64("hw.phymem")
	if err != nil {
		log.Error().Err(err).Msg("Error getting total physical memory")
		return Sysinfo{}, err
	}

	tv, err := unix.SysctlTimeval("kern.boottime")
	if err != nil {
		log.Error().Err(err).Msg("Error getting system bootime")
	}
	uptime := time.Now().Unix() - int64(tv.Sec)

	var stat syscall.Statfs_t
	err = syscall.Statfs("/", &stat)
	if err != nil {
		log.Error().Err(err).Msg("Error getting disk stats")
		return Sysinfo{}, err
	}

	return Sysinfo{
		TotalRam:  totalram,
		Uptime:    uptime,
		TotalDisk: stat.Blocks * uint64(stat.Bsize),
		AvailDisk: uint64(stat.Bavail) * uint64(stat.Bsize),
	}, nil
}
