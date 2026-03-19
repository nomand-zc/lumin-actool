package callback

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// CallbackServer OAuth 回调服务器接口。
// 统一接收所有供应商的 OAuth 授权回调，
// 通过 state 参数将回调结果路由到对应的注册会话。
type CallbackServer interface {
	// Start 启动回调服务器
	Start() error
	// Stop 优雅关闭回调服务器
	Stop(ctx context.Context) error
	// Port 返回实际监听端口
	Port() int
	// BaseURL 返回回调基础 URL（如 http://127.0.0.1:19800）
	BaseURL() string
	// CallbackURL 返回指定供应商的回调 URL
	// 例如：http://127.0.0.1:19800/callback/kiro
	CallbackURL(provider string) string
	// RegisterSession 注册一个等待回调的会话。
	// 供应商注册器在发起 OAuth 前调用此方法。
	RegisterSession(state, provider string, ttl time.Duration) <-chan *OAuthResult
}

// DefaultCallbackServer 默认回调服务器实现
type DefaultCallbackServer struct {
	port     int
	listener net.Listener
	server   *http.Server
	sessions *SessionManager
	mu       sync.Mutex
	running  bool
}

// NewCallbackServer 创建默认回调服务器。
// port 指定监听端口，0 表示自动分配。
func NewCallbackServer(port int) *DefaultCallbackServer {
	return &DefaultCallbackServer{
		port:     port,
		sessions: NewSessionManager(),
	}
}

func (s *DefaultCallbackServer) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("callback server is already running")
	}

	addr := fmt.Sprintf("127.0.0.1:%d", s.port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		// 端口被占用时尝试自动分配
		listener, err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return fmt.Errorf("failed to start callback server: %w", err)
		}
	}
	s.listener = listener
	s.port = listener.Addr().(*net.TCPAddr).Port

	mux := http.NewServeMux()
	// 统一回调路由：/callback/{provider}
	mux.HandleFunc("/callback/", s.handleCallback)
	// 健康检查
	mux.HandleFunc("/health", s.handleHealth)

	s.server = &http.Server{
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	s.running = true

	go func() {
		if err := s.server.Serve(s.listener); err != nil && err != http.ErrServerClosed {
			// 服务器异常退出，静默处理
		}
	}()

	// 启动定期清理过期会话的 goroutine
	go s.cleanupLoop()

	return nil
}

func (s *DefaultCallbackServer) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running || s.server == nil {
		return nil
	}

	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err := s.server.Shutdown(shutdownCtx)
	s.running = false
	s.server = nil
	return err
}

func (s *DefaultCallbackServer) Port() int {
	return s.port
}

func (s *DefaultCallbackServer) BaseURL() string {
	return fmt.Sprintf("http://127.0.0.1:%d", s.port)
}

func (s *DefaultCallbackServer) CallbackURL(provider string) string {
	return fmt.Sprintf("%s/callback/%s", s.BaseURL(), provider)
}

func (s *DefaultCallbackServer) RegisterSession(state, provider string, ttl time.Duration) <-chan *OAuthResult {
	return s.sessions.Register(state, provider, ttl)
}

// handleCallback 统一处理所有供应商的 OAuth 回调。
// 路由格式：GET /callback/{provider}?code=xxx&state=xxx&error=xxx
func (s *DefaultCallbackServer) handleCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 从路径中提取 provider: /callback/kiro → kiro
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/callback/"), "/")
	providerName := parts[0]
	if providerName == "" {
		http.Error(w, "Missing provider in path", http.StatusBadRequest)
		return
	}

	query := r.URL.Query()
	result := &OAuthResult{
		Provider:         providerName,
		Code:             strings.TrimSpace(query.Get("code")),
		State:            strings.TrimSpace(query.Get("state")),
		Error:            strings.TrimSpace(query.Get("error")),
		ErrorDescription: strings.TrimSpace(query.Get("error_description")),
		RawQuery:         make(map[string]string),
	}

	// 保存所有原始查询参数
	for key, values := range query {
		if len(values) > 0 {
			result.RawQuery[key] = values[0]
		}
	}

	// 通过 state 解析到对应会话
	if result.State != "" {
		if err := s.sessions.Resolve(result.State, result); err != nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, htmlError, "Invalid or expired session")
			return
		}
	}

	// 返回结果页面
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if result.IsSuccess() {
		fmt.Fprint(w, htmlSuccess)
	} else {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, htmlError, result.Error)
	}
}

func (s *DefaultCallbackServer) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (s *DefaultCallbackServer) cleanupLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		s.mu.Lock()
		running := s.running
		s.mu.Unlock()
		if !running {
			return
		}
		s.sessions.Cleanup()
	}
}

// HTML 模板
const htmlSuccess = `<!DOCTYPE html>
<html><head><title>Authorization Successful</title></head>
<body style="font-family:sans-serif;text-align:center;padding-top:50px;">
<h1>✅ Authorization Successful!</h1>
<p>You can close this window and return to the terminal.</p>
<script>setTimeout(function(){window.close();},3000);</script>
</body></html>`

const htmlError = `<!DOCTYPE html>
<html><head><title>Authorization Failed</title></head>
<body style="font-family:sans-serif;text-align:center;padding-top:50px;">
<h1>❌ Authorization Failed</h1>
<p>Error: %s</p>
<p>Please check the terminal for details.</p>
</body></html>`
