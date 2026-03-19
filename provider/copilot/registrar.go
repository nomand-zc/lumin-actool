package copilot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/nomand-zc/lumin-actool/callback"
	"github.com/nomand-zc/lumin-actool/email"
	"github.com/nomand-zc/lumin-actool/provider"
)

func init() {
	provider.Register("copilot", New())
}

const (
	providerName = "copilot"
	authMethod   = "device-code"

	// GitHub Device Code Flow 端点（参考 CLIProxyAPIPlus copilot/oauth.go）
	githubDeviceCodeURL  = "https://github.com/login/device/code"
	githubAccessTokenURL = "https://github.com/login/oauth/access_token"
	githubClientID       = "Iv1.b507a08c87ecfe98" // GitHub Copilot CLI client ID

	deviceCodePollInterval = 5 * time.Second
)

// Registrar GitHub Copilot 供应商注册器实现。
// 通过 GitHub Device Code Flow 自动注册并获取凭证。
type Registrar struct{}

// New 创建 Copilot 注册器
func New() *Registrar {
	return &Registrar{}
}

func (r *Registrar) Provider() string {
	return providerName
}

func (r *Registrar) AuthMethod() string {
	return authMethod
}

func (r *Registrar) Register(ctx context.Context, emailAccount *email.EmailAccount, callbackSrv callback.CallbackServer, opts ...provider.RegisterOption) (*provider.RegistrationResult, error) {
	o := provider.ApplyRegisterOptions(opts...)

	ctx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()

	// 1. 请求 device code
	deviceCode, err := requestDeviceCode(ctx)
	if err != nil {
		return nil, fmt.Errorf("request device code: %w", err)
	}

	fmt.Printf("  [copilot] 请在浏览器中访问: %s\n", deviceCode.VerificationURI)
	fmt.Printf("  [copilot] 输入代码: %s\n", deviceCode.UserCode)

	// 2. TODO: 使用浏览器自动化自动完成 GitHub 授权
	//   a. 打开 deviceCode.VerificationURI
	//   b. 使用邮箱登录 GitHub
	//   c. 输入 deviceCode.UserCode
	//   d. 点击授权
	_ = o.Headless

	// 3. 轮询等待用户完成授权
	tokenResp, err := pollForToken(ctx, deviceCode.DeviceCode, deviceCode.Interval)
	if err != nil {
		return nil, fmt.Errorf("poll for token: %w", err)
	}

	// 4. 构造凭证
	now := time.Now()
	credential := map[string]any{
		"github_token": tokenResp.AccessToken,
		"token_type":   tokenResp.TokenType,
		"scope":        tokenResp.Scope,
	}

	return &provider.RegistrationResult{
		Provider:   providerName,
		Email:      emailAccount.Email,
		Credential: credential,
		UserInfo: &provider.UserInfo{
			Email: emailAccount.Email,
		},
		RegisteredAt: now,
	}, nil
}

func (r *Registrar) Refresh(ctx context.Context, credential map[string]any) (*provider.RegistrationResult, error) {
	return nil, fmt.Errorf("copilot refresh: not implemented yet")
}

func (r *Registrar) Validate(ctx context.Context, credential map[string]any) (bool, error) {
	token, ok := credential["github_token"].(string)
	if !ok || token == "" {
		return false, nil
	}
	// TODO: 调用 GitHub API 验证 token
	return true, nil
}

// --- Device Code Flow 辅助函数 ---

// deviceCodeResponse device code 请求响应
type deviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// accessTokenResponse access token 轮询响应
type accessTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	Error       string `json:"error"`
}

// requestDeviceCode 请求 device code
func requestDeviceCode(ctx context.Context) (*deviceCodeResponse, error) {
	data := fmt.Sprintf("client_id=%s&scope=user:email", githubClientID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, githubDeviceCodeURL, strings.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("device code request failed: status=%d, body=%s", resp.StatusCode, string(body))
	}

	var dcResp deviceCodeResponse
	if err := json.Unmarshal(body, &dcResp); err != nil {
		return nil, fmt.Errorf("parse device code response: %w", err)
	}

	return &dcResp, nil
}

// pollForToken 轮询等待用户完成授权
func pollForToken(ctx context.Context, deviceCode string, interval int) (*accessTokenResponse, error) {
	pollInterval := time.Duration(interval) * time.Second
	if pollInterval < deviceCodePollInterval {
		pollInterval = deviceCodePollInterval
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			data := fmt.Sprintf("client_id=%s&device_code=%s&grant_type=urn:ietf:params:oauth:grant-type:device_code",
				githubClientID, deviceCode)

			req, err := http.NewRequestWithContext(ctx, http.MethodPost, githubAccessTokenURL, strings.NewReader(data))
			if err != nil {
				return nil, err
			}
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("Accept", "application/json")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				continue // 网络错误，继续重试
			}

			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				continue
			}

			var tokenResp accessTokenResponse
			if err := json.Unmarshal(body, &tokenResp); err != nil {
				continue
			}

			switch tokenResp.Error {
			case "":
				// 成功获取 token
				if tokenResp.AccessToken != "" {
					return &tokenResp, nil
				}
			case "authorization_pending":
				// 用户尚未完成授权，继续轮询
				continue
			case "slow_down":
				// 轮询过快，增加间隔
				pollInterval += 5 * time.Second
				ticker.Reset(pollInterval)
				continue
			case "expired_token":
				return nil, fmt.Errorf("device code expired, please try again")
			case "access_denied":
				return nil, fmt.Errorf("user denied authorization")
			default:
				return nil, fmt.Errorf("unexpected error: %s", tokenResp.Error)
			}
		}
	}
}
