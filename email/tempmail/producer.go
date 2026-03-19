package tempmail

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/nomand-zc/lumin-actool/email"
)

func init() {
	email.Register("tempmail", New())
}

const (
	providerName = "tempmail"
	charset      = "abcdefghijklmnopqrstuvwxyz0123456789"
	passwordLen  = 16
	usernameLen  = 10
)

// 默认支持的临时邮箱域名
var defaultDomains = []string{
	"tempmail.lol",
	"tmpmail.net",
	"tmpmail.org",
}

// Producer 临时邮箱生产者实现。
// 生成随机的临时邮箱账号（仅用于接收验证码等短期用途）。
type Producer struct{}

// New 创建临时邮箱生产者
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

		username, err := randomString(usernameLen)
		if err != nil {
			return accounts, fmt.Errorf("generate username: %w", err)
		}

		if o.NamePrefix != "" {
			username = o.NamePrefix + username
		}

		domain := o.Domain
		if domain == "" {
			// 随机选择一个域名
			idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(defaultDomains))))
			if err != nil {
				return accounts, fmt.Errorf("select domain: %w", err)
			}
			domain = defaultDomains[idx.Int64()]
		}

		password, err := randomString(passwordLen)
		if err != nil {
			return accounts, fmt.Errorf("generate password: %w", err)
		}

		accounts = append(accounts, &email.EmailAccount{
			Email:     fmt.Sprintf("%s@%s", username, domain),
			Password:  password,
			Provider:  providerName,
			CreatedAt: time.Now(),
		})
	}

	return accounts, nil
}

func (p *Producer) Verify(ctx context.Context, account *email.EmailAccount) (bool, error) {
	// TODO: 调用临时邮箱 API 验证邮箱是否仍然存在
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
