package apollo

import (
	"errors"
	"fmt"

	"github.com/apolloconfig/agollo/v4"
	"github.com/apolloconfig/agollo/v4/env/config"
	"github.com/ethereum/go-ethereum/log"
)

type Config struct {
	AppID         string
	IP            string
	Cluster       string
	NamespaceName string
}

type Client struct {
	config   Config
	client   agollo.Client
	listener *CustomChangeListener
}

func New(cfg Config) (*Client, error) {
	if cfg.AppID == "" || cfg.IP == "" || cfg.Cluster == "" || cfg.NamespaceName == "" {
		return nil, errors.New(fmt.Sprintf("apollo enabled but config is not valid, config: %+v", cfg))
	}
	c := &config.AppConfig{
		AppID:         cfg.AppID,
		Cluster:       cfg.Cluster,
		IP:            cfg.IP,
		NamespaceName: cfg.NamespaceName,
	}
	client, err := agollo.StartWithConfig(func() (*config.AppConfig, error) {
		return c, nil
	})
	if err != nil {
		return nil, err
	}
	listener := &CustomChangeListener{}
	client.AddChangeListener(listener)

	return &Client{config: cfg, client: client, listener: listener}, nil
}

func (client *Client) Start() error {
	log.Info("Apollo client started")
	return nil
}

func (client *Client) Stop() error {
	client.client.Close()
	log.Info("Apollo client stopped")
	return nil
}
