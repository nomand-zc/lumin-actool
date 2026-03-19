package storage

import "context"

// PipelineState 流水线状态
type PipelineState struct {
	ID              string `json:"id"`
	EmailProducer   string `json:"email_producer"`
	ProviderRegistrar string `json:"provider_registrar"`
	Count           int    `json:"count"`
	CompletedCount  int    `json:"completed_count"`
	FailedCount     int    `json:"failed_count"`
	Status          string `json:"status"` // "running", "completed", "failed", "paused"
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

// EmailAccountState 已生产邮箱的持久化状态
type EmailAccountState struct {
	ID         string `json:"id"`
	PipelineID string `json:"pipeline_id"`
	Email      string `json:"email"`
	Password   string `json:"password"`
	Provider   string `json:"provider"`
	Status     string `json:"status"` // "available", "used", "invalid"
	CreatedAt  string `json:"created_at"`
}

// CredentialState 已获取凭证的持久化状态
type CredentialState struct {
	ID             string `json:"id"`
	PipelineID     string `json:"pipeline_id"`
	ProviderType   string `json:"provider_type"`
	Email          string `json:"email"`
	CredentialJSON string `json:"credential_json"`
	Status         string `json:"status"` // "active", "expired", "exported"
	CreatedAt      string `json:"created_at"`
	ExpiresAt      string `json:"expires_at,omitempty"`
}

// EmailFilter 邮箱账号过滤条件
type EmailFilter struct {
	PipelineID string
	Provider   string
	Status     string
}

// CredentialFilter 凭证过滤条件
type CredentialFilter struct {
	PipelineID   string
	ProviderType string
	Status       string
}

// StateStorage 状态持久化接口。
// 用于持久化流水线执行状态，支持断点续传。
type StateStorage interface {
	// SavePipeline 保存流水线状态
	SavePipeline(ctx context.Context, pipeline *PipelineState) error
	// GetPipeline 获取流水线状态
	GetPipeline(ctx context.Context, id string) (*PipelineState, error)
	// ListPipelines 列出所有流水线
	ListPipelines(ctx context.Context) ([]*PipelineState, error)
	// SaveEmailAccount 保存已生产的邮箱账号
	SaveEmailAccount(ctx context.Context, account *EmailAccountState) error
	// ListEmailAccounts 列出邮箱账号（支持按状态过滤）
	ListEmailAccounts(ctx context.Context, filter *EmailFilter) ([]*EmailAccountState, error)
	// SaveCredential 保存已获取的凭证
	SaveCredential(ctx context.Context, cred *CredentialState) error
	// ListCredentials 列出凭证（支持按供应商过滤）
	ListCredentials(ctx context.Context, filter *CredentialFilter) ([]*CredentialState, error)
	// Close 关闭存储
	Close() error
}
