package apollo

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/apolloconfig/agollo/v4"
	"github.com/apolloconfig/agollo/v4/env/config"
	"github.com/apolloconfig/agollo/v4/storage"
	"github.com/ethereum/go-ethereum/log"
)

type Client struct {
	client *agollo.Client
	config Config
	logger log.Logger

	// Direct handlers for specific namespaces
	mu                sync.RWMutex
	namespaceHandlers map[string]NamespaceHandler
}

// ConfigItemHandler handles a specific configuration item change
type ConfigItemHandler func(value string) error

// NamespaceHandler handles all configuration changes for a namespace (legacy interface)
type NamespaceHandler interface {
	HandleConfigChange(namespace, key, value string) error
}

func NewClient(cfg Config, logger log.Logger) (*Client, error) {
	if !cfg.Enabled {
		return &Client{config: cfg, logger: logger}, nil
	}

	apolloConfig := &config.AppConfig{
		AppID:         cfg.AppID,
		Cluster:       cfg.Cluster,
		NamespaceName: cfg.Namespace,
		IP:            cfg.Endpoint,
		Secret:        cfg.Secret,
	}

	client, err := agollo.StartWithConfig(func() (*config.AppConfig, error) {
		return apolloConfig, nil
	})
	if err != nil {
		return nil, err
	}

	c := &Client{
		client:            client,
		config:            cfg,
		logger:            logger,
		namespaceHandlers: make(map[string]NamespaceHandler),
	}

	// Register configuration change listener
	client.AddChangeListener(c)

	return c, nil
}

func (c *Client) Enabled() bool {
	return c.config.Enabled && c.client != nil
}

func (c *Client) GetString(key, defaultValue string) string {
	if !c.Enabled() {
		return defaultValue
	}
	return c.client.GetStringValue(key, defaultValue)
}

func (c *Client) GetInt(key string, defaultValue int) int {
	if !c.Enabled() {
		return defaultValue
	}
	return c.client.GetIntValue(key, defaultValue)
}

func (c *Client) GetBool(key string, defaultValue bool) bool {
	if !c.Enabled() {
		return defaultValue
	}
	return c.client.GetBoolValue(key, defaultValue)
}

func (c *Client) GetDuration(key string, defaultValue time.Duration) time.Duration {
	if !c.Enabled() {
		return defaultValue
	}
	value, err := time.ParseDuration(key)
	if err != nil {
		return defaultValue
	}
	return value
}

func (c *Client) GetConfig(key string, target interface{}) error {
	if !c.Enabled() {
		return nil
	}
	value := c.client.GetStringValue(key, "")
	if value == "" {
		return nil
	}
	return json.Unmarshal([]byte(value), target)
}

// Configuration change listener - implements storage.ChangeListener interface
func (c *Client) OnChange(event *storage.ChangeEvent) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for key, value := range event.Changes {
		if value.ChangeType == storage.MODIFIED || value.ChangeType == storage.ADDED {
			if value.ChangeType == storage.ADDED {
				value.OldValue = nil
			}

			// Direct namespace handler - more efficient than callbacks
			if handler, exists := c.namespaceHandlers[event.Namespace]; exists {
				if newValue, ok := value.NewValue.(string); ok {
					c.logger.Info("Calling namespace handler",
						"namespace", event.Namespace,
						"key", key,
						"oldValue", value.OldValue,
						"newValue", newValue)
					if err := handler.HandleConfigChange(event.Namespace, key, newValue); err != nil {
						c.logger.Error("Failed to handle config change",
							"namespace", event.Namespace,
							"key", key,
							"error", err)
					}
				}
			} else {
				c.logger.Warn("No namespace handler found",
					"namespace", event.Namespace,
					"value", value.NewValue)
			}
		}
	}
}

// OnNewestChange implements the storage.ChangeListener interface
func (c *Client) OnNewestChange(event *storage.FullChangeEvent) {
}

func (c *Client) RegisterNamespaceHandler(namespace string, handler NamespaceHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.namespaceHandlers[namespace] = handler
}

// CreateConfigManager creates and registers a ConfigManager for a specific namespace
// This is the recommended way for components to handle Apollo configuration changes
func (c *Client) CreateConfigManager(namespace string) *ConfigManager {
	configManager := NewConfigManager(c.logger)
	c.RegisterNamespaceHandler(namespace, configManager)
	c.logger.Info("Created and registered config manager", "namespace", namespace)
	return configManager
}
