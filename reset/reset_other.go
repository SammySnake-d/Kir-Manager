//go:build !windows

package reset

// setWindowsMachineIDNative 非 Windows 平台的空實作
func setWindowsMachineIDNative(newGUID string) error {
	return ErrNotWindows
}
