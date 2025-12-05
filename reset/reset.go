package reset

import (
	"errors"
	"os"
	"runtime"
	"strings"

	"github.com/google/uuid"

	"kiro-manager/awssso"
	"kiro-manager/backup"
	"kiro-manager/machineid"
)

var (
	ErrNotWindows          = errors.New("machine ID replacement is only supported on Windows")
	ErrRequiresAdmin       = errors.New("modifying machine ID requires administrator privileges")
	ErrBackupRequired      = errors.New("current machine ID is not backed up")
	ErrCacheNotFound       = errors.New("SSO cache directory not found")
)

// ResetResult 代表重置操作的結果
type ResetResult struct {
	CacheCleared    bool   `json:"cacheCleared"`
	OldMachineID    string `json:"oldMachineId"`
	NewMachineID    string `json:"newMachineId"`
	MachineIDChanged bool  `json:"machineIdChanged"`
}

// ClearSSOCache 刪除 ~/.aws/sso/cache 資料夾
func ClearSSOCache() error {
	cachePath, err := awssso.GetSSOCachePath()
	if err != nil {
		return err
	}

	// 檢查資料夾是否存在
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		return nil // 不存在就不需要刪除
	}

	return os.RemoveAll(cachePath)
}

// GenerateNewMachineID 使用 UUID v4 生成新的 Machine ID
func GenerateNewMachineID() string {
	return strings.ToLower(uuid.New().String())
}


// SetWindowsMachineID 設定 Windows Registry 中的 MachineGuid
// 需要管理員權限
// 使用 Windows Registry API 直接寫入，無視窗閃爍問題
func SetWindowsMachineID(newGUID string) error {
	if runtime.GOOS != "windows" {
		return ErrNotWindows
	}
	return setWindowsMachineIDNative(newGUID)
}

// IsCurrentMachineIDBackedUp 檢查當前的 Machine ID 是否已在備份庫中
func IsCurrentMachineIDBackedUp() (bool, string, error) {
	// 取得當前 Machine ID
	currentID, err := machineid.GetRawMachineId()
	if err != nil {
		return false, "", err
	}

	// 列出所有備份
	backups, err := backup.ListBackups()
	if err != nil {
		return false, currentID, err
	}

	// 檢查每個備份的 Machine ID
	for _, b := range backups {
		mid, err := backup.ReadBackupMachineID(b.Name)
		if err != nil {
			continue
		}
		if strings.EqualFold(mid.MachineID, currentID) {
			return true, currentID, nil
		}
	}

	return false, currentID, nil
}


// ResetEnvironment 執行完整的一鍵新機流程
// 參數 skipBackupCheck: 若為 true，則跳過備份檢查
// 回傳 ResetResult 和 error
// 若當前 Machine ID 未備份且 skipBackupCheck 為 false，回傳 ErrBackupRequired
func ResetEnvironment(skipBackupCheck bool) (*ResetResult, error) {
	if runtime.GOOS != "windows" {
		return nil, ErrNotWindows
	}

	result := &ResetResult{}

	// 1. 取得當前 Machine ID
	oldMachineID, err := machineid.GetRawMachineId()
	if err != nil {
		return nil, err
	}
	result.OldMachineID = oldMachineID

	// 2. 檢查是否已備份（除非跳過）
	if !skipBackupCheck {
		isBackedUp, _, err := IsCurrentMachineIDBackedUp()
		if err != nil {
			return nil, err
		}
		if !isBackedUp {
			return result, ErrBackupRequired
		}
	}

	// 3. 刪除 SSO cache 資料夾
	if err := ClearSSOCache(); err != nil {
		return result, err
	}
	result.CacheCleared = true

	// 4. 生成新的 Machine ID
	newMachineID := GenerateNewMachineID()
	result.NewMachineID = newMachineID

	// 5. 寫入 Registry
	if err := SetWindowsMachineID(newMachineID); err != nil {
		return result, err
	}
	result.MachineIDChanged = true

	return result, nil
}
