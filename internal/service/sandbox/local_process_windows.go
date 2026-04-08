// +build windows

package sandbox

import (
	"os/exec"
)

func setProcessGroup(cmd *exec.Cmd) {
	// Windows 不支持进程组，使用 taskkill 命令杀死进程树
}