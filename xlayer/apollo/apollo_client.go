package apollo

import (
	"fmt"
	"strings"
	"sync"

	"github.com/apolloconfig/agollo/v4"
	"github.com/apolloconfig/agollo/v4/env/config"
	"github.com/apolloconfig/agollo/v4/storage"
	"github.com/ethereum/go-ethereum/log"
	"gopkg.in/yaml.v2"
)

type ApolloService struct {
	config       *config.AppConfig
	client       agollo.Client
	listener     *CustomChangeListener
	namespaceMap map[string]string
	cache        map[string]ConfigValue
	mu           sync.RWMutex
}

// Singleton instance and synchronization
var (
	instance *ApolloService
	initOnce sync.Once
	initErr  error
)

// GetInstance returns the singleton Apollo client instance.
// If no instance exists, it creates one with the provided configuration.
// If an instance already exists, it returns the existing instance.
func TryInitialize(cfg *config.AppConfig) (*ApolloService, error) {
	initOnce.Do(func() {
		// Validate configuration
		if cfg.AppID == "" || cfg.IP == "" || cfg.Cluster == "" || cfg.NamespaceName == "" {
			initErr = fmt.Errorf("apollo enabled but config is not valid, config: %+v", cfg)
			return
		}

		// Start Apollo client
		client, err := agollo.StartWithConfig(func() (*config.AppConfig, error) {
			return cfg, nil
		})

		if err != nil {
			initErr = err
			return
		}

		// Create namespace map
		nsMap := make(map[string]string)
		namespaces := strings.Split(cfg.NamespaceName, ",")
		for _, namespace := range namespaces {
			prefix := getNamespacePrefix(namespace)
			_, found := nsMap[prefix]
			if found {
				initErr = fmt.Errorf("duplicate apollo namespace: %s", prefix)
				return
			}
			nsMap[prefix] = namespace
		}

		// Create and attach change listener
		listener := &CustomChangeListener{}

		// Create cache
		cache := make(map[string]ConfigValue)

		// Create singleton instance
		instance = &ApolloService{
			config:       cfg,
			client:       client,
			listener:     listener,
			namespaceMap: nsMap,
			cache:        cache,
		}

		// Set up the listener reference
		listener.ApolloService = instance
		client.AddChangeListener(listener)

		// Load initial configs into cache
		err = instance.fetchAndUpdateConfigs()
		if err != nil {
			log.Error("[Apollo] Failed to fetch and update configs", "error", err)
			initErr = err
			return
		}

		log.Info("[Apollo] Apollo client initialized successfully", "config", cfg)
	})

	if initErr != nil {
		log.Error("[Apollo] Failed to initialize Apollo client", "error", initErr)
		return nil, initErr
	}

	return instance, nil
}

// OnChange handles configuration changes from Apollo
func (c *CustomChangeListener) OnChange(changeEvent *storage.ChangeEvent) {
	for key, value := range changeEvent.Changes {
		if value.ChangeType == storage.MODIFIED || value.ChangeType == storage.ADDED || value.ChangeType == storage.DELETED {
			log.Info("[Apollo] config updated",
				"namespace", changeEvent.Namespace,
				"key", key,
				"value", value.NewValue)

			err := c.ApolloService.updateCacheFromConfig(changeEvent.Namespace, value.NewValue)
			if err != nil {
				log.Warn("[Apollo] Failed to update cache from config", "error", err, "namespace", changeEvent.Namespace)
				continue
			}
		}
	}
}

// Start starts the Apollo client
func (a *ApolloService) Start() error {
	log.Info("[Apollo] client started")
	return nil
}

// Stop stops the Apollo client
func (a *ApolloService) Stop() error {
	if a != nil && a.client != nil {
		a.client.Close()
		log.Info("[Apollo] client stopped")
	}
	return nil
}

func (a *ApolloService) fetchAndUpdateConfigs() error {
	var err error
	for _, namespace := range a.namespaceMap {
		cache := a.client.GetConfigCache(namespace)
		if cache != nil {
			cache.Range(func(key, value interface{}) bool {
				err = a.updateCacheFromConfig(namespace, value)
				if err != nil {
					log.Error("[Apollo] failed to update cache from config", "value", value, "error", err)
					return false
				}
				return true
			})
		}
	}
	return err
}

func (a *ApolloService) updateCacheFromConfig(namespace string, value interface{}) error {
	strValue, ok := value.(string)
	if !ok {
		err := fmt.Errorf("expected string config, got %T", value)
		log.Error("[Apollo] Invalid config type from Apollo", "namespace", namespace, "type", fmt.Sprintf("%T", value))
		return err
	}

	config := make(map[string]interface{})
	err := yaml.Unmarshal([]byte(strValue), config)
	if err != nil {
		log.Error("[Apollo] Failed to unmarshal config", "namespace", namespace, "error", err)
		return err
	}

	// Create a map of updates first to avoid errors when updating the cache
	updates := make(map[string]ConfigValue, len(config))
	for key, value := range config {
		cacheKey, err := makeCacheKey(namespace, key)
		if err != nil {
			log.Error("[Apollo] Failed to make cache key", "namespace", namespace, "key", key, "error", err)
			return err
		}

		configValue, err := a.GetConfigValueFromType(value)
		if err != nil {
			log.Error("[Apollo] Failed to convert config value", "namespace", namespace, "key", key, "error", err)
			return err
		}
		updates[cacheKey] = configValue
	}

	// Obtaining the lock to update the cache
	a.mu.Lock()
	defer a.mu.Unlock()

	for cacheKey, configValue := range updates {
		a.cache[cacheKey] = configValue
	}
	return nil
}

// GetValue retrieves a configuration value by key from the default namespace
func (a *ApolloService) getCachedConfig(key string) (ConfigValue, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a == nil || a.cache == nil {
		return ConfigValue{}, false
	}
	value, ok := a.cache[key]
	return value, ok
}

func makeCacheKey(namespace string, key string) (string, error) {
	namespacePrefix := getNamespacePrefix(namespace)
	return fmt.Sprintf("%s:%s", namespacePrefix, key), nil
}

type CustomChangeListener struct {
	*ApolloService
}

func (c *CustomChangeListener) OnNewestChange(changeEvent *storage.FullChangeEvent) {
}

// Retrieves configuration from Apollo Config Service with fallback to default value.
// This function attempts to fetch a configuration value from the Apollo Config Service.
// If the value cannot be retrieved (Apollo client not initialized, key doesn't exist,
// or type conversion fails), it returns the provided default value and logs a warning.
func ApolloConfigOr[T any](namespace, key string, defaultVal T) (T, bool) {
	if instance == nil {
		log.Warn("[Apollo] Using default (client not initialized)",
			"namespace", namespace, "key", key, "default", defaultVal)
		return defaultVal, false
	}

	cacheKey, err := makeCacheKey(namespace, key)
	if err != nil {
		log.Error("[Apollo] Failed to make cache key", "namespace", namespace, "key", key, "error", err)
		return defaultVal, false
	}

	configVal, ok := instance.getCachedConfig(cacheKey)
	if !ok {
		log.Warn("[Apollo] Using default (cache key missing)",
			"namespace", namespace, "key", key, "default", defaultVal)
		return defaultVal, false
	}

	result, ok := tryFromConfigValue[T](configVal)
	if !ok {
		log.Warn("[Apollo] Using default (conversion failed)",
			"namespace", namespace, "key", key, "default", defaultVal)
		return defaultVal, false
	}
	return result, ok
}
