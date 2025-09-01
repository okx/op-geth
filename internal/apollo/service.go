package apollo

import (
	"fmt"

	"github.com/ethereum/go-ethereum/log"
)

// Service implements the node.Lifecycle interface for Apollo client
type Service struct {
	client        *Client
	configManager *ConfigManager
	logger        log.Logger
	config        Config
}

// NewService creates a new Apollo service
func NewService(config Config) *Service {
	if err := config.Check(); err != nil {
		log.Error("Invalid Apollo configuration", "error", err)
		return &Service{config: config}
	}

	logger := log.New("module", "apollo")

	var client *Client
	var configManager *ConfigManager

	if config.Enabled {
		clientConfig, err := NewClient(config, logger)
		if err != nil {
			log.Error("Failed to create Apollo client", "error", err)
			return &Service{config: config}
		}
		client = clientConfig
		configManager = NewConfigManager(logger)

		// Register the config manager as a namespace handler for the configured namespace
		client.RegisterNamespaceHandler(config.Namespace, configManager)
	}

	return &Service{
		client:        client,
		configManager: configManager,
		logger:        logger,
		config:        config,
	}
}

// Start implements the node.Lifecycle interface
func (s *Service) Start() error {
	if !s.config.Enabled {
		s.logger.Info("Apollo service is disabled")
		return nil
	}

	if s.client == nil || s.configManager == nil {
		s.logger.Error("Apollo client or config manager not initialized")
		return fmt.Errorf("apollo service not properly initialized")
	}

	s.logger.Info("Apollo configuration service started successfully")
	return nil
}

// Stop implements the node.Lifecycle interface
func (s *Service) Stop() error {
	if !s.config.Enabled || s.client == nil {
		return nil
	}

	s.logger.Info("Stopping Apollo configuration service")
	// Apollo client cleanup is handled automatically
	return nil
}

// GetConfigManager returns the config manager for external use
func (s *Service) GetConfigManager() *ConfigManager {
	return s.configManager
}

// RegisterConfigHandler registers a handler for a specific configuration key
// This allows other modules to handle specific Apollo configuration changes
func (s *Service) RegisterConfigHandler(key string, handler ConfigItemHandler) {
	if s.configManager != nil {
		s.logger.Info("Registering config handler", "key", key)
		s.configManager.RegisterConfigHandler(key, handler)
	}
}

// RegisterTxPoolSubscriber is a convenience method to register TxPool configuration handlers
func (s *Service) RegisterTxPoolSubscriber(backend interface{}) {
	subscriber := NewTxPoolConfigSubscriber(backend)

	// Register handlers for each TxPool configuration key
	for _, key := range subscriber.GetSupportedKeys() {
		s.RegisterConfigHandler(key, func(value string) error {
			return subscriber.HandleConfigItem(key, value)
		})
	}

	s.logger.Info("Registered TxPool configuration handlers", "keys", len(subscriber.GetSupportedKeys()))
}

// RegisterRollupSubscriber is a convenience method to register Rollup configuration handlers
func (s *Service) RegisterRollupSubscriber(backend interface{}) {
	subscriber := NewRollupConfigSubscriber(backend)
	if subscriber == nil {
		s.logger.Error("Failed to create Rollup configuration subscriber")
		return
	}

	// Register handlers for each TxGossip configuration key
	for _, key := range subscriber.GetSupportedKeys() {
		s.RegisterConfigHandler(key, func(value string) error {
			return subscriber.HandleConfigItem(key, value)
		})
	}

	s.logger.Info("Registered TxGossip configuration handlers", "keys", len(subscriber.GetSupportedKeys()))
}
