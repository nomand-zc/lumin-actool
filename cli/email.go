package cli

import (
	"fmt"
	"strings"

	"github.com/nomand-zc/lumin-actool/email"
	"github.com/spf13/cobra"
)

// emailCmd email 子命令组
var emailCmd = &cobra.Command{
	Use:   "email",
	Short: "邮箱账号管理",
	Long:  "批量生产和验证邮箱账号",
}

// emailProduceCmd 生产邮箱子命令
var emailProduceCmd = &cobra.Command{
	Use:   "produce",
	Short: "批量生产邮箱账号",
	Example: `  actool email produce --provider tempmail --count 10
  actool email produce --provider outlook --count 5 --prefix "lumin"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		providerName, _ := cmd.Flags().GetString("provider")
		count, _ := cmd.Flags().GetInt("count")
		prefix, _ := cmd.Flags().GetString("prefix")

		producer, err := email.Get(providerName)
		if err != nil {
			return fmt.Errorf("get email producer: %w", err)
		}

		var opts []email.ProduceOption
		if prefix != "" {
			opts = append(opts, email.WithNamePrefix(prefix))
		}

		accounts, err := producer.Produce(cmd.Context(), count, opts...)
		if err != nil {
			return fmt.Errorf("produce emails: %w", err)
		}

		fmt.Printf("成功生产 %d 个邮箱账号:\n", len(accounts))
		for i, acct := range accounts {
			fmt.Printf("  %d. %s (password: %s)\n", i+1, acct.Email, acct.Password)
		}

		return nil
	},
}

// emailVerifyCmd 验证邮箱子命令
var emailVerifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "验证邮箱账号可用性",
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: 从存储中加载邮箱账号并验证
		fmt.Println("email verify: not implemented yet")
		return nil
	},
}

// emailListCmd 列出已注册的邮箱提供商
var emailListCmd = &cobra.Command{
	Use:   "list-providers",
	Short: "列出所有已注册的邮箱提供商",
	Run: func(cmd *cobra.Command, args []string) {
		providers := email.List()
		if len(providers) == 0 {
			fmt.Println("暂无已注册的邮箱提供商")
			return
		}
		fmt.Printf("已注册的邮箱提供商 (%d):\n", len(providers))
		fmt.Printf("  %s\n", strings.Join(providers, ", "))
	},
}

func init() {
	emailProduceCmd.Flags().StringP("provider", "p", "", "邮箱提供商（必填）")
	emailProduceCmd.Flags().IntP("count", "c", 1, "生产数量")
	emailProduceCmd.Flags().String("prefix", "", "邮箱名前缀")
	_ = emailProduceCmd.MarkFlagRequired("provider")

	emailCmd.AddCommand(emailProduceCmd, emailVerifyCmd, emailListCmd)
	rootCmd.AddCommand(emailCmd)
}
