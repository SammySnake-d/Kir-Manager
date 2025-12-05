//go:build windows

package cmdutil

import (
	"os/exec"
	"syscall"
)

// HideWindow 設定命令以隱藏視窗方式執行
// 使用 STARTF_USESHOWWINDOW + SW_HIDE 來隱藏視窗
// 這種方式比 CREATE_NO_WINDOW 更不容易觸發防毒軟體誤報
func HideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
	}
}
