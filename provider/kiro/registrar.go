package kiro

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
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
	provider.Register("kiro", New())
}

const (
	providerName = "kiro"
	authMethod   = "social-oauth"

	// Kiro OAuth 端点（参考 CLIProxyAPIPlus kiro social_auth.go）
	kiroAuthBaseURL   = "https://kiro.dev"
	kiroAuthorizeURL  = "https://kiro.dev/authorize"
	kiroTokenURL      = "https://kiro.dev/oauth2/token"
	kiroClientID      = "kiro-cli"
	kiroProfileURL    = "https://kiro.dev/api/profile"

	sessionTTL = 5 * time.Minute
)

// Registrar Kiro 供应商注册器实现。
// 通过 Google/GitHub Social OAuth 流程自动注册并获取凭证。
type Registrar struct{}

// New 创建 Kiro 注册器
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

	// 设置超时上下文
	ctx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()

	// 1. 生成 PKCE 参数
	verifier, challenge, err := generatePKCE()
	if err != nil {
		return nil, fmt.Errorf("generate PKCE: %w", err)
	}

	// 2. 生成 state 参数
	state, err := generateState()
	if err != nil {
		return nil, fmt.Errorf("generate state: %w", err)
	}

	// 3. 获取回调 URL 并注册会话
	redirectURI := callbackSrv.CallbackURL(providerName)
	resultChan := callbackSrv.RegisterSession(state, providerName, sessionTTL)

	// 4. 构造授权 URL
	authURL := buildAuthURL(redirectURI, state, challenge)

	// 5. 使用浏览器自动化完成 OAuth 登录
	// TODO: 使用 BrowserAutomation 自动完成以下步骤：
	//   a. 打开 authURL
	//   b. 等待 Google/GitHub 登录页面加载
	//   c. 自动填写邮箱和密码
	//   d. 点击授权按钮
	//   e. 等待重定向到回调 URL
	_ = authURL
	_ = o.Headless

	fmt.Printf("  [kiro] 授权 URL: %s\n", authURL)
	fmt.Printf("  [kiro] 等待 OAuth 回调...\n")

	// 6. 等待回调结果
	var oauthResult *callback.OAuthResult
	select {
	case oauthResult = <-resultChan:
		if oauthResult == nil {
			return nil, fmt.Errorf("callback session expired")
		}
	case <-ctx.Done():
		return nil, fmt.Errorf("waiting for OAuth callback: %w", ctx.Err())
	}

	if !oauthResult.IsSuccess() {
		return nil, fmt.Errorf("OAuth error: %s - %s", oauthResult.Error, oauthResult.ErrorDescription)
	}

	// 7. 使用授权码交换 token
	tokenResp, err := exchangeToken(ctx, oauthResult.Code, redirectURI, verifier)
	if err != nil {
		return nil, fmt.Errorf("exchange token: %w", err)
	}

	// 8. 获取用户信息
	userInfo, err := getUserInfo(ctx, tokenResp.AccessToken)
	if err != nil {
		// 获取用户信息失败不影响注册结果
		userInfo = &provider.UserInfo{Email: emailAccount.Email}
	}

	// 9. 构造凭证数据（兼容 lumin-client kiro.Credential 格式）
	now := time.Now()
	expiresAt := now.Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	credential := map[string]any{
		"accessToken":  tokenResp.AccessToken,
		"refreshToken": tokenResp.RefreshToken,
		"authMethod":   "social",
		"provider":     "Google",
		"expiresAt":    expiresAt.Format(time.RFC3339),
	}
	if tokenResp.ProfileArn != "" {
		credential["profileArn"] = tokenResp.ProfileArn
	}
	if tokenResp.Region != "" {
		credential["region"] = tokenResp.Region
	}

	return &provider.RegistrationResult{
		Provider:     providerName,
		Email:        emailAccount.Email,
		Credential:   credential,
		UserInfo:     userInfo,
		ExpiresAt:    &expiresAt,
		RegisteredAt: now,
	}, nil
}

func (r *Registrar) Refresh(ctx context.Context, credential map[string]any) (*provider.RegistrationResult, error) {
	// TODO: 使用 refresh_token 刷新凭证
	return nil, fmt.Errorf("kiro refresh: not implemented yet")
}

func (r *Registrar) Validate(ctx context.Context, credential map[string]any) (bool, error) {
	accessToken, ok := credential["accessToken"].(string)
	if !ok || accessToken == "" {
		return false, nil
	}

	// 尝试获取用户信息来验证 token 是否有效
	_, err := getUserInfo(ctx, accessToken)
	return err == nil, nil
}

// --- OAuth 辅助函数 ---

// tokenResponse token 交换响应
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	ProfileArn   string `json:"profile_arn,omitempty"`
	Region       string `json:"region,omitempty"`
}

// buildAuthURL 构造 OAuth 授权 URL
func buildAuthURL(redirectURI, state, codeChallenge string) string {
	params := url.Values{
		"response_type":         {"code"},
		"client_id":             {kiroClientID},
		"redirect_uri":          {redirectURI},
		"state":                 {state},
		"code_challenge":        {codeChallenge},
		"code_challenge_method": {"S256"},
		"scope":                 {"openid profile email"},
	}
	return fmt.Sprintf("%s?%s", kiroAuthorizeURL, params.Encode())
}

// exchangeToken 使用授权码交换 token
func exchangeToken(ctx context.Context, code, redirectURI, codeVerifier string) (*tokenResponse, error) {
	data := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {kiroClientID},
		"code_verifier": {codeVerifier},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, kiroTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

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
		return nil, fmt.Errorf("token exchange failed: status=%d, body=%s", resp.StatusCode, string(body))
	}

	var tokenResp tokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("parse token response: %w", err)
	}

	return &tokenResp, nil
}

// getUserInfo 获取用户信息
func getUserInfo(ctx context.Context, accessToken string) (*provider.UserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, kiroProfileURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get user info failed: status=%d", resp.StatusCode)
	}

	var info struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}

	return &provider.UserInfo{
		ID:    info.ID,
		Email: info.Email,
		Name:  info.Name,
	}, nil
}

// --- PKCE 辅助函数 ---

// generatePKCE 生成 PKCE code_verifier 和 code_challenge
func generatePKCE() (verifier, challenge string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	verifier = base64.RawURLEncoding.EncodeToString(buf)

	h := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(h[:])

	return verifier, challenge, nil
}

// generateState 生成随机 state 参数
func generateState() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
