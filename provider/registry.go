package provider

import (
	"fmt"
	"sync"
)

// providerRegistry 供应商注册者全局注册表
var (
	registryMu sync.RWMutex
	registry   = make(map[string]ProviderRegistrar)
)

// Register 注册一个供应商注册者实现
func Register(name string, registrar ProviderRegistrar) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[name] = registrar
}

// Get 根据名称获取供应商注册者
func Get(name string) (ProviderRegistrar, error) {
	registryMu.RLock()
	defer registryMu.RUnlock()

	r, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("provider registrar not found: %s", name)
	}
	return r, nil
}

// List 列出所有已注册的供应商注册者名称
func List() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()

	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	return names
}
