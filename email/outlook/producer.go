package outlook

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/nomand-zc/lumin-actool/email"
)

func init() {
	email.Register("outlook", New())
}

const (
	providerName = "outlook"
	charset      = "abcdefghijklmnopqrstuvwxyz0123456789"
	passwordLen  = 20
	usernameLen  = 12
)

// Producer Outlook 邮箱生产者实现。
// 通过浏览器自动化注册 Outlook/Hotmail 邮箱账号。
type Producer struct{}

// New 创建 Outlook 邮箱生产者
func New() *Producer {
	return &Producer{}
}

func (p *Producer) Provider() string {
	return providerName
}

func (p *Producer) Produce(ctx context.Context, count int, opts ...email.ProduceOption) ([]*email.EmailAccount, error) {
	o := email.ApplyProduceOptions(opts...)

	accounts := make([]*email.EmailAccount, 0, count)

	for i := 0; i < count; i++ {
		select {
		case <-ctx.Done():
			return accounts, ctx.Err()
		default:
		}

		// TODO: 实现 Outlook 浏览器自动化注册流程
		// 1. 打开 https://signup.live.com
		// 2. 自动填写注册表单
		// 3. 处理验证码（可能需要第三方验证码服务）
		// 4. 完成注册

		username, err := randomString(usernameLen)
		if err != nil {
			return accounts, fmt.Errorf("generate username: %w", err)
		}

		if o.NamePrefix != "" {
			username = o.NamePrefix + username
		}

		domain := o.Domain
		if domain == "" {
			domain = "outlook.com"
		}

		password, err := randomPassword(passwordLen)
		if err != nil {
			return accounts, fmt.Errorf("generate password: %w", err)
		}

		accounts = append(accounts, &email.EmailAccount{
			Email:     fmt.Sprintf("%s@%s", username, domain),
			Password:  password,
			Provider:  providerName,
			CreatedAt: time.Now(),
			Metadata: map[string]any{
				"note": "outlook registration not yet automated, placeholder only",
			},
		})
	}

	return accounts, nil
}

func (p *Producer) Verify(ctx context.Context, account *email.EmailAccount) (bool, error) {
	// TODO: 使用 IMAP/SMTP 验证 Outlook 邮箱是否可用
	return true, nil
}

// randomString 生成指定长度的随机字符串
func randomString(length int) (string, error) {
	result := make([]byte, length)
	for i := range result {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		result[i] = charset[idx.Int64()]
	}
	return string(result), nil
}

// randomPassword 生成包含大小写字母、数字和特殊字符的安全密码
func randomPassword(length int) (string, error) {
	const pwCharset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%"
	result := make([]byte, length)
	for i := range result {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(pwCharset))))
		if err != nil {
			return "", err
		}
		result[i] = pwCharset[idx.Int64()]
	}
	return string(result), nil
}
