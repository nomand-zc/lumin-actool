// Package gemini 实现了 GeminiCLI 供应商的 OAuth 授权注册器。
//
// GeminiCLI 使用 Google OAuth2 作为身份认证，通过以下流程获取凭证：
//  1. 构造 Google OAuth2 授权 URL（含 ClientID、Scopes、redirect_uri）
//  2. 引导用户在浏览器中完成 Google 账号授权
//  3. 回调服务器接收 authorization code
//  4. 使用 code + client_secret 交换 token（标准 OAuth2 流程）
//  5. 调用 Google userinfo API 获取用户邮箱
//  6. 构造符合 lumin-client geminicli.Credential 格式的凭证
//
// 凭证格式（兼容 lumin-client credentials/geminicli/credentials.go）：
//
//	{
//	    "access_token": "ya29.xxx",
//	    "refresh_token": "1//xxx",
//	    "token": "ya29.xxx",
//	    "client_id": "681255809395-xxx.apps.googleusercontent.com",
//	    "client_secret": "GOCSPX-xxx",
//	    "project_id": "",
//	    "email": "user@gmail.com",
//	    "scopes": ["..."],
//	    "token_uri": "https://oauth2.googleapis.com/token",
//	    "expiry": "2026-03-19T17:33:30Z"
//	}
package gemini

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nomand-zc/lumin-actool/callback"
	"github.com/nomand-zc/lumin-actool/email"
	"github.com/nomand-zc/lumin-actool/provider"
)

func init() {
	provider.Register("gemini", New())
}

const (
	providerName = "gemini"
	authMethod   = "google-oauth"

	// Google OAuth2 常量（与 CLIProxyAPIPlus 和 lumin-client 完全一致）
	googleAuthorizeURL = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL     = "https://oauth2.googleapis.com/token"
	googleUserInfoURL  = "https://www.googleapis.com/oauth2/v1/userinfo?alt=json"

	// GeminiCLI 的 OAuth ClientID 和 ClientSecret
	// 来源：CLIProxyAPIPlus/internal/auth/gemini/gemini_auth.go
	geminiClientID     = "681255809395-oo8ft2oprdrnp9e3aqf6av3hmdib135j.apps.googleusercontent.com"
	geminiClientSecret = "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"

	sessionTTL = 5 * time.Minute
)

// OAuth Scopes（与 CLIProxyAPIPlus 和 lumin-client 一致）
var geminiScopes = []string{
	"https://www.googleapis.com/auth/cloud-platform",
	"https://www.googleapis.com/auth/userinfo.email",
	"https://www.googleapis.com/auth/userinfo.profile",
}

// --- 请求/响应结构体 ---

// tokenResponse Google OAuth2 token 交换响应
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
	IDToken      string `json:"id_token,omitempty"`
}

// tokenRefreshResponse Google OAuth2 token 刷新响应
type tokenRefreshResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	Scope       string `json:"scope"`
	TokenType   string `json:"token_type"`
	IDToken     string `json:"id_token,omitempty"`

	Error            string `json:"error,omitempty"`
	ErrorDescription string `json:"error_description,omitempty"`
}

// userInfoResponse Google userinfo API 响应
type userInfoResponse struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

// --- Registrar 实现 ---

// Registrar GeminiCLI 供应商注册器实现。
// 通过 Google OAuth2 流程自动注册并获取凭证。
type Registrar struct{}

// New 创建 Gemini 注册器
func New() *Registrar {
	return &Registrar{}
}

func (r *Registrar) Provider() string {
	return providerName
}

func (r *Registrar) AuthMethod() string {
	return authMethod
}

// Register 完整实现 GeminiCLI Google OAuth2 注册流程。
// 流程参考 CLIProxyAPIPlus/internal/auth/gemini/gemini_auth.go getTokenFromWeb
func (r *Registrar) Register(ctx context.Context, emailAccount *email.EmailAccount, callbackSrv callback.CallbackServer, opts ...provider.RegisterOption) (*provider.RegistrationResult, error) {
	o := provider.ApplyRegisterOptions(opts...)

	ctx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()

	// Step 1: 生成随机 state 参数（防 CSRF）
	state, err := generateState()
	if err != nil {
		return nil, fmt.Errorf("gemini: generate state: %w", err)
	}

	// Step 2: 获取回调 URL 并注册等待会话
	redirectURI := callbackSrv.CallbackURL(providerName)
	resultChan := callbackSrv.RegisterSession(state, providerName, sessionTTL)

	// Step 3: 构造 Google OAuth2 授权 URL
	// 与 CLIProxyAPIPlus gemini_auth.go 中使用 oauth2.Config.AuthCodeURL 等效：
	// AccessTypeOffline 获取 refresh_token，prompt=consent 强制展示授权页面
	authURL := buildAuthURL(redirectURI, state)

	fmt.Printf("  [gemini] 授权 URL: %s\n", authURL)

	// Step 4: 打开浏览器或等待用户手动访问
	// TODO: 集成 BrowserAutomation 自动完成 Google 登录
	fmt.Printf("  [gemini] 请在浏览器中打开上述 URL 完成 Google 授权...\n")
	fmt.Printf("  [gemini] 等待 OAuth 回调...\n")

	// Step 5: 等待 OAuth 回调
	var oauthResult *callback.OAuthResult
	select {
	case oauthResult = <-resultChan:
		if oauthResult == nil {
			return nil, fmt.Errorf("gemini: callback session expired or closed")
		}
	case <-ctx.Done():
		return nil, fmt.Errorf("gemini: waiting for OAuth callback: %w", ctx.Err())
	}

	if !oauthResult.IsSuccess() {
		return nil, fmt.Errorf("gemini: OAuth error: %s - %s", oauthResult.Error, oauthResult.ErrorDescription)
	}

	fmt.Printf("  [gemini] ✓ 收到授权码，正在交换 Token...\n")

	// Step 6: 使用 authorization code 交换 token
	// Google OAuth2 标准流程：使用 client_secret（非 PKCE）
	tokenResp, err := exchangeToken(ctx, oauthResult.Code, redirectURI, o.ProxyURL)
	if err != nil {
		return nil, fmt.Errorf("gemini: exchange token: %w", err)
	}

	fmt.Printf("  [gemini] ✓ Token 交换成功\n")

	// Step 7: 调用 Google userinfo API 获取用户邮箱
	// 参考 CLIProxyAPIPlus gemini_auth.go createTokenStorage
	userInfo, err := getUserInfo(ctx, tokenResp.AccessToken, o.ProxyURL)
	if err != nil {
		fmt.Printf("  [gemini] ⚠ 获取用户信息失败: %v，使用邮箱账号的 email\n", err)
	}

	// 确定 email
	userEmail := emailAccount.Email
	if userInfo != nil && userInfo.Email != "" {
		userEmail = userInfo.Email
		fmt.Printf("  [gemini] ✓ 认证用户邮箱: %s\n", userEmail)
	}

	// Step 8: 构造凭证数据
	// 严格兼容 lumin-client credentials/geminicli/credentials.go 格式
	// 对齐 CLIProxyAPIPlus 中 GeminiTokenStorage 的字段结构
	now := time.Now()
	expiresIn := tokenResp.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 3600
	}
	expiresAt := now.Add(time.Duration(expiresIn) * time.Second)

	credential := map[string]any{
		// 核心 token 字段
		"access_token":  tokenResp.AccessToken,
		"refresh_token": tokenResp.RefreshToken,
		"token":         tokenResp.AccessToken, // 与 access_token 冗余，保持兼容性
		"token_type":    tokenResp.TokenType,

		// OAuth2 客户端信息（refresh 时需要）
		"client_id":     geminiClientID,
		"client_secret": geminiClientSecret,

		// Google API 元数据
		"token_uri":       googleTokenURL,
		"scopes":          geminiScopes,
		"universe_domain": "googleapis.com",

		// 用户信息
		"email": userEmail,

		// project_id: 空字符串，等待后续通过 GeminiCLI onboarding 流程设置
		// 或由用户在 lumin-acpool 中手动配置
		"project_id": "",

		// 过期时间（字段名 expiry 与 lumin-client 的 json tag 一致）
		"expiry": expiresAt.Format(time.RFC3339),
	}

	fmt.Printf("  [gemini] ✓ 凭证构造完成，email=%s\n", userEmail)

	return &provider.RegistrationResult{
		Provider:   providerName,
		Email:      userEmail,
		Credential: credential,
		UserInfo: &provider.UserInfo{
			Email: userEmail,
			Name:  getUserName(userInfo),
		},
		ExpiresAt:    &expiresAt,
		RegisteredAt: now,
		Metadata: map[string]any{
			"token_type": tokenResp.TokenType,
		},
	}, nil
}

// Refresh 使用 refresh_token 通过 Google OAuth2 端点刷新 access_token。
// 参考 CLIProxyAPIPlus sdk/auth/filestore.go refreshGeminiAccessToken
// 和 lumin-client providers/geminicli/credential_manager.go refreshOAuth2Token
func (r *Registrar) Refresh(ctx context.Context, credential map[string]any) (*provider.RegistrationResult, error) {
	refreshToken, ok := credential["refresh_token"].(string)
	if !ok || refreshToken == "" {
		return nil, fmt.Errorf("gemini refresh: missing refresh_token")
	}

	clientID, _ := credential["client_id"].(string)
	if clientID == "" {
		clientID = geminiClientID
	}

	clientSecret, _ := credential["client_secret"].(string)
	if clientSecret == "" {
		clientSecret = geminiClientSecret
	}

	tokenURI, _ := credential["token_uri"].(string)
	if tokenURI == "" {
		tokenURI = googleTokenURL
	}

	// 构建标准 Google OAuth2 refresh 请求
	formData := url.Values{
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"refresh_token": {refreshToken},
		"grant_type":    {"refresh_token"},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURI, strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, fmt.Errorf("gemini refresh: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gemini refresh: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gemini refresh: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp tokenRefreshResponse
		if jsonErr := json.Unmarshal(body, &errResp); jsonErr == nil && errResp.Error != "" {
			return nil, fmt.Errorf("gemini refresh: %s - %s", errResp.Error, errResp.ErrorDescription)
		}
		return nil, fmt.Errorf("gemini refresh: status=%d, body=%s", resp.StatusCode, string(body))
	}

	var result tokenRefreshResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("gemini refresh: parse response: %w", err)
	}

	if result.Error != "" {
		return nil, fmt.Errorf("gemini refresh: %s - %s", result.Error, result.ErrorDescription)
	}

	// 构造刷新后的凭证（保留原有字段，更新 token 相关字段）
	now := time.Now()
	expiresIn := result.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 3600
	}
	expiresAt := now.Add(time.Duration(expiresIn) * time.Second)

	// 复制原凭证并更新
	newCredential := make(map[string]any)
	for k, v := range credential {
		newCredential[k] = v
	}
	newCredential["access_token"] = result.AccessToken
	newCredential["token"] = result.AccessToken
	newCredential["expiry"] = expiresAt.Format(time.RFC3339)

	emailStr, _ := credential["email"].(string)

	return &provider.RegistrationResult{
		Provider:   providerName,
		Email:      emailStr,
		Credential: newCredential,
		UserInfo: &provider.UserInfo{
			Email: emailStr,
		},
		ExpiresAt:    &expiresAt,
		RegisteredAt: now,
	}, nil
}

// Validate 验证凭证是否有效。
// 通过调用 Google userinfo API 来验证 access_token。
func (r *Registrar) Validate(ctx context.Context, credential map[string]any) (bool, error) {
	accessToken, ok := credential["access_token"].(string)
	if !ok || accessToken == "" {
		return false, nil
	}

	// 调用 Google userinfo API 验证 token 有效性
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, googleUserInfoURL, nil)
	if err != nil {
		return false, fmt.Errorf("gemini validate: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("gemini validate: request failed: %w", err)
	}
	defer resp.Body.Close()

	// 200 表示 token 有效
	if resp.StatusCode == http.StatusOK {
		return true, nil
	}

	// 401 表示 token 已过期或无效
	if resp.StatusCode == http.StatusUnauthorized {
		return false, nil
	}

	body, _ := io.ReadAll(resp.Body)
	return false, fmt.Errorf("gemini validate: unexpected status=%d, body=%s", resp.StatusCode, string(body))
}

// --- OAuth2 辅助函数 ---

// buildAuthURL 构造 Google OAuth2 授权 URL。
// 与 CLIProxyAPIPlus gemini_auth.go 中使用 oauth2.Config.AuthCodeURL 等效：
//   - AccessTypeOffline: 获取 refresh_token
//   - prompt=consent: 强制展示授权页面（确保每次获得 refresh_token）
func buildAuthURL(redirectURI, state string) string {
	params := url.Values{
		"response_type": {"code"},
		"client_id":     {geminiClientID},
		"redirect_uri":  {redirectURI},
		"state":         {state},
		"scope":         {strings.Join(geminiScopes, " ")},
		"access_type":   {"offline"},
		"prompt":        {"consent"},
	}
	return fmt.Sprintf("%s?%s", googleAuthorizeURL, params.Encode())
}

// exchangeToken 使用授权码交换 token。
// Google OAuth2 标准流程：使用 client_id + client_secret（非 PKCE 模式）。
// 参考 CLIProxyAPIPlus gemini_auth.go config.Exchange(ctx, authCode)
func exchangeToken(ctx context.Context, code, redirectURI, proxyURL string) (*tokenResponse, error) {
	formData := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {geminiClientID},
		"client_secret": {geminiClientSecret},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, googleTokenURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := buildHTTPClient(proxyURL)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed (status %d): %s", resp.StatusCode, string(body))
	}

	var tokenResp tokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("parse token response: %w", err)
	}

	return &tokenResp, nil
}

// getUserInfo 调用 Google userinfo API 获取用户信息。
// 参考 CLIProxyAPIPlus gemini_auth.go createTokenStorage 中的 userinfo 调用
func getUserInfo(ctx context.Context, accessToken, proxyURL string) (*userInfoResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, googleUserInfoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create userinfo request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	client := buildHTTPClient(proxyURL)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("userinfo request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read userinfo response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("userinfo request failed (status %d): %s", resp.StatusCode, string(body))
	}

	var info userInfoResponse
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("parse userinfo response: %w", err)
	}

	return &info, nil
}

// --- 通用辅助函数 ---

// generateState 生成随机 state 参数（防 CSRF）
func generateState() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// buildHTTPClient 构造 HTTP 客户端（支持代理）
func buildHTTPClient(proxyURL string) *http.Client {
	client := &http.Client{Timeout: 30 * time.Second}
	if proxyURL != "" {
		proxyParsed, err := url.Parse(proxyURL)
		if err == nil {
			client.Transport = &http.Transport{
				Proxy: http.ProxyURL(proxyParsed),
			}
		}
	}
	return client
}

// getUserName 从 userinfo 响应中获取用户名
func getUserName(info *userInfoResponse) string {
	if info == nil {
		return ""
	}
	return info.Name
}
