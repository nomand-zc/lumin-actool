package browser

import "context"

// Page 浏览器页面抽象
type Page interface {
	// Navigate 导航到指定 URL
	Navigate(ctx context.Context, url string) error
	// WaitForSelector 等待某个 CSS 选择器出现
	WaitForSelector(ctx context.Context, selector string) error
	// Click 点击元素
	Click(ctx context.Context, selector string) error
	// Type 在输入框中输入文本
	Type(ctx context.Context, selector string, text string) error
	// GetURL 获取当前页面 URL
	GetURL() string
	// GetContent 获取页面内容
	GetContent(ctx context.Context) (string, error)
	// WaitForNavigation 等待页面导航完成
	WaitForNavigation(ctx context.Context) error
	// EvalJS 执行 JavaScript
	EvalJS(ctx context.Context, expression string) (any, error)
	// Screenshot 截图（用于调试）
	Screenshot(ctx context.Context, path string) error
	// Close 关闭页面
	Close() error
}

// BrowserAutomation 浏览器自动化接口。
// 提供无头浏览器的生命周期管理和页面操作能力。
type BrowserAutomation interface {
	// Launch 启动浏览器实例
	Launch(ctx context.Context, opts ...LaunchOption) error
	// NewPage 创建新页面（标签页）
	NewPage(ctx context.Context) (Page, error)
	// Close 关闭浏览器实例
	Close() error
}

// LaunchOption 启动选项函数
type LaunchOption func(*LaunchOptions)

// LaunchOptions 浏览器启动配置
type LaunchOptions struct {
	Headless    bool   // 是否无头模式
	ProxyURL    string // 代理地址
	UserDataDir string // 用户数据目录（隔离 session）
	BinaryPath  string // Chrome 二进制路径
}

// ApplyLaunchOptions 应用启动选项
func ApplyLaunchOptions(opts ...LaunchOption) *LaunchOptions {
	o := &LaunchOptions{
		Headless: true,
	}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// WithBrowserHeadless 设置是否无头模式
func WithBrowserHeadless(headless bool) LaunchOption {
	return func(o *LaunchOptions) {
		o.Headless = headless
	}
}

// WithBrowserProxy 设置代理地址
func WithBrowserProxy(proxyURL string) LaunchOption {
	return func(o *LaunchOptions) {
		o.ProxyURL = proxyURL
	}
}

// WithUserDataDir 设置用户数据目录
func WithUserDataDir(dir string) LaunchOption {
	return func(o *LaunchOptions) {
		o.UserDataDir = dir
	}
}

// WithBinaryPath 设置 Chrome 二进制路径
func WithBinaryPath(path string) LaunchOption {
	return func(o *LaunchOptions) {
		o.BinaryPath = path
	}
}
