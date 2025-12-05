//go:build windows

package reset

import (
	"os/exec"
	"strings"

	"kiro-manager/internal/cmdutil"
)

// setWindowsMachineIDNative 使用 reg.exe 修改 Registry 中的 MachineGuid
// 使用系統內建工具避免防毒軟體誤報
// 需要管理員權限
func setWindowsMachineIDNative(newGUID string) error {
	// reg add "HKLM\SOFTWARE\Microsoft\Cryptography" /v MachineGuid /t REG_SZ /d "xxx" /f
	cmd := exec.Command("reg", "add",
		`HKLM\SOFTWARE\Microsoft\Cryptography`,
		"/v", "MachineGuid",
		"/t", "REG_SZ",
		"/d", newGUID,
		"/f")
	cmdutil.HideWindow(cmd)

	output, err := cmd.CombinedOutput()
	if err != nil {
		// 檢查是否為權限不足
		outputStr := string(output)
		if strings.Contains(outputStr, "拒絕存取") ||
			strings.Contains(outputStr, "Access is denied") ||
			strings.Contains(outputStr, "ERROR: Access is denied") {
			return ErrRequiresAdmin
		}
		return err
	}

	return nil
}
