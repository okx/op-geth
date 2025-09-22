package streamclient

import "time"

type StreamClientConfig struct {
	RealtimeStreamerUrl     string
	RealtimeStreamerUseTLS  bool
	RealtimeStreamerTimeout time.Duration
}
