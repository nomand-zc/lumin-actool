package cli

import (
	"fmt"

	"github.com/nomand-zc/lumin-actool/export"
	plpkg "github.com/nomand-zc/lumin-actool/pipeline"
	"github.com/spf13/cobra"
)

// pipelineCmd pipeline 子命令组
var pipelineCmd = &cobra.Command{
	Use:   "pipeline",
	Short: "批量生产流水线",
	Long:  "一键执行邮箱生产 + 供应商注册 + 凭证导出的完整流水线",
}

// pipelineRunCmd 运行流水线子命令
var pipelineRunCmd = &cobra.Command{
	Use:   "run",
	Short: "执行批量生产流水线",
	Example: `  actool pipeline run --email-provider tempmail --vendor kiro --count 10
  actool pipeline run --email-provider outlook --vendor claude --count 5 --concurrency 3 --output ./credentials`,
	RunE: func(cmd *cobra.Command, args []string) error {
		emailProvider, _ := cmd.Flags().GetString("email-provider")
		vendor, _ := cmd.Flags().GetString("vendor")
		count, _ := cmd.Flags().GetInt("count")
		concurrency, _ := cmd.Flags().GetInt("concurrency")
		retryCount, _ := cmd.Flags().GetInt("retry")
		outputDir, _ := cmd.Flags().GetString("output")

		cfg := GetConfig()

		if concurrency <= 0 {
			concurrency = cfg.Pipeline.Concurrency
		}
		if retryCount < 0 {
			retryCount = cfg.Pipeline.RetryCount
		}
		if outputDir == "" {
			outputDir = cfg.Export.OutputDir
		}

		exporter := export.NewJSONExporter()
		pl := plpkg.NewPipeline(cfg.Pipeline.CallbackPort, exporter)

		plConfig := &plpkg.PipelineConfig{
			EmailProducer:     emailProvider,
			ProviderRegistrar: vendor,
			Count:             count,
			Concurrency:       concurrency,
			RetryCount:        retryCount,
			OutputDir:         outputDir,
		}

		fmt.Printf("启动流水线: email=%s, vendor=%s, count=%d, concurrency=%d\n",
			emailProvider, vendor, count, concurrency)

		tasks, err := pl.Run(cmd.Context(), plConfig, func(completed, total int, current *plpkg.Task) {
			status := "✅"
			if current.Status == plpkg.TaskFailed {
				status = "❌"
			}
			fmt.Printf("  [%d/%d] %s %s\n", completed, total, status, current.EmailAccount.Email)
		})
		if err != nil {
			return fmt.Errorf("pipeline run: %w", err)
		}

		// 统计结果
		var successCount, failedCount int
		for _, t := range tasks {
			switch t.Status {
			case plpkg.TaskSuccess:
				successCount++
			case plpkg.TaskFailed:
				failedCount++
			}
		}

		fmt.Printf("\n流水线执行完成: 总计=%d, 成功=%d, 失败=%d\n",
			len(tasks), successCount, failedCount)

		if successCount > 0 {
			fmt.Printf("凭证已导出到: %s\n", outputDir)
		}

		return nil
	},
}

func init() {
	pipelineRunCmd.Flags().String("email-provider", "", "邮箱提供商（必填）")
	pipelineRunCmd.Flags().StringP("vendor", "v", "", "供应商标识（必填）")
	pipelineRunCmd.Flags().IntP("count", "c", 1, "生产数量")
	pipelineRunCmd.Flags().Int("concurrency", 0, "并发数（默认使用配置值）")
	pipelineRunCmd.Flags().Int("retry", -1, "重试次数（默认使用配置值）")
	pipelineRunCmd.Flags().StringP("output", "o", "", "输出目录（默认使用配置值）")
	_ = pipelineRunCmd.MarkFlagRequired("email-provider")
	_ = pipelineRunCmd.MarkFlagRequired("vendor")

	pipelineCmd.AddCommand(pipelineRunCmd)
	rootCmd.AddCommand(pipelineCmd)
}
