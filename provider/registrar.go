package provider

import (
	"context"
	"time"

	"github.com/nomand-zc/lumin-actool/callback"
	"github.com/nomand-zc/lumin-actool/email"
)

// ProviderRegistrar 供应商账号注册接口。
// 不同的 AI 供应商实现此接口，支持使用邮箱自动注册并获取凭证。
type ProviderRegistrar interface {
	// Provider 返回供应商标识（如 "kiro", "claude", "copilot"）
	Provider() string
	// AuthMethod 返回认证方式描述（如 "oauth-pkce", "device-code", "social-oauth"）
	AuthMethod() string
	// Register 使用邮箱账号注册供应商平台，自动完成 OAuth 流程并返回凭证。
	// callbackSrv 由 Pipeline 统一注入，供应商注册器无需自行管理回调服务器。
	Register(ctx context.Context, emailAccount *email.EmailAccount, callbackSrv callback.CallbackServer, opts ...RegisterOption) (*RegistrationResult, error)
	// Refresh 刷新已有凭证（用于凭证即将过期时）
	Refresh(ctx context.Context, credential map[string]any) (*RegistrationResult, error)
	// Validate 验证凭证是否有效
	Validate(ctx context.Context, credential map[string]any) (bool, error)
}

// RegisterOption 注册选项函数
type RegisterOption func(*RegisterOptions)

// RegisterOptions 注册配置
type RegisterOptions struct {
	Headless bool          // 是否无头浏览器模式
	Timeout  time.Duration // 注册超时时间
	ProxyURL string        // 代理地址
	Metadata map[string]string
}

// ApplyRegisterOptions 应用选项函数到配置
func ApplyRegisterOptions(opts ...RegisterOption) *RegisterOptions {
	o := &RegisterOptions{
		Headless: true,
		Timeout:  5 * time.Minute,
	}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// WithHeadless 设置是否无头浏览器模式
func WithHeadless(headless bool) RegisterOption {
	return func(o *RegisterOptions) {
		o.Headless = headless
	}
}

// WithTimeout 设置注册超时时间
func WithTimeout(timeout time.Duration) RegisterOption {
	return func(o *RegisterOptions) {
		o.Timeout = timeout
	}
}

// WithProxyURL 设置代理地址
func WithProxyURL(proxyURL string) RegisterOption {
	return func(o *RegisterOptions) {
		o.ProxyURL = proxyURL
	}
}

// WithRegisterMetadata 设置额外参数
func WithRegisterMetadata(metadata map[string]string) RegisterOption {
	return func(o *RegisterOptions) {
		o.Metadata = metadata
	}
}
