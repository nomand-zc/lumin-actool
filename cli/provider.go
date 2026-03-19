package cli

import (
	"fmt"
	"strings"

	"github.com/nomand-zc/lumin-actool/callback"
	"github.com/nomand-zc/lumin-actool/email"
	"github.com/nomand-zc/lumin-actool/provider"
	"github.com/spf13/cobra"
)

// providerCmd provider 子命令组
var providerCmd = &cobra.Command{
	Use:   "provider",
	Short: "供应商账号管理",
	Long:  "使用邮箱账号注册各 AI 供应商平台",
}

// providerRegisterCmd 注册供应商账号子命令
var providerRegisterCmd = &cobra.Command{
	Use:   "register",
	Short: "使用邮箱注册供应商账号",
	Example: `  actool provider register --vendor kiro --email user@example.com --password "pass123"
  actool provider register --vendor claude --email user@example.com --password "pass123" --headless=false`,
	RunE: func(cmd *cobra.Command, args []string) error {
		vendorName, _ := cmd.Flags().GetString("vendor")
		emailAddr, _ := cmd.Flags().GetString("email")
		password, _ := cmd.Flags().GetString("password")
		headless, _ := cmd.Flags().GetBool("headless")

		registrar, err := provider.Get(vendorName)
		if err != nil {
			return fmt.Errorf("get provider registrar: %w", err)
		}

		// 构造邮箱账号
		emailAccount := &email.EmailAccount{
			Email:    emailAddr,
			Password: password,
		}

		// 启动回调服务器
		cfg := GetConfig()
		callbackSrv := callback.NewCallbackServer(cfg.Pipeline.CallbackPort)
		if err := callbackSrv.Start(); err != nil {
			return fmt.Errorf("start callback server: %w", err)
		}
		defer callbackSrv.Stop(cmd.Context())

		fmt.Printf("正在使用 %s 注册 %s 账号...\n", emailAddr, vendorName)

		result, err := registrar.Register(cmd.Context(), emailAccount, callbackSrv,
			provider.WithHeadless(headless),
		)
		if err != nil {
			return fmt.Errorf("register: %w", err)
		}

		fmt.Printf("注册成功!\n")
		fmt.Printf("  供应商: %s\n", result.Provider)
		fmt.Printf("  邮箱:   %s\n", result.Email)
		if result.UserInfo != nil {
			fmt.Printf("  用户ID: %s\n", result.UserInfo.ID)
		}

		return nil
	},
}

// providerListCmd 列出已注册的供应商
var providerListCmd = &cobra.Command{
	Use:   "list",
	Short: "列出所有已注册的供应商注册器",
	Run: func(cmd *cobra.Command, args []string) {
		providers := provider.List()
		if len(providers) == 0 {
			fmt.Println("暂无已注册的供应商注册器")
			return
		}
		fmt.Printf("已注册的供应商注册器 (%d):\n", len(providers))
		fmt.Printf("  %s\n", strings.Join(providers, ", "))
	},
}

func init() {
	providerRegisterCmd.Flags().StringP("vendor", "v", "", "供应商标识（必填）")
	providerRegisterCmd.Flags().StringP("email", "e", "", "邮箱地址（必填）")
	providerRegisterCmd.Flags().String("password", "", "邮箱密码（必填）")
	providerRegisterCmd.Flags().Bool("headless", true, "是否无头浏览器模式")
	_ = providerRegisterCmd.MarkFlagRequired("vendor")
	_ = providerRegisterCmd.MarkFlagRequired("email")
	_ = providerRegisterCmd.MarkFlagRequired("password")

	providerCmd.AddCommand(providerRegisterCmd, providerListCmd)
	rootCmd.AddCommand(providerCmd)
}
