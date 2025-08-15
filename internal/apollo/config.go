package apollo

import (
	"errors"
	"time"
)

type Config struct {
	Enabled     bool
	Endpoint    string
	AppID       string
	Cluster     string
	Namespace   string
	Secret      string
	SyncTimeout time.Duration
}

func (c Config) Check() error {
	if !c.Enabled {
		return nil
	}
	if c.Endpoint == "" {
		return errors.New("apollo endpoint is required when apollo is enabled")
	}
	if c.AppID == "" {
		return errors.New("apollo app-id is required when apollo is enabled")
	}
	return nil
}
