// Package providers contains the provider registry and helpers for building
// and composing task providers. New integrations register themselves here.
package providers

import (
	"fmt"

	"github.com/ben-fourie/flow-cli/internal/config"
	"github.com/ben-fourie/flow-cli/internal/keychain"
	"github.com/ben-fourie/flow-cli/internal/tasks"
)

// Factory creates a Provider from the application config and keychain.
// It returns (provider, true, nil) when the provider is fully configured,
// (nil, false, nil) when it is absent from the config (skip silently),
// or (nil, false, err) when config is present but invalid.
type Factory func(cfg config.Config, kr *keychain.Keychain) (tasks.Provider, bool, error)

// Registry maps provider names to their Factory functions.
// Register all desired providers before calling Build.
type Registry struct {
	factories map[string]Factory
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{factories: make(map[string]Factory)}
}

// Register associates a provider name with its factory.
// Panics on duplicate registration to surface wiring mistakes early.
func (r *Registry) Register(name string, f Factory) {
	if _, exists := r.factories[name]; exists {
		panic(fmt.Sprintf("providers: duplicate registration for %q", name))
	}
	r.factories[name] = f
}

// Build constructs all providers listed in cfg.Providers.Active, in order.
// Unknown provider names return an error; unconfigured providers are skipped.
func (r *Registry) Build(cfg config.Config, kr *keychain.Keychain) ([]tasks.Provider, error) {
	var ps []tasks.Provider
	for _, name := range cfg.Providers.Active {
		f, ok := r.factories[name]
		if !ok {
			return nil, fmt.Errorf("unknown provider %q — did you register it?", name)
		}
		p, configured, err := f(cfg, kr)
		if err != nil {
			return nil, fmt.Errorf("initialising %s provider: %w", name, err)
		}
		if configured {
			ps = append(ps, p)
		}
	}
	return ps, nil
}
