package main

import (
	"os"

	"github.com/nomand-zc/lumin-actool/cli"

	// 导入 register 包触发所有邮箱生产者和供应商注册器的自动注册
	_ "github.com/nomand-zc/lumin-actool/register"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
