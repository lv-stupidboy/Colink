package im

import (
	"context"
	"fmt"
	"sync"

	"go.uber.org/zap"
)

// IMPlatformConfig holds configuration for creating an IM adapter.
type IMPlatformConfig struct {
	Platform    string         // e.g., "feishu", "slack"
	LarkCLIPath string         // Path to lark-cli executable (for Feishu)
	Extra       map[string]any // Platform-specific configuration
}

// AdapterFactory is a function type that creates an IMAdapter from configuration.
type AdapterFactory func(cfg IMPlatformConfig, logger *zap.Logger) (IMAdapter, error)

// Registry manages adapter factories for different IM platforms.
type Registry struct {
	mu        sync.RWMutex
	factories map[string]AdapterFactory
}

// NewRegistry creates a new adapter registry.
func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[string]AdapterFactory),
	}
}

// Register registers an adapter factory for a platform type.
// Panics if the platform is already registered.
func (r *Registry) Register(platformType string, factory AdapterFactory) {
	if factory == nil {
		panic(fmt.Sprintf("adapter factory for platform %q cannot be nil", platformType))
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.factories[platformType]; exists {
		panic(fmt.Sprintf("adapter factory for platform %q already registered", platformType))
	}

	r.factories[platformType] = factory
}

// Create creates an IMAdapter for the given configuration.
// Returns an error if the platform is not registered.
func (r *Registry) Create(cfg IMPlatformConfig, logger *zap.Logger) (IMAdapter, error) {
	r.mu.RLock()
	factory, exists := r.factories[cfg.Platform]
	r.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no adapter factory registered for platform %q", cfg.Platform)
	}

	return factory(cfg, logger)
}

// MustCreate creates an IMAdapter, panicking if creation fails.
// Useful for initialization code where errors should be fatal.
func (r *Registry) MustCreate(cfg IMPlatformConfig, logger *zap.Logger) IMAdapter {
	adapter, err := r.Create(cfg, logger)
	if err != nil {
		panic(fmt.Sprintf("failed to create adapter for platform %q: %v", cfg.Platform, err))
	}
	return adapter
}

// NewFeishuAdapterFactory returns a factory function for creating Feishu adapters.
func NewFeishuAdapterFactory() AdapterFactory {
	return func(cfg IMPlatformConfig, logger *zap.Logger) (IMAdapter, error) {
		if cfg.LarkCLIPath == "" {
			return nil, fmt.Errorf("lark CLI path is required for Feishu adapter")
		}

		larkClient := NewLarkCLIClient(cfg.LarkCLIPath, logger)
		adapter := NewFeishuAdapter(larkClient, logger)

		if err := adapter.CheckHealth(context.Background()); err != nil {
			logger.Warn("Feishu adapter health check failed", zap.Error(err))
		}

		return adapter, nil
	}
}
