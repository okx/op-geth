package streamer

import "time"

type StreamerConfig struct {
	Port                   uint16
	WriteTimeout           time.Duration
	InactivityTimeout      time.Duration
	HeartbeatCheckInterval time.Duration
}
