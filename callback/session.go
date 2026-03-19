package callback

import (
	"fmt"
	"sync"
	"time"
)

// Session 表示一个等待回调的 OAuth 会话
type Session struct {
	// State OAuth state 参数，全局唯一
	State string
	// Provider 供应商标识
	Provider string
	// ResultChan 结果通道，回调到达时将结果发送到此通道
	ResultChan chan *OAuthResult
	// CreatedAt 会话创建时间
	CreatedAt time.Time
	// TTL 会话存活时间
	TTL time.Duration
}

// IsExpired 判断会话是否已过期
func (s *Session) IsExpired() bool {
	return time.Since(s.CreatedAt) > s.TTL
}

// SessionManager 管理 state → Session 的映射。
// 每个供应商注册器在发起 OAuth 时创建一个 session，
// 回调服务器收到回调后通过 state 找到对应 session 并发送结果。
type SessionManager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

// NewSessionManager 创建会话管理器
func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*Session),
	}
}

// Register 注册一个新的等待回调的会话，返回用于接收回调结果的通道
func (m *SessionManager) Register(state, provider string, ttl time.Duration) <-chan *OAuthResult {
	m.mu.Lock()
	defer m.mu.Unlock()

	ch := make(chan *OAuthResult, 1)
	m.sessions[state] = &Session{
		State:      state,
		Provider:   provider,
		ResultChan: ch,
		CreatedAt:  time.Now(),
		TTL:        ttl,
	}
	return ch
}

// Resolve 当回调到达时，通过 state 解析并发送结果
func (m *SessionManager) Resolve(state string, result *OAuthResult) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[state]
	if !exists {
		return fmt.Errorf("no pending session for state: %s", state)
	}

	if session.IsExpired() {
		delete(m.sessions, state)
		return fmt.Errorf("session expired for state: %s", state)
	}

	select {
	case session.ResultChan <- result:
	default:
		// 通道已满，说明已经有结果了
	}

	delete(m.sessions, state)
	return nil
}

// Cleanup 清理过期的会话
func (m *SessionManager) Cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for state, session := range m.sessions {
		if session.IsExpired() {
			close(session.ResultChan)
			delete(m.sessions, state)
		}
	}
}

// PendingCount 返回当前等待中的会话数量
func (m *SessionManager) PendingCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}
