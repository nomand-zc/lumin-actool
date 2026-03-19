package provider

import "time"

// RegistrationResult 供应商注册结果
type RegistrationResult struct {
	// Provider 供应商类型（如 "kiro", "claude", "copilot"）
	Provider string `json:"provider"`
	// Email 注册使用的邮箱
	Email string `json:"email"`
	// Credential 凭证数据（JSON 序列化后即为 lumin-acpool 可导入的格式）
	Credential map[string]any `json:"credential"`
	// UserInfo 用户信息
	UserInfo *UserInfo `json:"user_info,omitempty"`
	// ExpiresAt 凭证过期时间
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	// RegisteredAt 注册时间
	RegisteredAt time.Time `json:"registered_at"`
	// Metadata 扩展元数据
	Metadata map[string]any `json:"metadata,omitempty"`
}

// UserInfo 注册后获取的用户信息
type UserInfo struct {
	ID    string `json:"id,omitempty"`
	Email string `json:"email,omitempty"`
	Name  string `json:"name,omitempty"`
}
