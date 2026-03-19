package callback

// OAuthResult OAuth 回调统一结果。
// 所有供应商的 OAuth 回调都归一化到这个结构体。
type OAuthResult struct {
	// Provider 供应商标识（如 "kiro", "claude", "copilot"）
	Provider string
	// Code 授权码
	Code string
	// State 状态参数（防 CSRF，同时用于关联注册会话）
	State string
	// Error 错误信息（供应商返回的 error 参数）
	Error string
	// ErrorDescription 错误详细描述
	ErrorDescription string
	// RawQuery 原始查询参数（供特殊供应商使用）
	RawQuery map[string]string
}

// IsSuccess 判断回调是否成功
func (r *OAuthResult) IsSuccess() bool {
	return r.Error == "" && r.Code != ""
}
