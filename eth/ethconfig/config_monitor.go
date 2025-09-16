package ethconfig

// MonitorConfig contains configuration for transaction monitoring
type MonitorConfig struct {
	EnableTraceLog bool   `toml:",omitempty"`
	TraceLogPath   string `toml:",omitempty"`
}

// DefaultMonitorConfig returns the default configuration for monitoring
func DefaultMonitorConfig() MonitorConfig {
	return MonitorConfig{
		EnableTraceLog: false,
		TraceLogPath:   "/var/log/op-geth/trace.log",
	}
}
