package apollo

import (
	"fmt"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/log"
)

const (
	// Standard key name for configuration content in Apollo
	ContentKey = "content"
)

// ConfigManager provides a modular way to handle Apollo configuration changes
// It parses the configuration content and dispatches to registered handlers
type ConfigManager struct {
	logger log.Logger
	mu     sync.RWMutex

	// Map of configuration key to handler function
	configHandlers map[string]ConfigItemHandler
}

// NewConfigManager creates a new configuration manager
func NewConfigManager(logger log.Logger) *ConfigManager {
	return &ConfigManager{
		logger:         logger,
		configHandlers: make(map[string]ConfigItemHandler),
	}
}

// RegisterConfigHandler registers a handler for a specific configuration key
func (cm *ConfigManager) RegisterConfigHandler(key string, handler ConfigItemHandler) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.configHandlers[key] = handler
	cm.logger.Debug("Registered configuration handler", "key", key)
}

// HandleConfigChange implements the NamespaceHandler interface
// This method parses the configuration content and dispatches to registered handlers
func (cm *ConfigManager) HandleConfigChange(namespace, key, value string) error {
	if key != ContentKey {
		cm.logger.Debug("Ignoring non-content configuration change",
			"namespace", namespace, "key", key)
		return nil
	}

	cm.logger.Info("Processing configuration content change",
		"namespace", namespace, "content_length", len(value))

	return cm.parseAndApplyConfigContent(value)
}

// parseAndApplyConfigContent parses the Apollo configuration content string and applies relevant changes
func (cm *ConfigManager) parseAndApplyConfigContent(content string) error {
	// Parse the configuration content into key-value pairs
	configMap := cm.parseConfigContent(content)

	cm.logger.Info("Processing configuration changes", "total_configs", len(configMap))

	var errors []string
	appliedCount := 0

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// Process each configuration item
	for key, value := range configMap {
		if handler, exists := cm.configHandlers[key]; exists {
			if err := handler(value); err != nil {
				errors = append(errors, fmt.Sprintf("%s: %v", key, err))
				cm.logger.Error("Failed to apply configuration item", "key", key, "value", value, "error", err)
			} else {
				appliedCount++
				cm.logger.Debug("Successfully applied configuration item", "key", key, "value", value)
			}
		} else {
			cm.logger.Debug("No handler registered for configuration key", "key", key, "value", value)
		}
	}

	cm.logger.Info("Configuration processing completed",
		"applied", appliedCount,
		"unhandled", len(configMap)-appliedCount-len(errors),
		"failed", len(errors),
		"total", len(configMap))

	if len(errors) > 0 {
		return fmt.Errorf("failed to apply some configurations: %s", strings.Join(errors, "; "))
	}

	return nil
}

// parseConfigContent parses a configuration content string into a map of key-value pairs
// Expected format: "key1: value1\nkey2: value2\nkey3: value3\n"
func (cm *ConfigManager) parseConfigContent(content string) map[string]string {
	configMap := make(map[string]string)

	// Split by newlines
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Split by first colon to separate key and value
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			cm.logger.Warn("Invalid configuration line format, skipping", "line", line)
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		configMap[key] = value
		cm.logger.Debug("Parsed configuration item", "key", key, "value", value)
	}

	cm.logger.Debug("Parsed configuration content", "total_items", len(configMap))
	return configMap
}
