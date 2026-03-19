// Package register 是所有邮箱生产者和供应商注册器的集中注册入口。
// 在 main 包中导入此包，即可通过 init() 自动注册所有实现。
//
// 用法:
//
//	import _ "github.com/nomand-zc/lumin-actool/register"
package register

// 导入所有邮箱生产者实现（触发 init 注册）
import (
	_ "github.com/nomand-zc/lumin-actool/email/outlook"
	_ "github.com/nomand-zc/lumin-actool/email/tempmail"

	// 导入所有供应商注册器实现（触发 init 注册）

	_ "github.com/nomand-zc/lumin-actool/provider/claude"

	_ "github.com/nomand-zc/lumin-actool/provider/codex"

	_ "github.com/nomand-zc/lumin-actool/provider/copilot"

	_ "github.com/nomand-zc/lumin-actool/provider/gemini"

	_ "github.com/nomand-zc/lumin-actool/provider/kiro"
)
