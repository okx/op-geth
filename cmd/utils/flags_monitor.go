package utils

import (
	"github.com/urfave/cli/v2"
)

var (
	// TraceLogPath Transaction monitoring flags
	TraceLogPath = &cli.StringFlag{
		Name:  "monitor.trace-log-path",
		Usage: "Path of trace.log for transaction monitoring",
		Value: "/var/log/op-geth/trace.log",
	}

	EnableTraceLog = &cli.BoolFlag{
		Name:  "monitor.enable-trace-log",
		Usage: "Enable full transaction trace log",
		Value: false,
	}
)
