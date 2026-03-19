package email

import (
	"fmt"
	"sync"
)

// producerRegistry 邮箱生产者全局注册表
var (
	registryMu sync.RWMutex
	registry   = make(map[string]EmailProducer)
)

// Register 注册一个邮箱生产者实现
func Register(name string, producer EmailProducer) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[name] = producer
}

// Get 根据名称获取邮箱生产者
func Get(name string) (EmailProducer, error) {
	registryMu.RLock()
	defer registryMu.RUnlock()

	p, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("email producer not found: %s", name)
	}
	return p, nil
}

// List 列出所有已注册的邮箱生产者名称
func List() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()

	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	return names
}
