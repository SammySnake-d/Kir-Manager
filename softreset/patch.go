package softreset

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"kiro-manager/kiropath"
)

const (
	// PatchMarker 用於識別是否已 patch 的標記
	PatchMarker    = "/* KIRO_MANAGER_PATCH_V2 */"
	PatchEndMarker = "/* END_KIRO_MANAGER_PATCH */"
	BackupSuffix   = ".kiro-manager-backup"
	// OldPatchMarker 用於識別舊版 patch，需要重新 patch
	OldPatchMarker = "/* KIRO_MANAGER_PATCH_V1 */"
)

var (
	ErrExtensionNotFound = errors.New("extension.js not found")
	ErrAlreadyPatched    = errors.New("extension.js is already patched")
	ErrNotPatched        = errors.New("extension.js is not patched")
	ErrBackupNotFound    = errors.New("backup file not found")
)

// patchCode 注入的 JavaScript 程式碼
const patchCode = `/* KIRO_MANAGER_PATCH_V2 */
(function() {
  const fs = require('fs');
  const path = require('path');
  const os = require('os');
  const customIdPath = path.join(os.homedir(), '.kiro', 'custom-machine-id');
  let customMachineId = null;
  try {
    customMachineId = fs.readFileSync(customIdPath, 'utf8').trim();
  } catch {}
  if (customMachineId) {
    const Module = require('module');
    const originalLoad = Module._load;
    Module._load = function(request, parent, isMain) {
      const mod = originalLoad.call(this, request, parent, isMain);
      if (request === 'vscode') {
        return new Proxy(mod, {
          get(target, prop) {
            if (prop === 'env') {
              return new Proxy(target.env, {
                get(envTarget, envProp) {
                  if (envProp === 'machineId') return customMachineId;
                  return envTarget[envProp];
                }
              });
            }
            return target[prop];
          }
        });
      }
      if (mod && typeof mod === 'object' && (typeof mod.machineIdSync === 'function' || typeof mod.machineId === 'function')) {
        return new Proxy(mod, {
          get(target, prop) {
            if (prop === 'machineIdSync') return function() { return customMachineId; };
            if (prop === 'machineId') return function() { return Promise.resolve(customMachineId); };
            return target[prop];
          }
        });
      }
      return mod;
    };
  }
})();
/* END_KIRO_MANAGER_PATCH */
`


// GetExtensionJSPath 取得 extension.js 的路徑
func GetExtensionJSPath() (string, error) {
	installPath, err := kiropath.GetKiroInstallPath()
	if err != nil {
		return "", err
	}

	var extensionPath string
	switch runtime.GOOS {
	case "windows":
		// Windows: {install}/resources/app/extensions/kiro.kiro-agent/dist/extension.js
		extensionPath = filepath.Join(installPath, "resources", "app", "extensions", "kiro.kiro-agent", "dist", "extension.js")
	case "darwin":
		// macOS: {install}/Contents/Resources/app/extensions/kiro.kiro-agent/dist/extension.js
		extensionPath = filepath.Join(installPath, "Contents", "Resources", "app", "extensions", "kiro.kiro-agent", "dist", "extension.js")
	case "linux":
		// Linux: {install}/resources/app/extensions/kiro.kiro-agent/dist/extension.js
		extensionPath = filepath.Join(installPath, "resources", "app", "extensions", "kiro.kiro-agent", "dist", "extension.js")
	default:
		return "", errors.New("unsupported platform: " + runtime.GOOS)
	}

	if _, err := os.Stat(extensionPath); os.IsNotExist(err) {
		return "", ErrExtensionNotFound
	}

	return extensionPath, nil
}

// IsPatched 檢查 extension.js 是否已被 patch（當前版本）
func IsPatched() (bool, error) {
	extPath, err := GetExtensionJSPath()
	if err != nil {
		return false, err
	}

	// 只讀取檔案開頭部分來檢查
	file, err := os.Open(extPath)
	if err != nil {
		return false, err
	}
	defer file.Close()

	// 讀取前 1KB 來檢查標記
	buf := make([]byte, 1024)
	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		return false, err
	}

	return strings.Contains(string(buf[:n]), PatchMarker), nil
}

// IsOldPatched 檢查 extension.js 是否被舊版 patch
func IsOldPatched() (bool, error) {
	extPath, err := GetExtensionJSPath()
	if err != nil {
		return false, err
	}

	file, err := os.Open(extPath)
	if err != nil {
		return false, err
	}
	defer file.Close()

	buf := make([]byte, 1024)
	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		return false, err
	}

	content := string(buf[:n])
	// 有舊版標記但沒有新版標記
	return strings.Contains(content, OldPatchMarker) && !strings.Contains(content, PatchMarker), nil
}

// BackupExtensionJS 備份原始 extension.js
func BackupExtensionJS() error {
	extPath, err := GetExtensionJSPath()
	if err != nil {
		return err
	}

	backupPath := extPath + BackupSuffix

	// 如果備份已存在，不覆蓋
	if _, err := os.Stat(backupPath); err == nil {
		return nil
	}

	return copyFile(extPath, backupPath)
}

// RestoreExtensionJS 從備份還原 extension.js
func RestoreExtensionJS() error {
	extPath, err := GetExtensionJSPath()
	if err != nil {
		return err
	}

	backupPath := extPath + BackupSuffix

	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return ErrBackupNotFound
	}

	return copyFile(backupPath, extPath)
}

// PatchExtensionJS 在 extension.js 開頭注入攔截程式碼
func PatchExtensionJS() error {
	extPath, err := GetExtensionJSPath()
	if err != nil {
		return err
	}

	// 檢查是否已是最新版 patch
	patched, err := IsPatched()
	if err != nil {
		return err
	}
	if patched {
		return nil // 已經是最新版 patch，不重複處理
	}

	// 檢查是否有舊版 patch，需要先移除
	oldPatched, err := IsOldPatched()
	if err != nil {
		return err
	}
	if oldPatched {
		// 移除舊版 patch
		if err := UnpatchExtensionJS(); err != nil {
			return err
		}
	}

	// 備份原始檔案
	if err := BackupExtensionJS(); err != nil {
		return err
	}

	// 讀取原始內容
	content, err := os.ReadFile(extPath)
	if err != nil {
		return err
	}

	// 在開頭加入 patch 程式碼
	newContent := patchCode + string(content)

	// 寫回檔案
	return os.WriteFile(extPath, []byte(newContent), 0644)
}

// UnpatchExtensionJS 移除注入的程式碼
func UnpatchExtensionJS() error {
	extPath, err := GetExtensionJSPath()
	if err != nil {
		return err
	}

	// 檢查是否有任何版本的 patch
	patched, err := IsPatched()
	if err != nil {
		return err
	}
	oldPatched, err := IsOldPatched()
	if err != nil {
		return err
	}
	if !patched && !oldPatched {
		return nil // 沒有任何 patch，不需要處理
	}

	// 讀取內容
	content, err := os.ReadFile(extPath)
	if err != nil {
		return err
	}

	contentStr := string(content)

	// 找到 patch 結束標記的位置
	endIdx := strings.Index(contentStr, PatchEndMarker)
	if endIdx == -1 {
		// 找不到結束標記，嘗試從備份還原
		return RestoreExtensionJS()
	}

	// 移除 patch 程式碼（包含結束標記和換行）
	endIdx += len(PatchEndMarker)
	if endIdx < len(contentStr) && contentStr[endIdx] == '\n' {
		endIdx++
	}

	newContent := contentStr[endIdx:]

	return os.WriteFile(extPath, []byte(newContent), 0644)
}

// copyFile 複製檔案
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	return dstFile.Sync()
}
