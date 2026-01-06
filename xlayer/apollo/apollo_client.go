package apollo

import (
	"fmt"
	"strings"
	"sync"

	"github.com/apolloconfig/agollo/v4"
	"github.com/apolloconfig/agollo/v4/env/config"
	"github.com/apolloconfig/agollo/v4/storage"
	"github.com/ethereum/go-ethereum/log"
	"github.com/urfave/cli/v2"
)

// Client is the apollo client
type Client struct {
	config       *config.AppConfig
	client       agollo.Client
	listener     *CustomChangeListener
	namespaceMap map[string]string
	flags        []cli.Flag
}

// Singleton instance and synchronization
var (
	instance *Client
	mu       sync.RWMutex
)

// GetInstance returns the singleton Apollo client instance.
// If no instance exists, it creates one with the provided configuration.
// If an instance already exists, it returns the existing instance.
// To reinitialize with new config, call ResetInstance() first.
func GetInstance(cfg *config.AppConfig, flags []cli.Flag) (*Client, error) {
	mu.RLock()
	if instance != nil {
		mu.RUnlock()
		return instance, nil
	}
	mu.RUnlock()

	mu.Lock()
	defer mu.Unlock()

	if instance != nil {
		return instance, nil
	}

	// Validate configuration
	if cfg.AppID == "" || cfg.IP == "" || cfg.Cluster == "" || cfg.NamespaceName == "" {
		return nil, fmt.Errorf("apollo enabled but config is not valid, config: %+v", cfg)
	}

	// Start Apollo client
	client, err := agollo.StartWithConfig(func() (*config.AppConfig, error) {
		return cfg, nil
	})
	if err != nil {
		return nil, err
	}

	// Create namespace map
	nsMap := make(map[string]string)
	namespaces := strings.Split(cfg.NamespaceName, ",")
	for _, namespace := range namespaces {
		prefix, err := getNamespacePrefix(namespace)
		if err != nil {
			return nil, fmt.Errorf("failed to get namespace prefix: %v", err)
		}

		_, found := nsMap[prefix]
		if found {
			//return nil, fmt.Errorf("duplicate apollo namespace prefix: %s", prefix)
			return nil, fmt.Errorf("duplicate apollo namespace: %s", prefix)
		}
		nsMap[prefix] = namespace
	}

	// Create and attach change listener
	listener := &CustomChangeListener{}

	// Create singleton instance
	instance = &Client{
		config:       cfg,
		client:       client,
		listener:     listener,
		namespaceMap: nsMap,
		flags:        flags,
	}

	// Set up the listener reference
	listener.Client = instance
	client.AddChangeListener(listener)

	log.Info("Apollo client initialized successfully", "config", cfg)

	return instance, nil
}

// ResetInstance closes the current instance and allows a new one to be created.
// This is useful for testing or when configuration changes require a restart.
func ResetInstance() error {
	mu.Lock()
	defer mu.Unlock()

	if instance != nil {
		if err := instance.Stop(); err != nil {
			log.Warn("Error stopping Apollo client during reset", "error", err)
		}
		instance = nil
	}

	return nil
}

// IsInitialized returns true if the singleton instance has been created.
func IsInitialized() bool {
	mu.RLock()
	defer mu.RUnlock()
	return instance != nil
}

// Start starts the Apollo client
func (c *Client) Start() error {
	log.Info("Apollo client started")
	return nil
}

// Stop stops the Apollo client
func (c *Client) Stop() error {
	if c != nil && c.client != nil {
		c.client.Close()
		log.Info("Apollo client stopped")
	}
	return nil
}

// GetConfig returns the current configuration of the client
func (c *Client) GetConfig() *config.AppConfig {
	return c.config
}

// GetValue retrieves a configuration value by key from the default namespace
func (c *Client) GetValue(key string) string {
	if c == nil || c.client == nil {
		return ""
	}
	return c.client.GetStringValue(key, c.config.NamespaceName)
}

// GetRawClient returns the underlying agollo client for advanced usage
func (c *Client) GetRawClient() agollo.Client {
	return c.client
}

// LoadConfig loads the config from Apollo
func (c *Client) LoadConfig() (loaded bool) {
	if c == nil || c.client == nil {
		return false
	}

	for prefix, namespace := range c.namespaceMap {
		cache := c.client.GetConfigCache(namespace)
		if cache != nil {
			cache.Range(func(key, value interface{}) bool {
				loaded = true
				// Use handler to load config if available
				if c.listener != nil && c.listener.handler != nil {
					ctx, _, err := c.GetConfigContext(value)
					if err != nil {
						log.Error(fmt.Sprintf("load config from apollo config failed, err: %v", err))
						return true
					}
					c.listener.handler.LoadConfig(prefix, ctx)
				}
				return true
			})
		}
	}
	return loaded
}

func (c *Client) AddHandler(handler CustomHandler) {
	c.listener.handler = handler
}

type CustomHandler interface {
	HandleConfigChange(prefix string, ctx *cli.Context, key string, value *storage.ConfigChange)
	LoadConfig(prefix string, ctx *cli.Context) // Add config loading interface
}

type CustomChangeListener struct {
	*Client
	handler CustomHandler
}

// OnChange handles configuration changes from Apollo
func (c *CustomChangeListener) OnChange(changeEvent *storage.ChangeEvent) {
	for key, value := range changeEvent.Changes {
		if value.ChangeType == storage.MODIFIED || value.ChangeType == storage.ADDED {
			if value.ChangeType == storage.ADDED {
				value.OldValue = ""
				log.Info("Apollo config added",
					"namespace", changeEvent.Namespace,
					"key", key,
					"value", value.NewValue)
			}

			suffix, err := getNamespaceSuffix(changeEvent.Namespace)
			if err != nil {
				log.Warn(fmt.Sprintf("not processing change event: %v", err))
				continue
			}
			switch suffix {
			case Halt:
				c.fireHalt(key, value)
				continue
			}

			prefix, err := getNamespacePrefix(changeEvent.Namespace)
			if err != nil {
				log.Warn("Failed to get namespace prefix", "error", err, "namespace", changeEvent.Namespace)
				continue
			}

			ctx, _, err := c.GetConfigContext(value.NewValue)
			if err != nil {
				log.Warn("Failed to get config context", "error", err, "namespace", changeEvent.Namespace)
				continue
			}

			// Handle configuration changes based on prefix
			if c.handler != nil {
				c.handler.HandleConfigChange(prefix, ctx, key, value)
			}
		}
	}
}

// OnNewestChange is the newest change listener
func (c *CustomChangeListener) OnNewestChange(event *storage.FullChangeEvent) {
}
