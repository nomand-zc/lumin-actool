package cdp

import (
	"context"
	"fmt"
	"sync"

	"github.com/nomand-zc/lumin-actool/browser"
)

// Automation 基于 Chrome DevTools Protocol 的浏览器自动化实现。
// 使用 chromedp 库与 Chrome/Chromium 通信。
type Automation struct {
	mu        sync.Mutex
	allocCtx  context.Context
	cancelFn  context.CancelFunc
	opts      *browser.LaunchOptions
	launched  bool
}

// New 创建 CDP 浏览器自动化实例
func New() browser.BrowserAutomation {
	return &Automation{}
}

func (a *Automation) Launch(ctx context.Context, opts ...browser.LaunchOption) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.launched {
		return fmt.Errorf("browser already launched")
	}

	a.opts = browser.ApplyLaunchOptions(opts...)

	// TODO: 使用 chromedp 初始化浏览器实例
	// allocOpts := append(chromedp.DefaultExecAllocatorOptions[:],
	//     chromedp.Flag("headless", a.opts.Headless),
	// )
	// if a.opts.ProxyURL != "" {
	//     allocOpts = append(allocOpts, chromedp.ProxyServer(a.opts.ProxyURL))
	// }
	// if a.opts.UserDataDir != "" {
	//     allocOpts = append(allocOpts, chromedp.UserDataDir(a.opts.UserDataDir))
	// }
	// if a.opts.BinaryPath != "" {
	//     allocOpts = append(allocOpts, chromedp.ExecPath(a.opts.BinaryPath))
	// }
	// a.allocCtx, a.cancelFn = chromedp.NewExecAllocator(ctx, allocOpts...)

	a.allocCtx, a.cancelFn = context.WithCancel(ctx)
	a.launched = true
	return nil
}

func (a *Automation) NewPage(ctx context.Context) (browser.Page, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.launched {
		return nil, fmt.Errorf("browser not launched, call Launch() first")
	}

	// TODO: 使用 chromedp.NewContext 创建新的浏览器页面
	// tabCtx, _ := chromedp.NewContext(a.allocCtx)

	return &cdpPage{
		ctx: a.allocCtx,
	}, nil
}

func (a *Automation) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.launched {
		return nil
	}

	if a.cancelFn != nil {
		a.cancelFn()
	}
	a.launched = false
	return nil
}

// cdpPage CDP 页面实现
type cdpPage struct {
	ctx context.Context
	url string
}

func (p *cdpPage) Navigate(ctx context.Context, url string) error {
	// TODO: chromedp.Run(p.ctx, chromedp.Navigate(url))
	p.url = url
	return nil
}

func (p *cdpPage) WaitForSelector(ctx context.Context, selector string) error {
	// TODO: chromedp.Run(p.ctx, chromedp.WaitVisible(selector))
	return nil
}

func (p *cdpPage) Click(ctx context.Context, selector string) error {
	// TODO: chromedp.Run(p.ctx, chromedp.Click(selector))
	return nil
}

func (p *cdpPage) Type(ctx context.Context, selector string, text string) error {
	// TODO: chromedp.Run(p.ctx, chromedp.SendKeys(selector, text))
	return nil
}

func (p *cdpPage) GetURL() string {
	return p.url
}

func (p *cdpPage) GetContent(ctx context.Context) (string, error) {
	// TODO: chromedp.Run(p.ctx, chromedp.OuterHTML("html", &content))
	return "", nil
}

func (p *cdpPage) WaitForNavigation(ctx context.Context) error {
	// TODO: chromedp.Run(p.ctx, chromedp.WaitReady("body"))
	return nil
}

func (p *cdpPage) EvalJS(ctx context.Context, expression string) (any, error) {
	// TODO: chromedp.Run(p.ctx, chromedp.Evaluate(expression, &result))
	return nil, nil
}

func (p *cdpPage) Screenshot(ctx context.Context, path string) error {
	// TODO: chromedp.Run(p.ctx, chromedp.CaptureScreenshot(&buf))
	return nil
}

func (p *cdpPage) Close() error {
	// TODO: 关闭标签页
	return nil
}
