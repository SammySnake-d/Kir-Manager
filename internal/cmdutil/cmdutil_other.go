//go:build !windows

package cmdutil

import "os/exec"

// HideWindow 非 Windows 平台不需要處理
func HideWindow(cmd *exec.Cmd) {
	// no-op on non-Windows platforms
}
