package browser

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
)

// SessionManager 浏览器会话管理器。
// 为每个并发注册任务创建独立的浏览器用户数据目录，
// 确保多个浏览器实例之间的 cookie/session 完全隔离。
type SessionManager struct {
	baseDir string
	counter atomic.Int64
	mu      sync.Mutex
	dirs    []string // 追踪创建的目录，用于清理
}

// NewSessionManager 创建浏览器会话管理器
// baseDir 为浏览器 profile 的基础目录
func NewSessionManager(baseDir string) *SessionManager {
	return &SessionManager{
		baseDir: baseDir,
	}
}

// NewSessionDir 创建一个新的隔离会话目录
func (m *SessionManager) NewSessionDir(provider string) (string, error) {
	idx := m.counter.Add(1)
	dirName := fmt.Sprintf("%s-session-%d", provider, idx)
	dir := filepath.Join(m.baseDir, dirName)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create browser session dir: %w", err)
	}

	m.mu.Lock()
	m.dirs = append(m.dirs, dir)
	m.mu.Unlock()

	return dir, nil
}

// Cleanup 清理所有会话目录
func (m *SessionManager) Cleanup() error {
	m.mu.Lock()
	dirs := make([]string, len(m.dirs))
	copy(dirs, m.dirs)
	m.dirs = nil
	m.mu.Unlock()

	var lastErr error
	for _, dir := range dirs {
		if err := os.RemoveAll(dir); err != nil {
			lastErr = err
		}
	}
	return lastErr
}
