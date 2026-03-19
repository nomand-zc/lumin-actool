package export

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/nomand-zc/lumin-actool/provider"
)

// ExportFormat 导出格式
type ExportFormat string

const (
	FormatJSON ExportFormat = "json" // 导出为独立 JSON 文件目录
	FormatZip  ExportFormat = "zip"  // 导出为 zip 压缩归档
)

// CredentialExporter 凭证导出接口。
// 将注册结果导出为 lumin-acpool 可导入的凭证归档文件。
type CredentialExporter interface {
	// Export 将注册结果导出为归档文件。
	// 每个凭证生成一个独立的 JSON 文件，最终打包为压缩归档。
	// 返回输出文件路径。
	Export(ctx context.Context, results []*provider.RegistrationResult, opts ...ExportOption) (string, error)
}

// ExportOption 导出选项函数
type ExportOption func(*ExportOptions)

// ExportOptions 导出配置
type ExportOptions struct {
	OutputDir  string       // 输出目录
	Format     ExportFormat // 归档格式
	FilePrefix string       // 文件名前缀
}

// ApplyExportOptions 应用导出选项
func ApplyExportOptions(opts ...ExportOption) *ExportOptions {
	o := &ExportOptions{
		OutputDir:  "./output",
		Format:     FormatJSON,
		FilePrefix: "credential",
	}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// WithOutputDir 设置输出目录
func WithOutputDir(dir string) ExportOption {
	return func(o *ExportOptions) {
		o.OutputDir = dir
	}
}

// WithFormat 设置导出格式
func WithFormat(format ExportFormat) ExportOption {
	return func(o *ExportOptions) {
		o.Format = format
	}
}

// WithFilePrefix 设置文件名前缀
func WithFilePrefix(prefix string) ExportOption {
	return func(o *ExportOptions) {
		o.FilePrefix = prefix
	}
}

// JSONExporter 将凭证导出为独立的 JSON 文件
type JSONExporter struct{}

// NewJSONExporter 创建 JSON 导出器
func NewJSONExporter() CredentialExporter {
	return &JSONExporter{}
}

func (e *JSONExporter) Export(ctx context.Context, results []*provider.RegistrationResult, opts ...ExportOption) (string, error) {
	o := ApplyExportOptions(opts...)

	// 确保输出目录存在
	if err := os.MkdirAll(o.OutputDir, 0755); err != nil {
		return "", fmt.Errorf("export: create output dir: %w", err)
	}

	for i, result := range results {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		// 构造输出数据：凭证 + 元信息
		output := map[string]any{
			"provider":      result.Provider,
			"email":         result.Email,
			"credential":    result.Credential,
			"registered_at": result.RegisteredAt.Format(time.RFC3339),
		}
		if result.ExpiresAt != nil {
			output["expires_at"] = result.ExpiresAt.Format(time.RFC3339)
		}
		if result.UserInfo != nil {
			output["user_info"] = result.UserInfo
		}

		data, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			return "", fmt.Errorf("export: marshal credential %d: %w", i, err)
		}

		filename := fmt.Sprintf("%s-%s-%d.json", o.FilePrefix, result.Provider, i+1)
		filePath := filepath.Join(o.OutputDir, filename)

		if err := os.WriteFile(filePath, data, 0644); err != nil {
			return "", fmt.Errorf("export: write file %s: %w", filePath, err)
		}
	}

	return o.OutputDir, nil
}
