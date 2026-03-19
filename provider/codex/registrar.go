package codex

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
	provider.Register("codex", New())
}

const (
	providerName = "codex"
	authMethod   = "oauth-pkce"

	// Codex (OpenAI) OAuth 端点（参考 CLIProxyAPIPlus codex auth）
	codexAuthorizeURL = "https://auth.openai.com/authorize"
	codexTokenURL     = "https://auth.openai.com/oauth/token"
	codexClientID     = "codex-cli"
	codexAudience     = "https://api.openai.com/v1"

	sessionTTL = 5 * time.Minute
)

// Registrar Codex (OpenAI) 供应商注册器实现。
// 通过 OAuth PKCE 流程自动注册并获取凭证。
type Registrar struct{}

// New 创建 Codex 注册器
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

	// 1. 生成 PKCE 参数
	verifier, challenge, err := generatePKCE()
	if err != nil {
		return nil, fmt.Errorf("generate PKCE: %w", err)
	}

	// 2. 生成 state
	state, err := generateState()
	if err != nil {
		return nil, fmt.Errorf("generate state: %w", err)
	}

	// 3. 注册回调会话
	redirectURI := callbackSrv.CallbackURL(providerName)
	resultChan := callbackSrv.RegisterSession(state, providerName, sessionTTL)

	// 4. 构造授权 URL
	authURL := buildAuthURL(redirectURI, state, challenge)

	// 5. TODO: 使用浏览器自动化完成 OpenAI 登录
	_ = authURL
	_ = o.Headless

	fmt.Printf("  [codex] 授权 URL: %s\n", authURL)
	fmt.Printf("  [codex] 等待 OAuth 回调...\n")

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

	// 7. 交换 token
	tokenResp, err := exchangeToken(ctx, oauthResult.Code, redirectURI, verifier)
	if err != nil {
		return nil, fmt.Errorf("exchange token: %w", err)
	}

	// 8. 构造凭证
	now := time.Now()
	expiresAt := now.Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	credential := map[string]any{
		"access_token":  tokenResp.AccessToken,
		"refresh_token": tokenResp.RefreshToken,
		"expires_at":    expiresAt.Format(time.RFC3339),
	}

	return &provider.RegistrationResult{
		Provider:   providerName,
		Email:      emailAccount.Email,
		Credential: credential,
		UserInfo: &provider.UserInfo{
			Email: emailAccount.Email,
		},
		ExpiresAt:    &expiresAt,
		RegisteredAt: now,
	}, nil
}

func (r *Registrar) Refresh(ctx context.Context, credential map[string]any) (*provider.RegistrationResult, error) {
	return nil, fmt.Errorf("codex refresh: not implemented yet")
}

func (r *Registrar) Validate(ctx context.Context, credential map[string]any) (bool, error) {
	accessToken, ok := credential["access_token"].(string)
	if !ok || accessToken == "" {
		return false, nil
	}
	// TODO: 调用 OpenAI API 验证 token
	return true, nil
}

// --- OAuth 辅助函数 ---

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

func buildAuthURL(redirectURI, state, codeChallenge string) string {
	params := url.Values{
		"response_type":         {"code"},
		"client_id":             {codexClientID},
		"redirect_uri":          {redirectURI},
		"state":                 {state},
		"code_challenge":        {codeChallenge},
		"code_challenge_method": {"S256"},
		"scope":                 {"openid profile email offline_access"},
		"audience":              {codexAudience},
	}
	return fmt.Sprintf("%s?%s", codexAuthorizeURL, params.Encode())
}

func exchangeToken(ctx context.Context, code, redirectURI, codeVerifier string) (*tokenResponse, error) {
	data := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {codexClientID},
		"code_verifier": {codeVerifier},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, codexTokenURL, strings.NewReader(data.Encode()))
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

func generateState() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
