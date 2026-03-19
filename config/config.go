package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 全局配置
type Config struct {
	Email    EmailConfig    `yaml:"email"`
	Provider ProviderConfig `yaml:"provider"`
	Browser  BrowserConfig  `yaml:"browser"`
	Pipeline PipelineConfig `yaml:"pipeline"`
	Storage  StorageConfig  `yaml:"storage"`
	Export   ExportConfig   `yaml:"export"`
}

// EmailConfig 邮箱生产配置
type EmailConfig struct {
	Providers map[string]EmailProviderConfig `yaml:"providers"`
}

// EmailProviderConfig 单个邮箱提供商配置
type EmailProviderConfig struct {
	Proxy      string `yaml:"proxy"`
	NamePrefix string `yaml:"name_prefix"`
	Domain     string `yaml:"domain"`
	APIURL     string `yaml:"api_url"`
	TTL        int    `yaml:"ttl"` // 临时邮箱存活秒数
}

// ProviderConfig 供应商注册配置
type ProviderConfig struct {
	Registrars map[string]ProviderRegistrarConfig `yaml:"registrars"`
}

// ProviderRegistrarConfig 单个供应商注册者配置
type ProviderRegistrarConfig struct {
	AuthMethod string        `yaml:"auth_method"`
	Headless   bool          `yaml:"headless"`
	Timeout    time.Duration `yaml:"timeout"`
	Proxy      string        `yaml:"proxy"`
	ProjectID  string        `yaml:"project_id"`
}

// BrowserConfig 浏览器自动化配置
type BrowserConfig struct {
	BinaryPath   string `yaml:"binary_path"`
	Headless     bool   `yaml:"headless"`
	UserDataBase string `yaml:"user_data_base"`
}

// PipelineConfig 流水线配置
type PipelineConfig struct {
	Concurrency  int `yaml:"concurrency"`
	RetryCount   int `yaml:"retry_count"`
	CallbackPort int `yaml:"callback_port"`
}

// StorageConfig 存储配置
type StorageConfig struct {
	Type string `yaml:"type"` // "sqlite"
	Path string `yaml:"path"`
}

// ExportConfig 导出配置
type ExportConfig struct {
	OutputDir string `yaml:"output_dir"`
	Format    string `yaml:"format"` // "json", "zip"
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Browser: BrowserConfig{
			Headless:     true,
			UserDataBase: "./browser_profiles",
		},
		Pipeline: PipelineConfig{
			Concurrency:  3,
			RetryCount:   2,
			CallbackPort: 19800,
		},
		Storage: StorageConfig{
			Type: "sqlite",
			Path: "./actool.db",
		},
		Export: ExportConfig{
			OutputDir: "./output",
			Format:    "json",
		},
	}
}

// Load 从文件加载配置
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("config: read file: %w", err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("config: parse yaml: %w", err)
	}

	return cfg, nil
}

// Save 保存配置到文件
func Save(cfg *Config, path string) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("config: marshal yaml: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("config: write file: %w", err)
	}

	return nil
}
