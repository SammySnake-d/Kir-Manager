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
	PatchMarker    = "/* KIRO_MANAGER_PATCH_V3 */"
	PatchEndMarker = "/* END_KIRO_MANAGER_PATCH */"
	BackupSuffix   = ".kiro-manager-backup"
	// OldPatchMarker 用於識別舊版 patch，需要重新 patch
	OldPatchMarker   = "/* KIRO_MANAGER_PATCH_V1 */"
	OldPatchMarkerV2 = "/* KIRO_MANAGER_PATCH_V2 */"
)

var (
	ErrExtensionNotFound = errors.New("extension.js not found")
	ErrAlreadyPatched    = errors.New("extension.js is already patched")
	ErrNotPatched        = errors.New("extension.js is not patched")
	ErrBackupNotFound    = errors.New("backup file not found")
)

// patchCode 注入的 JavaScript 程式碼
// V3: 底層全面攔截 - 覆蓋 vscode.env.machineId, node-machine-id, child_process, fs
const patchCode = `/* KIRO_MANAGER_PATCH_V3 */
(function() {
  const fs = require('fs');
  const path = require('path');
  const os = require('os');
  const childProcess = require('child_process');
  const customIdPath = path.join(os.homedir(), '.kiro', 'custom-machine-id');
  let customMachineId = null;
  try {
    customMachineId = fs.readFileSync(customIdPath, 'utf8').trim();
  } catch {}
  if (!customMachineId) return;

  // 1. 攔截 Module._load（vscode.env.machineId 和 node-machine-id）
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
          if (prop === 'machineIdSync') return () => customMachineId;
          if (prop === 'machineId') return () => Promise.resolve(customMachineId);
          return target[prop];
        }
      });
    }
    return mod;
  };

  // 2. 攔截 child_process（針對 @opentelemetry 和其他直接執行命令的模組）
  const machineIdPatterns = [
    'REG.exe QUERY', 'REG QUERY', 'MachineGuid',
    'ioreg', 'IOPlatformExpertDevice',
    'kenv', 'smbios.system.uuid', 'kern.hostuuid'
  ];
  const isMachineIdCmd = (cmd) => cmd && machineIdPatterns.some(p => cmd.includes(p));

  const originalExec = childProcess.exec;
  childProcess.exec = function(cmd, options, callback) {
    if (isMachineIdCmd(cmd)) {
      if (typeof options === 'function') { callback = options; options = {}; }
      setImmediate(() => callback && callback(null, customMachineId, ''));
      return { on: () => {}, stdout: { on: () => {} }, stderr: { on: () => {} } };
    }
    return originalExec.apply(this, arguments);
  };

  const originalExecSync = childProcess.execSync;
  childProcess.execSync = function(cmd, options) {
    if (isMachineIdCmd(cmd)) return Buffer.from(customMachineId);
    return originalExecSync.apply(this, arguments);
  };

  // 3. 攔截 fs（針對 Linux /etc/machine-id）
  const machineIdPaths = ['/etc/machine-id', '/var/lib/dbus/machine-id', '/etc/hostid'];
  const isMachineIdPath = (p) => p && machineIdPaths.some(mp => String(p).includes(mp));

  const originalReadFile = fs.readFile;
  fs.readFile = function(filePath, options, callback) {
    if (isMachineIdPath(filePath)) {
      if (typeof options === 'function') { callback = options; }
      setImmediate(() => callback && callback(null, customMachineId));
      return;
    }
    return originalReadFile.apply(this, arguments);
  };

  const originalReadFileSync = fs.readFileSync;
  fs.readFileSync = function(filePath, options) {
    if (isMachineIdPath(filePath)) return customMachineId;
    return originalReadFileSync.apply(this, arguments);
  };

  if (fs.promises) {
    const originalPromisesReadFile = fs.promises.readFile;
    fs.promises.readFile = async function(filePath, options) {
      if (isMachineIdPath(filePath)) return customMachineId;
      return originalPromisesReadFile.apply(this, arguments);
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

// IsOldPatched 檢查 extension.js 是否被舊版 patch（V1 或 V2）
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
	// 有舊版標記（V1 或 V2）但沒有新版標記（V3）
	hasOldPatch := strings.Contains(content, OldPatchMarker) || strings.Contains(content, OldPatchMarkerV2)
	hasCurrentPatch := strings.Contains(content, PatchMarker)
	return hasOldPatch && !hasCurrentPatch, nil
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

	// 還原檔案
	if err := copyFile(backupPath, extPath); err != nil {
		return err
	}

	// 還原成功後刪除備份檔案
	_ = os.Remove(backupPath)

	return nil
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
