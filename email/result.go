package email

import "time"

// EmailAccount 表示一个已生产的邮箱账号
type EmailAccount struct {
	// Email 邮箱地址
	Email string `json:"email"`
	// Password 邮箱密码
	Password string `json:"password"`
	// Provider 邮箱服务提供商标识（如 "outlook", "tempmail", "gmail-alias"）
	Provider string `json:"provider"`
	// CreatedAt 创建时间
	CreatedAt time.Time `json:"created_at"`
	// Metadata 扩展元数据（如恢复邮箱、手机号等）
	Metadata map[string]any `json:"metadata,omitempty"`
}
