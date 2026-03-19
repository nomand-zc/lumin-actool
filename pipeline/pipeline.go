package pipeline

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nomand-zc/lumin-actool/callback"
	"github.com/nomand-zc/lumin-actool/email"
	"github.com/nomand-zc/lumin-actool/export"
	"github.com/nomand-zc/lumin-actool/provider"
)

// PipelineConfig 流水线配置
type PipelineConfig struct {
	EmailProducer   string // 邮箱生产者标识
	ProviderRegistrar string // 供应商注册者标识
	Count           int    // 生产数量
	Concurrency     int    // 并发数
	RetryCount      int    // 重试次数
	OutputDir       string // 输出目录
}

// ProgressCallback 进度回调
type ProgressCallback func(completed, total int, current *Task)

// Pipeline 批量生产编排引擎接口
type Pipeline interface {
	// Run 执行批量生产流水线
	// 阶段一：批量生产邮箱 → 阶段二：批量注册供应商 → 阶段三：导出凭证归档
	Run(ctx context.Context, config *PipelineConfig, progress ProgressCallback) ([]*Task, error)
}

// DefaultPipeline 默认流水线实现
type DefaultPipeline struct {
	callbackPort int
	exporter     export.CredentialExporter
}

// NewPipeline 创建默认流水线实例
func NewPipeline(callbackPort int, exporter export.CredentialExporter) Pipeline {
	return &DefaultPipeline{
		callbackPort: callbackPort,
		exporter:     exporter,
	}
}

func (p *DefaultPipeline) Run(ctx context.Context, config *PipelineConfig, progress ProgressCallback) ([]*Task, error) {
	// 1. 获取邮箱生产者
	emailProducer, err := email.Get(config.EmailProducer)
	if err != nil {
		return nil, fmt.Errorf("pipeline: get email producer: %w", err)
	}

	// 2. 获取供应商注册者
	providerRegistrar, err := provider.Get(config.ProviderRegistrar)
	if err != nil {
		return nil, fmt.Errorf("pipeline: get provider registrar: %w", err)
	}

	// 3. 启动统一回调服务器
	callbackSrv := callback.NewCallbackServer(p.callbackPort)
	if err := callbackSrv.Start(); err != nil {
		return nil, fmt.Errorf("pipeline: start callback server: %w", err)
	}
	defer callbackSrv.Stop(ctx)

	// 4. 阶段一：批量生产邮箱
	accounts, err := emailProducer.Produce(ctx, config.Count)
	if err != nil {
		return nil, fmt.Errorf("pipeline: produce emails: %w", err)
	}

	if len(accounts) == 0 {
		return nil, fmt.Errorf("pipeline: no email accounts produced")
	}

	// 5. 创建任务列表
	tasks := make([]*Task, len(accounts))
	for i, acct := range accounts {
		tasks[i] = &Task{
			ID:           uuid.New().String(),
			EmailAccount: acct,
			Status:       TaskPending,
		}
	}

	// 6. 阶段二：并发注册供应商账号
	concurrency := config.Concurrency
	if concurrency <= 0 {
		concurrency = 1
	}

	var (
		wg        sync.WaitGroup
		taskCh    = make(chan *Task, len(tasks))
		completed int
		mu        sync.Mutex
	)

	// 填充任务队列
	for _, t := range tasks {
		taskCh <- t
	}
	close(taskCh)

	// 启动工作协程
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range taskCh {
				p.executeTask(ctx, task, providerRegistrar, callbackSrv, config.RetryCount)

				mu.Lock()
				completed++
				if progress != nil {
					progress(completed, len(tasks), task)
				}
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	// 7. 阶段三：导出凭证归档
	var successResults []*provider.RegistrationResult
	for _, t := range tasks {
		if t.Status == TaskSuccess && t.Result != nil {
			successResults = append(successResults, t.Result)
		}
	}

	if len(successResults) > 0 && p.exporter != nil && config.OutputDir != "" {
		_, exportErr := p.exporter.Export(ctx, successResults,
			export.WithOutputDir(config.OutputDir),
		)
		if exportErr != nil {
			// 导出失败不影响整体结果，仅记录
			fmt.Printf("warning: export credentials failed: %v\n", exportErr)
		}
	}

	return tasks, nil
}

// executeTask 执行单个注册任务（含重试）
func (p *DefaultPipeline) executeTask(
	ctx context.Context,
	task *Task,
	registrar provider.ProviderRegistrar,
	callbackSrv callback.CallbackServer,
	maxRetries int,
) {
	task.Status = TaskRunning

	for attempt := 0; attempt <= maxRetries; attempt++ {
		task.Retries = attempt

		result, err := registrar.Register(ctx, task.EmailAccount, callbackSrv)
		if err == nil {
			task.Result = result
			task.Status = TaskSuccess
			task.Error = nil
			return
		}

		task.Error = err

		// 最后一次重试仍然失败
		if attempt == maxRetries {
			break
		}

		// 等待一段时间后重试
		select {
		case <-ctx.Done():
			task.Status = TaskFailed
			task.Error = ctx.Err()
			return
		case <-time.After(time.Duration(attempt+1) * 5 * time.Second):
			// 指数退避
		}
	}

	task.Status = TaskFailed
}
