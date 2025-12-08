package kiroversion

import (
	"errors"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"kiro-manager/internal/cmdutil"
	"kiro-manager/kiropath"
)

var (
	ErrVersionNotFound = errors.New("kiro version not found")
)

// GetKiroVersion 取得 Kiro IDE 的版本號
// 從 Kiro 執行檔的 metadata 讀取實際版本
func GetKiroVersion() (string, error) {
	switch runtime.GOOS {
	case "windows":
		return getWindowsKiroVersion()
	case "darwin":
		return getDarwinKiroVersion()
	case "linux":
		return getLinuxKiroVersion()
	default:
		return "", ErrVersionNotFound
	}
}

// getWindowsKiroVersion 使用 PowerShell 讀取 exe 的 FileVersion
func getWindowsKiroVersion() (string, error) {
	installPath, err := kiropath.GetKiroInstallPath()
	if err != nil {
		return "", err
	}

	exePath := filepath.Join(installPath, "Kiro.exe")

	// 使用 PowerShell 讀取版本資訊
	// (Get-Item "path").VersionInfo.FileVersion
	cmd := exec.Command("powershell", "-NoProfile", "-Command",
		"(Get-Item '"+exePath+"').VersionInfo.FileVersion")
	cmdutil.HideWindow(cmd)

	output, err := cmd.Output()
	if err != nil {
		return "", ErrVersionNotFound
	}

	version := strings.TrimSpace(string(output))
	if version == "" {
		return "", ErrVersionNotFound
	}

	return version, nil
}


// getDarwinKiroVersion 讀取 Kiro.app 的 Info.plist 取得版本
func getDarwinKiroVersion() (string, error) {
	installPath, err := kiropath.GetKiroInstallPath()
	if err != nil {
		return "", err
	}

	// Info.plist 位於 Kiro.app/Contents/Info.plist
	plistPath := filepath.Join(installPath, "Contents", "Info.plist")

	// 使用 defaults read 讀取 CFBundleShortVersionString
	cmd := exec.Command("defaults", "read", plistPath, "CFBundleShortVersionString")
	output, err := cmd.Output()
	if err != nil {
		// 嘗試讀取 CFBundleVersion
		cmd = exec.Command("defaults", "read", plistPath, "CFBundleVersion")
		output, err = cmd.Output()
		if err != nil {
			return "", ErrVersionNotFound
		}
	}

	version := strings.TrimSpace(string(output))
	if version == "" {
		return "", ErrVersionNotFound
	}

	return version, nil
}

// getLinuxKiroVersion 嘗試從常見位置讀取版本資訊
func getLinuxKiroVersion() (string, error) {
	installPath, err := kiropath.GetKiroInstallPath()
	if err != nil {
		return "", err
	}

	// 嘗試讀取 package.json 或 version 檔案
	// Electron 應用通常會有 resources/app/package.json
	packageJsonPath := filepath.Join(installPath, "resources", "app", "package.json")

	cmd := exec.Command("grep", "-oP", `"version"\s*:\s*"\K[^"]+`, packageJsonPath)
	output, err := cmd.Output()
	if err != nil {
		return "", ErrVersionNotFound
	}

	version := strings.TrimSpace(string(output))
	if version == "" {
		return "", ErrVersionNotFound
	}

	return version, nil
}
