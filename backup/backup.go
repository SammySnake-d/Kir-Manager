package backup

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"kiro-manager/awssso"
	"kiro-manager/machineid"
)

const (
	BackupDirName       = "backups"
	MachineIDFileName   = "machine-id.json"
	KiroAuthTokenFile   = "kiro-auth-token.json"
)

var (
	ErrBackupNotFound    = errors.New("backup not found")
	ErrBackupExists      = errors.New("backup already exists")
	ErrInvalidBackupName = errors.New("invalid backup name")
	ErrNoTokenToBackup   = errors.New("no kiro auth token to backup")
)

// MachineIDBackup 代表備份的 Machine ID 結構
type MachineIDBackup struct {
	MachineID  string `json:"machineId"`
	BackupTime string `json:"backupTime"`
}

// BackupInfo 代表備份的基本資訊
type BackupInfo struct {
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	BackupTime time.Time `json:"backupTime"`
	HasToken   bool      `json:"hasToken"`
	HasMachineID bool    `json:"hasMachineId"`
}

// GetBackupRootPath 取得備份根目錄（執行檔同層的 backups 資料夾）
func GetBackupRootPath() (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", err
	}
	execDir := filepath.Dir(execPath)
	return filepath.Join(execDir, BackupDirName), nil
}


// ensureBackupRoot 確保備份根目錄存在
func ensureBackupRoot() (string, error) {
	rootPath, err := GetBackupRootPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(rootPath, 0755); err != nil {
		return "", err
	}
	return rootPath, nil
}

// GetBackupPath 取得指定備份的完整路徑
func GetBackupPath(name string) (string, error) {
	if name == "" {
		return "", ErrInvalidBackupName
	}
	rootPath, err := GetBackupRootPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(rootPath, name), nil
}

// BackupExists 檢查指定名稱的備份是否存在
func BackupExists(name string) bool {
	backupPath, err := GetBackupPath(name)
	if err != nil {
		return false
	}
	info, err := os.Stat(backupPath)
	return err == nil && info.IsDir()
}

// ListBackups 列出所有備份
func ListBackups() ([]BackupInfo, error) {
	rootPath, err := GetBackupRootPath()
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(rootPath); os.IsNotExist(err) {
		return []BackupInfo{}, nil
	}

	entries, err := os.ReadDir(rootPath)
	if err != nil {
		return nil, err
	}

	var backups []BackupInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		backupPath := filepath.Join(rootPath, entry.Name())
		info := BackupInfo{
			Name: entry.Name(),
			Path: backupPath,
		}

		// 檢查是否有 token 檔案
		tokenPath := filepath.Join(backupPath, KiroAuthTokenFile)
		if _, err := os.Stat(tokenPath); err == nil {
			info.HasToken = true
		}

		// 檢查是否有 machine-id 檔案並讀取備份時間
		machineIDPath := filepath.Join(backupPath, MachineIDFileName)
		if data, err := os.ReadFile(machineIDPath); err == nil {
			info.HasMachineID = true
			var mid MachineIDBackup
			if json.Unmarshal(data, &mid) == nil && mid.BackupTime != "" {
				if t, err := time.Parse(time.RFC3339, mid.BackupTime); err == nil {
					info.BackupTime = t
				}
			}
		}

		backups = append(backups, info)
	}

	return backups, nil
}


// CreateBackup 創建一個新的備份
func CreateBackup(name string) error {
	if name == "" {
		return ErrInvalidBackupName
	}

	if BackupExists(name) {
		return ErrBackupExists
	}

	// 確保備份根目錄存在
	_, err := ensureBackupRoot()
	if err != nil {
		return fmt.Errorf("failed to create backup root: %w", err)
	}

	// 創建備份資料夾
	backupPath, err := GetBackupPath(name)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(backupPath, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// 備份 kiro-auth-token.json
	tokenSrcPath, err := awssso.GetKiroAuthTokenPath()
	if err != nil {
		// 清理已創建的資料夾
		os.RemoveAll(backupPath)
		return fmt.Errorf("failed to get token path: %w", err)
	}

	if _, err := os.Stat(tokenSrcPath); os.IsNotExist(err) {
		os.RemoveAll(backupPath)
		return ErrNoTokenToBackup
	}

	tokenDstPath := filepath.Join(backupPath, KiroAuthTokenFile)
	if err := copyFile(tokenSrcPath, tokenDstPath); err != nil {
		os.RemoveAll(backupPath)
		return fmt.Errorf("failed to backup token: %w", err)
	}

	// 備份 Machine ID
	rawMachineID, err := machineid.GetRawMachineId()
	if err != nil {
		os.RemoveAll(backupPath)
		return fmt.Errorf("failed to get machine id: %w", err)
	}

	machineIDBackup := MachineIDBackup{
		MachineID:  rawMachineID,
		BackupTime: time.Now().Format(time.RFC3339),
	}

	machineIDData, err := json.MarshalIndent(machineIDBackup, "", "  ")
	if err != nil {
		os.RemoveAll(backupPath)
		return fmt.Errorf("failed to marshal machine id: %w", err)
	}

	machineIDPath := filepath.Join(backupPath, MachineIDFileName)
	if err := os.WriteFile(machineIDPath, machineIDData, 0644); err != nil {
		os.RemoveAll(backupPath)
		return fmt.Errorf("failed to write machine id: %w", err)
	}

	return nil
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


// RestoreBackup 恢復指定的備份
func RestoreBackup(name string) error {
	if name == "" {
		return ErrInvalidBackupName
	}

	if !BackupExists(name) {
		return ErrBackupNotFound
	}

	backupPath, err := GetBackupPath(name)
	if err != nil {
		return err
	}

	// 恢復 kiro-auth-token.json
	tokenSrcPath := filepath.Join(backupPath, KiroAuthTokenFile)
	if _, err := os.Stat(tokenSrcPath); os.IsNotExist(err) {
		return fmt.Errorf("backup token file not found")
	}

	tokenDstPath, err := awssso.GetKiroAuthTokenPath()
	if err != nil {
		return fmt.Errorf("failed to get token destination path: %w", err)
	}

	// 確保目標目錄存在
	tokenDstDir := filepath.Dir(tokenDstPath)
	if err := os.MkdirAll(tokenDstDir, 0755); err != nil {
		return fmt.Errorf("failed to create token directory: %w", err)
	}

	if err := copyFile(tokenSrcPath, tokenDstPath); err != nil {
		return fmt.Errorf("failed to restore token: %w", err)
	}

	return nil
}

// DeleteBackup 刪除指定的備份
func DeleteBackup(name string) error {
	if name == "" {
		return ErrInvalidBackupName
	}

	if !BackupExists(name) {
		return ErrBackupNotFound
	}

	backupPath, err := GetBackupPath(name)
	if err != nil {
		return err
	}

	return os.RemoveAll(backupPath)
}

// GetBackupInfo 取得指定備份的詳細資訊
func GetBackupInfo(name string) (*BackupInfo, error) {
	if name == "" {
		return nil, ErrInvalidBackupName
	}

	if !BackupExists(name) {
		return nil, ErrBackupNotFound
	}

	backupPath, err := GetBackupPath(name)
	if err != nil {
		return nil, err
	}

	info := &BackupInfo{
		Name: name,
		Path: backupPath,
	}

	// 檢查 token 檔案
	tokenPath := filepath.Join(backupPath, KiroAuthTokenFile)
	if _, err := os.Stat(tokenPath); err == nil {
		info.HasToken = true
	}

	// 檢查 machine-id 檔案
	machineIDPath := filepath.Join(backupPath, MachineIDFileName)
	if data, err := os.ReadFile(machineIDPath); err == nil {
		info.HasMachineID = true
		var mid MachineIDBackup
		if json.Unmarshal(data, &mid) == nil && mid.BackupTime != "" {
			if t, err := time.Parse(time.RFC3339, mid.BackupTime); err == nil {
				info.BackupTime = t
			}
		}
	}

	return info, nil
}

// ReadBackupMachineID 讀取備份中的 Machine ID
func ReadBackupMachineID(name string) (*MachineIDBackup, error) {
	if name == "" {
		return nil, ErrInvalidBackupName
	}

	if !BackupExists(name) {
		return nil, ErrBackupNotFound
	}

	backupPath, err := GetBackupPath(name)
	if err != nil {
		return nil, err
	}

	machineIDPath := filepath.Join(backupPath, MachineIDFileName)
	data, err := os.ReadFile(machineIDPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read machine id file: %w", err)
	}

	var mid MachineIDBackup
	if err := json.Unmarshal(data, &mid); err != nil {
		return nil, fmt.Errorf("failed to parse machine id file: %w", err)
	}

	return &mid, nil
}

// OriginalBackupName 原始備份的固定名稱
const OriginalBackupName = "original"

// CreateMachineIDOnlyBackup 僅備份 Machine ID（不備份 token）
// 用於軟體啟動時確保原始 Machine ID 被保存
func CreateMachineIDOnlyBackup(name string) error {
	if name == "" {
		return ErrInvalidBackupName
	}

	if BackupExists(name) {
		return ErrBackupExists
	}

	// 確保備份根目錄存在
	_, err := ensureBackupRoot()
	if err != nil {
		return fmt.Errorf("failed to create backup root: %w", err)
	}

	// 創建備份資料夾
	backupPath, err := GetBackupPath(name)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(backupPath, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// 僅備份 Machine ID
	rawMachineID, err := machineid.GetRawMachineId()
	if err != nil {
		os.RemoveAll(backupPath)
		return fmt.Errorf("failed to get machine id: %w", err)
	}

	machineIDBackup := MachineIDBackup{
		MachineID:  rawMachineID,
		BackupTime: time.Now().Format(time.RFC3339),
	}

	machineIDData, err := json.MarshalIndent(machineIDBackup, "", "  ")
	if err != nil {
		os.RemoveAll(backupPath)
		return fmt.Errorf("failed to marshal machine id: %w", err)
	}

	machineIDPath := filepath.Join(backupPath, MachineIDFileName)
	if err := os.WriteFile(machineIDPath, machineIDData, 0644); err != nil {
		os.RemoveAll(backupPath)
		return fmt.Errorf("failed to write machine id: %w", err)
	}

	return nil
}

// EnsureOriginalBackup 確保原始 Machine ID 已備份
// 如果名為 "original" 的備份不存在，則自動創建
// 回傳 (true, nil) 表示新建了備份
// 回傳 (false, nil) 表示備份已存在，無需操作
func EnsureOriginalBackup() (bool, error) {
	if BackupExists(OriginalBackupName) {
		return false, nil
	}

	// 使用僅備份 Machine ID 的方式，不強制要求 token
	if err := CreateMachineIDOnlyBackup(OriginalBackupName); err != nil {
		return false, fmt.Errorf("failed to create original backup: %w", err)
	}

	return true, nil
}

// ReadBackupToken 讀取備份中的 kiro-auth-token.json
func ReadBackupToken(name string) (*awssso.KiroAuthToken, error) {
	if name == "" {
		return nil, ErrInvalidBackupName
	}

	if !BackupExists(name) {
		return nil, ErrBackupNotFound
	}

	backupPath, err := GetBackupPath(name)
	if err != nil {
		return nil, err
	}

	tokenPath := filepath.Join(backupPath, KiroAuthTokenFile)
	data, err := os.ReadFile(tokenPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read token file: %w", err)
	}

	var token awssso.KiroAuthToken
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("failed to parse token file: %w", err)
	}

	return &token, nil
}
