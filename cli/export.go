package cli

import (
	"fmt"

	"github.com/nomand-zc/lumin-actool/export"
	"github.com/nomand-zc/lumin-actool/provider"
	"github.com/spf13/cobra"
)

// exportCmd 导出子命令
var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "导出凭证归档文件",
	Example: `  actool export --vendor kiro --output ./credentials
  actool export --vendor claude --format json --output ./credentials`,
	RunE: func(cmd *cobra.Command, args []string) error {
		vendor, _ := cmd.Flags().GetString("vendor")
		outputDir, _ := cmd.Flags().GetString("output")
		format, _ := cmd.Flags().GetString("format")

		if outputDir == "" {
			cfg := GetConfig()
			outputDir = cfg.Export.OutputDir
		}

		_ = vendor  // TODO: 从存储中按供应商过滤凭证
		_ = format  // TODO: 支持 zip 格式

		exporter := export.NewJSONExporter()

		// TODO: 从存储中加载凭证数据
		var results []*provider.RegistrationResult

		if len(results) == 0 {
			fmt.Println("暂无可导出的凭证")
			return nil
		}

		path, err := exporter.Export(cmd.Context(), results,
			export.WithOutputDir(outputDir),
		)
		if err != nil {
			return fmt.Errorf("export: %w", err)
		}

		fmt.Printf("凭证已导出到: %s\n", path)
		return nil
	},
}

func init() {
	exportCmd.Flags().StringP("vendor", "v", "", "供应商标识（过滤）")
	exportCmd.Flags().StringP("output", "o", "", "输出目录")
	exportCmd.Flags().String("format", "json", "导出格式 (json|zip)")

	rootCmd.AddCommand(exportCmd)
}
