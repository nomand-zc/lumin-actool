// Package kiro 实现了 Kiro 供应商的 OAuth 授权注册器。
//
// Kiro 使用 AWS Cognito 作为身份认证服务，支持以下 Social Login 提供商：
//   - Google
//   - GitHub
//
// 认证流程（Social OAuth + PKCE）：
//  1. 生成 PKCE code_verifier 和 code_challenge
//  2. 启动本地 HTTP 回调服务器
//  3. 构造授权 URL 并打开浏览器
//  4. 用户在浏览器中完成 Google/GitHub 授权
//  5. 回调服务器接收授权码 code
//  6. 使用 code + code_verifier 交换 token
//  7. 从 JWT access token 中提取用户 email
//  8. 构造符合 lumin-client kiro.Credential 格式的凭证
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

	// Kiro AuthService 端点（来自 CLIProxyAPIPlus）
	kiroAuthServiceEndpoint = "https://prod.us-east-1.auth.desktop.kiro.dev"

	// OAuth 超时
	socialAuthTimeout = 10 * time.Minute
)

// SocialProvider 社交登录提供商类型
type SocialProvider string

const (
	ProviderGoogle SocialProvider = "Google"
	ProviderGitHub SocialProvider = "Github"
)

// --- 请求/响应结构体（与 Kiro AuthService API 对齐） ---

// CreateTokenRequest 发送到 Kiro /oauth/token 端点的请求体（JSON 格式）
type CreateTokenRequest struct {
	Code         string `json:"code"`
	CodeVerifier string `json:"code_verifier"`
	RedirectURI  string `json:"redirect_uri"`
}

// SocialTokenResponse 来自 Kiro /oauth/token 和 /refreshToken 端点的响应
type SocialTokenResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ProfileArn   string `json:"profileArn"`
	ExpiresIn    int    `json:"expiresIn"`
}

// RefreshTokenRequest 发送到 Kiro /refreshToken 端点的请求体
type RefreshTokenRequest struct {
	RefreshToken string `json:"refreshToken"`
}

// JWTClaims JWT payload 中我们关心的字段
type JWTClaims struct {
	Email         string `json:"email,omitempty"`
	Sub           string `json:"sub,omitempty"`
	PreferredUser string `json:"preferred_username,omitempty"`
	Name          string `json:"name,omitempty"`
}

// --- Registrar 实现 ---

// Registrar Kiro 供应商注册器实现。
// 通过 Google/GitHub Social OAuth + PKCE 流程获取凭证。
type Registrar struct {
	// socialProvider 社交登录提供商，默认 Google
	socialProvider SocialProvider
}

// New 创建 Kiro 注册器（默认使用 Google 登录）
func New() *Registrar {
	return &Registrar{
		socialProvider: ProviderGoogle,
	}
}

// NewWithProvider 创建指定社交提供商的 Kiro 注册器
func NewWithProvider(sp SocialProvider) *Registrar {
	return &Registrar{
		socialProvider: sp,
	}
}

func (r *Registrar) Provider() string {
	return providerName
}

func (r *Registrar) AuthMethod() string {
	return authMethod
}

// Register 完整实现 Kiro Social OAuth 注册流程
func (r *Registrar) Register(ctx context.Context, emailAccount *email.EmailAccount, callbackSrv callback.CallbackServer, opts ...provider.RegisterOption) (*provider.RegistrationResult, error) {
	o := provider.ApplyRegisterOptions(opts...)

	// 设置超时上下文
	timeout := o.Timeout
	if timeout <= 0 {
		timeout = socialAuthTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 解析社交提供商：默认 Google，可通过 Metadata 指定
	socialProv := r.socialProvider
	if o.Metadata != nil {
		if sp, ok := o.Metadata["social_provider"]; ok {
			switch strings.ToLower(sp) {
			case "github":
				socialProv = ProviderGitHub
			case "google":
				socialProv = ProviderGoogle
			}
		}
	}

	// Step 1: 生成 PKCE code_verifier 和 code_challenge
	codeVerifier, codeChallenge, err := generatePKCE()
	if err != nil {
		return nil, fmt.Errorf("kiro: generate PKCE: %w", err)
	}

	// Step 2: 生成随机 state 参数（防 CSRF）
	state, err := generateState()
	if err != nil {
		return nil, fmt.Errorf("kiro: generate state: %w", err)
	}

	// Step 3: 获取回调 URL 并注册等待会话
	redirectURI := callbackSrv.CallbackURL(providerName)
	resultChan := callbackSrv.RegisterSession(state, providerName, timeout)

	// Step 4: 构造 Kiro 授权 URL
	// 格式: {endpoint}/login?idp={Google|Github}&redirect_uri=...&code_challenge=...&code_challenge_method=S256&state=...&prompt=select_account
	authURL := buildLoginURL(string(socialProv), redirectURI, codeChallenge, state)

	fmt.Printf("  [kiro] Social Provider: %s\n", socialProv)
	fmt.Printf("  [kiro] 授权 URL: %s\n", authURL)

	// Step 5: 打开浏览器或等待用户手动访问
	// TODO: 集成 BrowserAutomation 自动完成以下步骤：
	//   a. 使用 incognito 模式打开 authURL
	//   b. 等待 Google/GitHub 登录页面加载
	//   c. 自动填写邮箱和密码
	//   d. 点击授权按钮
	//   e. 等待重定向到回调 URL
	fmt.Printf("  [kiro] 请在浏览器中打开上述 URL 完成授权...\n")

	// Step 6: 等待 OAuth 回调
	var oauthResult *callback.OAuthResult
	select {
	case oauthResult = <-resultChan:
		if oauthResult == nil {
			return nil, fmt.Errorf("kiro: callback session expired or closed")
		}
	case <-ctx.Done():
		return nil, fmt.Errorf("kiro: waiting for OAuth callback: %w", ctx.Err())
	}

	if !oauthResult.IsSuccess() {
		return nil, fmt.Errorf("kiro: OAuth error: %s - %s", oauthResult.Error, oauthResult.ErrorDescription)
	}

	fmt.Printf("  [kiro] ✓ 收到授权码，正在交换 Token...\n")

	// Step 7: 使用授权码交换 token
	// 关键：Kiro 的 /oauth/token 端点接收 JSON 请求体（不是 form-urlencoded）
	tokenResp, err := createToken(ctx, &CreateTokenRequest{
		Code:         oauthResult.Code,
		CodeVerifier: codeVerifier,
		RedirectURI:  redirectURI,
	}, o.ProxyURL)
	if err != nil {
		return nil, fmt.Errorf("kiro: exchange token: %w", err)
	}

	fmt.Printf("  [kiro] ✓ Token 交换成功\n")

	// Step 8: 从 JWT access token 中提取 email
	jwtEmail := extractEmailFromJWT(tokenResp.AccessToken)
	if jwtEmail == "" {
		jwtEmail = emailAccount.Email
	}

	// Step 9: 构造凭证数据（严格兼容 lumin-client kiro.Credential 格式）
	// 字段命名与 lumin-client/credentials/kiro/credentials.go 完全对齐
	now := time.Now()
	expiresIn := tokenResp.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 3600 // 默认 1 小时
	}
	expiresAt := now.Add(time.Duration(expiresIn) * time.Second)

	credential := map[string]any{
		"accessToken":  tokenResp.AccessToken,
		"refreshToken": tokenResp.RefreshToken,
		"profileArn":   tokenResp.ProfileArn,
		"expiresAt":    expiresAt.Format(time.RFC3339),
		"authMethod":   "social",
		"provider":     string(socialProv),
		"region":       "us-east-1",
	}

	return &provider.RegistrationResult{
		Provider:   providerName,
		Email:      jwtEmail,
		Credential: credential,
		UserInfo: &provider.UserInfo{
			Email: jwtEmail,
		},
		ExpiresAt:    &expiresAt,
		RegisteredAt: now,
		Metadata: map[string]any{
			"social_provider": string(socialProv),
			"profile_arn":     tokenResp.ProfileArn,
		},
	}, nil
}

// Refresh 使用 refreshToken 刷新过期的凭证
// 参考 CLIProxyAPIPlus social_auth.go RefreshSocialToken 实现
func (r *Registrar) Refresh(ctx context.Context, credential map[string]any) (*provider.RegistrationResult, error) {
	refreshToken, ok := credential["refreshToken"].(string)
	if !ok || refreshToken == "" {
		return nil, fmt.Errorf("kiro refresh: missing refreshToken")
	}

	region, _ := credential["region"].(string)
	if region == "" {
		region = "us-east-1"
	}
	socialProv, _ := credential["provider"].(string)

	// 调用 Kiro /refreshToken 端点
	tokenResp, err := refreshSocialToken(ctx, refreshToken)
	if err != nil {
		return nil, fmt.Errorf("kiro refresh: %w", err)
	}

	// 构造刷新后的凭证
	expiresIn := tokenResp.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 3600
	}
	now := time.Now()
	expiresAt := now.Add(time.Duration(expiresIn) * time.Second)

	email := extractEmailFromJWT(tokenResp.AccessToken)
	if email == "" {
		email, _ = credential["email"].(string)
	}

	newCredential := map[string]any{
		"accessToken":  tokenResp.AccessToken,
		"refreshToken": tokenResp.RefreshToken,
		"profileArn":   tokenResp.ProfileArn,
		"expiresAt":    expiresAt.Format(time.RFC3339),
		"authMethod":   "social",
		"provider":     socialProv,
		"region":       region,
	}

	return &provider.RegistrationResult{
		Provider:   providerName,
		Email:      email,
		Credential: newCredential,
		UserInfo: &provider.UserInfo{
			Email: email,
		},
		ExpiresAt:    &expiresAt,
		RegisteredAt: now,
	}, nil
}

// Validate 通过调用 Kiro getUsageLimits API 验证凭证是否有效
// 参考 CLIProxyAPIPlus aws_auth.go ValidateToken 实现
func (r *Registrar) Validate(ctx context.Context, credential map[string]any) (bool, error) {
	accessToken, ok := credential["accessToken"].(string)
	if !ok || accessToken == "" {
		return false, nil
	}

	profileArn, _ := credential["profileArn"].(string)
	if profileArn == "" {
		return false, fmt.Errorf("kiro validate: missing profileArn")
	}

	// 调用 getUsageLimits 来验证 token
	err := validateToken(ctx, accessToken, profileArn)
	return err == nil, err
}

// --- Kiro AuthService API 调用 ---

// buildLoginURL 构造 Kiro OAuth 登录 URL
// 格式与 CLIProxyAPIPlus social_auth.go buildLoginURL 完全一致
func buildLoginURL(providerName, redirectURI, codeChallenge, state string) string {
	return fmt.Sprintf(
		"%s/login?idp=%s&redirect_uri=%s&code_challenge=%s&code_challenge_method=S256&state=%s&prompt=select_account",
		kiroAuthServiceEndpoint,
		providerName,
		url.QueryEscape(redirectURI),
		codeChallenge,
		state,
	)
}

// createToken 使用授权码交换 token
// 关键：请求体是 JSON 格式（不是 form-urlencoded），与标准 OAuth2 不同
// 参考 CLIProxyAPIPlus social_auth.go CreateToken
func createToken(ctx context.Context, req *CreateTokenRequest, proxyURL string) (*SocialTokenResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal token request: %w", err)
	}

	tokenURL := kiroAuthServiceEndpoint + "/oauth/token"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("create token request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", buildUserAgent())
	httpReq.Header.Set("Accept", "application/json, text/plain, */*")

	client := buildHTTPClient(proxyURL)
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var tokenResp SocialTokenResponse
	if err := json.Unmarshal(respBody, &tokenResp); err != nil {
		return nil, fmt.Errorf("parse token response: %w", err)
	}

	return &tokenResp, nil
}

// refreshSocialToken 使用 refreshToken 刷新 token
// 参考 CLIProxyAPIPlus social_auth.go RefreshSocialToken
func refreshSocialToken(ctx context.Context, refreshToken string) (*SocialTokenResponse, error) {
	body, err := json.Marshal(&RefreshTokenRequest{RefreshToken: refreshToken})
	if err != nil {
		return nil, fmt.Errorf("marshal refresh request: %w", err)
	}

	refreshURL := kiroAuthServiceEndpoint + "/refreshToken"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, refreshURL, strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("create refresh request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", buildUserAgent())
	httpReq.Header.Set("Accept", "application/json, text/plain, */*")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("refresh request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read refresh response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token refresh failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var tokenResp SocialTokenResponse
	if err := json.Unmarshal(respBody, &tokenResp); err != nil {
		return nil, fmt.Errorf("parse refresh response: %w", err)
	}

	return &tokenResp, nil
}

// validateToken 通过调用 getUsageLimits API 验证 token 有效性
// 参考 CLIProxyAPIPlus aws_auth.go ValidateToken → GetUsageLimits
func validateToken(ctx context.Context, accessToken, profileArn string) error {
	// 根据 profileArn 确定 API endpoint
	endpoint := getKiroAPIEndpoint(profileArn)
	apiURL := fmt.Sprintf("%s/getUsageLimits?origin=AI_EDITOR&profileArn=%s&resourceType=AGENTIC_REQUEST",
		endpoint, url.QueryEscape(profileArn))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return fmt.Errorf("create validate request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("User-Agent", buildUserAgent())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("validate request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("validate failed (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// --- 辅助函数 ---

// buildUserAgent 构造 KiroIDE 风格的 User-Agent
// 参考 CLIProxyAPIPlus fingerprint.go 的格式
func buildUserAgent() string {
	return "KiroIDE-0.10.32-actool"
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

// getKiroAPIEndpoint 根据 profileArn 提取区域并返回 API endpoint
// 参考 CLIProxyAPIPlus aws.go GetKiroAPIEndpointFromProfileArn
func getKiroAPIEndpoint(profileArn string) string {
	// profileArn 格式: arn:aws:codewhisperer:{region}:{account}:profile/{id}
	parts := strings.Split(profileArn, ":")
	if len(parts) >= 4 {
		region := parts[3]
		if region != "" {
			return fmt.Sprintf("https://codewhisperer.%s.amazonaws.com", region)
		}
	}
	// 默认 us-east-1
	return "https://codewhisperer.us-east-1.amazonaws.com"
}

// --- PKCE 辅助函数 ---

// generatePKCE 生成 PKCE code_verifier 和 code_challenge
// 与 CLIProxyAPIPlus social_auth.go generatePKCE 完全一致
func generatePKCE() (verifier, challenge string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	verifier = base64.RawURLEncoding.EncodeToString(b)

	h := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(h[:])

	return verifier, challenge, nil
}

// generateState 生成随机 state 参数
func generateState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// --- JWT Email 提取 ---

// extractEmailFromJWT 从 JWT access token 中提取用户 email
// 参考 CLIProxyAPIPlus aws.go ExtractEmailFromJWT
func extractEmailFromJWT(accessToken string) string {
	if accessToken == "" {
		return ""
	}

	// JWT 格式: header.payload.signature
	parts := strings.Split(accessToken, ".")
	if len(parts) != 3 {
		return ""
	}

	// 解码 payload（第二部分）
	payload := parts[1]

	// base64url 可能需要填充
	switch len(payload) % 4 {
	case 2:
		payload += "=="
	case 3:
		payload += "="
	}

	decoded, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		// 尝试不带填充的解码
		decoded, err = base64.RawURLEncoding.DecodeString(parts[1])
		if err != nil {
			return ""
		}
	}

	var claims JWTClaims
	if err := json.Unmarshal(decoded, &claims); err != nil {
		return ""
	}

	// 优先返回 email 字段
	if claims.Email != "" {
		return claims.Email
	}

	// 回退到 preferred_username（某些 provider 使用此字段）
	if claims.PreferredUser != "" && strings.Contains(claims.PreferredUser, "@") {
		return claims.PreferredUser
	}

	// 回退到 sub（如果看起来像 email）
	if claims.Sub != "" && strings.Contains(claims.Sub, "@") {
		return claims.Sub
	}

	return ""
}
