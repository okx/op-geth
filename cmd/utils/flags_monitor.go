package utils

import (
	"github.com/urfave/cli/v2"
)

var (
	// Transaction monitoring flags
	TraceLogPath = cli.StringFlag{
		Name:  "monitor.trace-log-path",
		Usage: "Path of trace.log for transaction monitoring",
		Value: "/var/log/op-geth/trace.log",
	}

	EnableTraceLog = cli.BoolFlag{
		Name:  "monitor.enable-trace-log",
		Usage: "Enable full transaction trace log",
		Value: false,
	}

	MonitorLogLevel = cli.StringFlag{
		Name:  "monitor.log-level",
		Usage: "Log level for monitoring (debug, info, warn, error)",
		Value: "info",
	}
)
