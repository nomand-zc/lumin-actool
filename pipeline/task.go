package pipeline

import (
	"github.com/nomand-zc/lumin-actool/email"
	"github.com/nomand-zc/lumin-actool/provider"
)

// TaskStatus 任务状态
type TaskStatus int

const (
	TaskPending TaskStatus = iota // 待执行
	TaskRunning                   // 执行中
	TaskSuccess                   // 成功
	TaskFailed                    // 失败
	TaskSkipped                   // 跳过
)

// String 返回任务状态的字符串表示
func (s TaskStatus) String() string {
	switch s {
	case TaskPending:
		return "pending"
	case TaskRunning:
		return "running"
	case TaskSuccess:
		return "success"
	case TaskFailed:
		return "failed"
	case TaskSkipped:
		return "skipped"
	default:
		return "unknown"
	}
}

// Task 单个账号生产任务
type Task struct {
	// ID 任务唯一标识
	ID string
	// EmailAccount 邮箱账号（阶段一的产出）
	EmailAccount *email.EmailAccount
	// Result 注册结果（阶段二的产出）
	Result *provider.RegistrationResult
	// Status 任务状态
	Status TaskStatus
	// Error 失败时的错误信息
	Error error
	// Retries 已重试次数
	Retries int
}
