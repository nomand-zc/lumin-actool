package email

import "context"

// EmailProducer 邮箱账号生产接口。
// 不同的邮箱服务提供商实现此接口，支持批量注册新邮箱。
type EmailProducer interface {
	// Provider 返回邮箱提供商标识（如 "outlook", "tempmail"）
	Provider() string
	// Produce 批量生产邮箱账号
	// count: 需要生产的数量
	// 返回成功生产的邮箱列表和遇到的错误
	Produce(ctx context.Context, count int, opts ...ProduceOption) ([]*EmailAccount, error)
	// Verify 验证邮箱账号是否仍然有效
	Verify(ctx context.Context, account *EmailAccount) (bool, error)
}

// ProduceOption 邮箱生产选项函数
type ProduceOption func(*ProduceOptions)

// ProduceOptions 邮箱生产配置
type ProduceOptions struct {
	NamePrefix  string            // 邮箱名前缀
	Domain      string            // 指定域名（某些 provider 支持）
	Concurrency int               // 并发数
	Metadata    map[string]string // 额外参数
}

// ApplyOptions 应用选项函数到配置
func ApplyProduceOptions(opts ...ProduceOption) *ProduceOptions {
	o := &ProduceOptions{
		Concurrency: 1,
	}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// WithNamePrefix 设置邮箱名前缀
func WithNamePrefix(prefix string) ProduceOption {
	return func(o *ProduceOptions) {
		o.NamePrefix = prefix
	}
}

// WithDomain 设置指定域名
func WithDomain(domain string) ProduceOption {
	return func(o *ProduceOptions) {
		o.Domain = domain
	}
}

// WithConcurrency 设置并发数
func WithConcurrency(n int) ProduceOption {
	return func(o *ProduceOptions) {
		if n > 0 {
			o.Concurrency = n
		}
	}
}

// WithProduceMetadata 设置额外参数
func WithProduceMetadata(metadata map[string]string) ProduceOption {
	return func(o *ProduceOptions) {
		o.Metadata = metadata
	}
}
