package cli

import (
	"fmt"
	"os"

	"github.com/nomand-zc/lumin-actool/config"
	"github.com/spf13/cobra"
)

var (
	cfgFile string
	cfg     *config.Config
)

// rootCmd 根命令
var rootCmd = &cobra.Command{
	Use:   "actool",
	Short: "lumin-actool - AI 供应商账号批量生产工具",
	Long: `lumin-actool 是 LUMIN 生态系统中的独立 CLI 工具，
用于批量生产邮箱账号并自动注册各 AI 供应商平台，获取凭证归档文件。`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		cfg, err = config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "config.yaml", "配置文件路径")
}

// Execute 执行 CLI
func Execute() error {
	return rootCmd.Execute()
}

// GetConfig 获取当前配置（供子命令使用）
func GetConfig() *config.Config {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	return cfg
}

// exitWithError 输出错误并退出
func exitWithError(msg string, err error) {
	fmt.Fprintf(os.Stderr, "Error: %s: %v\n", msg, err)
	os.Exit(1)
}
