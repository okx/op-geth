package streamclient

import "time"

type StreamClientConfig struct {
	RealtimeStreamerUrl          string
	RealtimeStreamerUseTLS       bool
	RealtimeStreamerWriteTimeout time.Duration
	RealtimeStreamerReadTimeout  time.Duration
	RealtimeStreamerRetryDelay   time.Duration
}
