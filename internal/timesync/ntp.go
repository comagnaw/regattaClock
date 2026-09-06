package timesync

import (
	"time"

	"github.com/beevik/ntp"
)

// queryNTP is the real network query, kept in its own file so tests can stub
// Config.QueryFunc without ever opening a socket. It measures the offset
// without touching the system clock (no elevation, no w32tm).
func queryNTP(host string, timeout time.Duration) (offset, rtt time.Duration, err error) {
	resp, err := ntp.QueryWithOptions(host, ntp.QueryOptions{Timeout: timeout})
	if err != nil {
		return 0, 0, err
	}
	if err := resp.Validate(); err != nil {
		return 0, 0, err
	}
	return resp.ClockOffset, resp.RTT, nil
}
