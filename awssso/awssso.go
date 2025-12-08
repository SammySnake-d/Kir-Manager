package awssso

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

const (
	KiroAuthTokenFile = "kiro-auth-token.json"
)

var (
	ErrCacheNotFound = errors.New("sso cache directory not found")
	ErrTokenNotFound = errors.New("kiro auth token not found")
)

// KiroAuthToken 代表 Kiro 的認證 token 結構
type KiroAuthToken struct {
	AccessToken  string `json:"accessToken,omitempty"`
	ExpiresAt    string `json:"expiresAt,omitempty"`
	Provider     string `json:"provider,omitempty"`
	AuthMethod   string `json:"authMethod,omitempty"`
	RefreshToken string `json:"refreshToken,omitempty"`
	TokenType    string `json:"tokenType,omitempty"`
	Region       string `json:"region,omitempty"`
	StartURL     string `json:"startUrl,omitempty"`
	ProfileArn   string `json:"profileArn,omitempty"`
	ClientIdHash string `json:"clientIdHash,omitempty"` // BuilderId (IdC) 用於關聯 clientId/clientSecret 文件
}

// SSOCacheFile 代表通用的 SSO 快取檔案結構
type SSOCacheFile struct {
	AccessToken  string `json:"accessToken,omitempty"`
	ExpiresAt    string `json:"expiresAt,omitempty"`
	RefreshToken string `json:"refreshToken,omitempty"`
	TokenType    string `json:"tokenType,omitempty"`
	Region       string `json:"region,omitempty"`
	StartURL     string `json:"startUrl,omitempty"`
	ClientID     string `json:"clientId,omitempty"`
	ClientSecret string `json:"clientSecret,omitempty"`
	// 保留原始 JSON 以便存取未定義的欄位
	Raw map[string]interface{} `json:"-"`
}

// GetSSOCachePath 取得 AWS SSO 快取目錄路徑 (~/.aws/sso/cache)
func GetSSOCachePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".aws", "sso", "cache"), nil
}


// SSOCacheExists 檢查 SSO 快取目錄是否存在
func SSOCacheExists() bool {
	path, err := GetSSOCachePath()
	if err != nil {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// GetKiroAuthTokenPath 取得 Kiro 認證 token 檔案的完整路徑
func GetKiroAuthTokenPath() (string, error) {
	cachePath, err := GetSSOCachePath()
	if err != nil {
		return "", err
	}
	return filepath.Join(cachePath, KiroAuthTokenFile), nil
}

// ReadKiroAuthToken 讀取 Kiro 的認證 token
func ReadKiroAuthToken() (*KiroAuthToken, error) {
	tokenPath, err := GetKiroAuthTokenPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(tokenPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrTokenNotFound
		}
		return nil, err
	}

	var token KiroAuthToken
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, err
	}

	return &token, nil
}

// ListCacheFiles 列出 SSO 快取目錄中的所有 JSON 檔案
func ListCacheFiles() ([]string, error) {
	cachePath, err := GetSSOCachePath()
	if err != nil {
		return nil, err
	}

	if !SSOCacheExists() {
		return nil, ErrCacheNotFound
	}

	entries, err := os.ReadDir(cachePath)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			files = append(files, entry.Name())
		}
	}

	return files, nil
}

// ReadCacheFile 讀取指定的快取檔案
func ReadCacheFile(filename string) (*SSOCacheFile, error) {
	cachePath, err := GetSSOCachePath()
	if err != nil {
		return nil, err
	}

	filePath := filepath.Join(cachePath, filename)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var cache SSOCacheFile
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}

	// 同時保存原始 JSON 到 Raw 欄位
	if err := json.Unmarshal(data, &cache.Raw); err != nil {
		return nil, err
	}

	return &cache, nil
}

// ReadCacheFileRaw 讀取指定的快取檔案並返回原始 map
func ReadCacheFileRaw(filename string) (map[string]interface{}, error) {
	cachePath, err := GetSSOCachePath()
	if err != nil {
		return nil, err
	}

	filePath := filepath.Join(cachePath, filename)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	return raw, nil
}


// IsTokenExpired 檢查 token 是否已過期
func IsTokenExpired(token *KiroAuthToken) bool {
	if token == nil || token.ExpiresAt == "" {
		return true
	}

	// 解析 ISO 8601 格式的時間字串
	expiresAt, err := time.Parse(time.RFC3339, token.ExpiresAt)
	if err != nil {
		// 嘗試其他可能的格式
		expiresAt, err = time.Parse("2006-01-02T15:04:05.000Z", token.ExpiresAt)
		if err != nil {
			return true
		}
	}

	return time.Now().After(expiresAt)
}
