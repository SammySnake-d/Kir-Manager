package softreset

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"kiro-manager/awssso"
	"kiro-manager/kiropath"
)

const (
	CustomMachineIDFileName = "custom-machine-id"
)

var (
	ErrCustomIDNotFound = errors.New("custom machine ID not found")
	ErrKiroHomeNotFound = errors.New("kiro home directory not found")
)

// SoftResetResult 軟重置結果
type SoftResetResult struct {
	OldMachineID string `json:"oldMachineId"`
	NewMachineID string `json:"newMachineId"`
	Patched      bool   `json:"patched"`
	CacheCleared bool   `json:"cacheCleared"`
}

// SoftResetStatus 軟重置狀態
type SoftResetStatus struct {
	IsPatched       bool   `json:"isPatched"`
	HasCustomID     bool   `json:"hasCustomId"`
	CustomMachineID string `json:"customMachineId"`
	ExtensionPath   string `json:"extensionPath"`
}

// GetCustomMachineIDPath 取得自訂 Machine ID 檔案路徑 (~/.kiro/custom-machine-id)
func GetCustomMachineIDPath() (string, error) {
	kiroHome, err := kiropath.GetKiroHomePath()
	if err != nil {
		return "", err
	}
	return filepath.Join(kiroHome, CustomMachineIDFileName), nil
}

// ReadCustomMachineID 讀取自訂 Machine ID（如果存在）
func ReadCustomMachineID() (string, error) {
	idPath, err := GetCustomMachineIDPath()
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(idPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", ErrCustomIDNotFound
		}
		return "", err
	}

	id := strings.TrimSpace(string(data))
	if id == "" {
		return "", ErrCustomIDNotFound
	}

	return id, nil
}

// WriteCustomMachineID 寫入自訂 Machine ID
func WriteCustomMachineID(machineID string) error {
	idPath, err := GetCustomMachineIDPath()
	if err != nil {
		return err
	}

	// 確保 ~/.kiro 目錄存在
	dir := filepath.Dir(idPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(idPath, []byte(machineID), 0644)
}

// GenerateNewMachineID 生成新的 UUID v4
func GenerateNewMachineID() string {
	return strings.ToLower(uuid.New().String())
}

// ClearCustomMachineID 刪除自訂 Machine ID 檔案（還原為系統原始值）
func ClearCustomMachineID() error {
	idPath, err := GetCustomMachineIDPath()
	if err != nil {
		return err
	}

	if _, err := os.Stat(idPath); os.IsNotExist(err) {
		return nil // 不存在就不需要刪除
	}

	return os.Remove(idPath)
}

// ClearSSOCache 刪除 SSO cache（複用 reset 模組的邏輯）
func ClearSSOCache() error {
	cachePath, err := awssso.GetSSOCachePath()
	if err != nil {
		return err
	}

	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		return nil
	}

	return os.RemoveAll(cachePath)
}

// SoftResetEnvironment 執行軟一鍵新機
func SoftResetEnvironment() (*SoftResetResult, error) {
	result := &SoftResetResult{}

	// 1. 讀取舊的自訂 Machine ID（如果有）
	oldID, _ := ReadCustomMachineID()
	result.OldMachineID = oldID

	// 2. 生成新的 Machine ID
	newID := GenerateNewMachineID()
	result.NewMachineID = newID

	// 3. 寫入自訂 Machine ID 檔案
	if err := WriteCustomMachineID(newID); err != nil {
		return result, err
	}

	// 4. Patch extension.js（如果尚未 patch）
	patched, err := IsPatched()
	if err != nil {
		return result, err
	}

	if !patched {
		if err := PatchExtensionJS(); err != nil {
			return result, err
		}
		result.Patched = true
	} else {
		result.Patched = true // 已經 patch 過
	}

	// 5. 清除 SSO cache
	if err := ClearSSOCache(); err != nil {
		return result, err
	}
	result.CacheCleared = true

	return result, nil
}

// RestoreOriginalMachineID 還原為系統原始 Machine ID
func RestoreOriginalMachineID() error {
	// 1. 刪除自訂 Machine ID 檔案
	if err := ClearCustomMachineID(); err != nil {
		return err
	}

	// 2. 清除 SSO cache
	return ClearSSOCache()
}

// GetSoftResetStatus 取得軟重置狀態
func GetSoftResetStatus() (*SoftResetStatus, error) {
	status := &SoftResetStatus{}

	// 檢查是否已 patch
	patched, err := IsPatched()
	if err == nil {
		status.IsPatched = patched
	}

	// 檢查自訂 Machine ID
	customID, err := ReadCustomMachineID()
	if err == nil {
		status.HasCustomID = true
		status.CustomMachineID = customID
	}

	// 取得 extension.js 路徑
	extPath, err := GetExtensionJSPath()
	if err == nil {
		status.ExtensionPath = extPath
	}

	return status, nil
}
