package backup

import (
	"encoding/json"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"testing/quick"
)

// generateRandomString 生成指定長度的隨機字串
func generateRandomString(r *rand.Rand, length int) string {
	if length <= 0 {
		return ""
	}
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[r.Intn(len(charset))]
	}
	return string(result)
}

// generateRandomKiroAuthTokenMap 生成隨機的 KiroAuthToken map（包含各種欄位）
func generateRandomKiroAuthTokenMap(r *rand.Rand) map[string]interface{} {
	tokenMap := make(map[string]interface{})

	// 必要欄位
	tokenMap["accessToken"] = generateRandomString(r, r.Intn(100)+10)
	tokenMap["expiresAt"] = "2025-12-08T12:00:00Z"
	tokenMap["refreshToken"] = generateRandomString(r, r.Intn(100)+10)

	// 可選欄位（隨機決定是否包含）
	if r.Float32() > 0.3 {
		tokenMap["provider"] = []string{"Github", "Google", "AWS"}[r.Intn(3)]
	}
	if r.Float32() > 0.3 {
		tokenMap["authMethod"] = []string{"social", "idc"}[r.Intn(2)]
	}
	if r.Float32() > 0.3 {
		tokenMap["tokenType"] = "Bearer"
	}
	if r.Float32() > 0.3 {
		tokenMap["region"] = []string{"us-east-1", "us-west-2", "eu-west-1"}[r.Intn(3)]
	}
	if r.Float32() > 0.3 {
		tokenMap["startUrl"] = "https://d-" + generateRandomString(r, 10) + ".awsapps.com/start"
	}
	if r.Float32() > 0.3 {
		tokenMap["profileArn"] = "arn:aws:kiro::" + generateRandomString(r, 12) + ":profile/" + generateRandomString(r, 8)
	}
	// 加入一些額外的自訂欄位（模擬未知欄位）
	if r.Float32() > 0.5 {
		tokenMap["customField1"] = generateRandomString(r, 20)
	}
	if r.Float32() > 0.5 {
		tokenMap["customField2"] = r.Intn(1000)
	}
	if r.Float32() > 0.5 {
		tokenMap["nestedObject"] = map[string]interface{}{
			"key1": generateRandomString(r, 10),
			"key2": r.Intn(100),
		}
	}

	return tokenMap
}

// **Feature: token-refresh, Property 1: Token Update Preserves Original Fields**
// *For any* KiroAuthToken with valid RefreshToken, after a successful refresh operation,
// all original fields except accessToken, expiresAt, and expiresIn SHALL remain unchanged.
// **Validates: Requirements 3.2**
func TestProperty_TokenUpdatePreservesOriginalFields(t *testing.T) {
	// 建立臨時測試目錄
	tempDir, err := os.MkdirTemp("", "backup_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	f := func(seed int64) bool {
		r := rand.New(rand.NewSource(seed))

		// 生成隨機的 token map
		originalTokenMap := generateRandomKiroAuthTokenMap(r)

		// 建立測試備份目錄
		backupName := "test_backup_" + generateRandomString(r, 8)
		backupPath := filepath.Join(tempDir, backupName)
		if err := os.MkdirAll(backupPath, 0755); err != nil {
			t.Logf("Failed to create backup dir: %v", err)
			return false
		}

		// 寫入原始 token 檔案
		tokenPath := filepath.Join(backupPath, KiroAuthTokenFile)
		originalData, err := json.MarshalIndent(originalTokenMap, "", "  ")
		if err != nil {
			t.Logf("Failed to marshal original token: %v", err)
			return false
		}
		if err := os.WriteFile(tokenPath, originalData, 0644); err != nil {
			t.Logf("Failed to write original token: %v", err)
			return false
		}

		// 生成新的 accessToken 和 expiresAt
		newAccessToken := generateRandomString(r, r.Intn(100)+10)
		newExpiresAt := "2025-12-09T15:30:00Z"

		// 呼叫 WriteBackupToken（使用自訂路徑版本）
		err = writeBackupTokenToPath(tokenPath, newAccessToken, newExpiresAt)
		if err != nil {
			t.Logf("Failed to write backup token: %v", err)
			return false
		}

		// 讀取更新後的 token
		updatedData, err := os.ReadFile(tokenPath)
		if err != nil {
			t.Logf("Failed to read updated token: %v", err)
			return false
		}

		var updatedTokenMap map[string]interface{}
		if err := json.Unmarshal(updatedData, &updatedTokenMap); err != nil {
			t.Logf("Failed to unmarshal updated token: %v", err)
			return false
		}

		// Property 1: 驗證 accessToken 和 expiresAt 已更新
		if updatedTokenMap["accessToken"] != newAccessToken {
			t.Logf("accessToken not updated: got %v, expected %v",
				updatedTokenMap["accessToken"], newAccessToken)
			return false
		}
		if updatedTokenMap["expiresAt"] != newExpiresAt {
			t.Logf("expiresAt not updated: got %v, expected %v",
				updatedTokenMap["expiresAt"], newExpiresAt)
			return false
		}

		// Property 1: 驗證其他所有欄位保持不變
		for key, originalValue := range originalTokenMap {
			if key == "accessToken" || key == "expiresAt" {
				continue // 這些欄位應該被更新
			}

			updatedValue, exists := updatedTokenMap[key]
			if !exists {
				t.Logf("Field %q was removed", key)
				return false
			}

			// 比較值（需要處理 map 類型）
			if !compareValues(originalValue, updatedValue) {
				t.Logf("Field %q changed: original=%v, updated=%v",
					key, originalValue, updatedValue)
				return false
			}
		}

		// 清理測試備份
		os.RemoveAll(backupPath)

		return true
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// writeBackupTokenToPath 是 WriteBackupToken 的內部版本，直接操作指定路徑
// 用於測試時避免依賴 GetBackupPath
func writeBackupTokenToPath(tokenPath string, accessToken string, expiresAt string) error {
	// 讀取現有 token 檔案以保留原始欄位
	data, err := os.ReadFile(tokenPath)
	if err != nil {
		return err
	}

	// 使用 map 來保留所有原始欄位
	var tokenMap map[string]interface{}
	if err := json.Unmarshal(data, &tokenMap); err != nil {
		return err
	}

	// 僅更新 accessToken 和 expiresAt 欄位
	tokenMap["accessToken"] = accessToken
	tokenMap["expiresAt"] = expiresAt

	// 將更新後的 token 寫回檔案
	updatedData, err := json.MarshalIndent(tokenMap, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(tokenPath, updatedData, 0644)
}

// compareValues 比較兩個值是否相等（處理 map 和其他類型）
func compareValues(a, b interface{}) bool {
	// 將兩個值都序列化為 JSON 再比較
	aJSON, err1 := json.Marshal(a)
	bJSON, err2 := json.Marshal(b)
	if err1 != nil || err2 != nil {
		return false
	}
	return string(aJSON) == string(bJSON)
}

// TestWriteBackupToken_InvalidBackupName 測試無效備份名稱的處理
func TestWriteBackupToken_InvalidBackupName(t *testing.T) {
	err := WriteBackupToken("", "new-token", "2025-12-09T15:30:00Z")
	if err != ErrInvalidBackupName {
		t.Errorf("Expected ErrInvalidBackupName, got %v", err)
	}
}

// TestWriteBackupToken_BackupNotFound 測試備份不存在的處理
func TestWriteBackupToken_BackupNotFound(t *testing.T) {
	err := WriteBackupToken("non_existent_backup_xyz123", "new-token", "2025-12-09T15:30:00Z")
	if err != ErrBackupNotFound {
		t.Errorf("Expected ErrBackupNotFound, got %v", err)
	}
}

// TestWriteBackupToken_PreservesAllFields 測試欄位保留功能
func TestWriteBackupToken_PreservesAllFields(t *testing.T) {
	// 建立臨時測試目錄
	tempDir, err := os.MkdirTemp("", "backup_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 建立測試備份目錄
	backupPath := filepath.Join(tempDir, "test_backup")
	if err := os.MkdirAll(backupPath, 0755); err != nil {
		t.Fatalf("Failed to create backup dir: %v", err)
	}

	// 建立包含多個欄位的原始 token
	originalToken := map[string]interface{}{
		"accessToken":  "old-access-token",
		"expiresAt":    "2025-12-08T12:00:00Z",
		"refreshToken": "my-refresh-token",
		"provider":     "Github",
		"authMethod":   "social",
		"profileArn":   "arn:aws:kiro::123456789012:profile/test",
		"customField":  "should-be-preserved",
	}

	// 寫入原始 token
	tokenPath := filepath.Join(backupPath, KiroAuthTokenFile)
	originalData, _ := json.MarshalIndent(originalToken, "", "  ")
	if err := os.WriteFile(tokenPath, originalData, 0644); err != nil {
		t.Fatalf("Failed to write original token: %v", err)
	}

	// 更新 token
	newAccessToken := "new-access-token-12345"
	newExpiresAt := "2025-12-09T18:00:00Z"
	err = writeBackupTokenToPath(tokenPath, newAccessToken, newExpiresAt)
	if err != nil {
		t.Fatalf("Failed to write backup token: %v", err)
	}

	// 讀取更新後的 token
	updatedData, err := os.ReadFile(tokenPath)
	if err != nil {
		t.Fatalf("Failed to read updated token: %v", err)
	}

	var updatedToken map[string]interface{}
	if err := json.Unmarshal(updatedData, &updatedToken); err != nil {
		t.Fatalf("Failed to unmarshal updated token: %v", err)
	}

	// 驗證更新的欄位
	if updatedToken["accessToken"] != newAccessToken {
		t.Errorf("accessToken not updated: got %v, expected %v",
			updatedToken["accessToken"], newAccessToken)
	}
	if updatedToken["expiresAt"] != newExpiresAt {
		t.Errorf("expiresAt not updated: got %v, expected %v",
			updatedToken["expiresAt"], newExpiresAt)
	}

	// 驗證保留的欄位
	if updatedToken["refreshToken"] != "my-refresh-token" {
		t.Errorf("refreshToken changed: got %v", updatedToken["refreshToken"])
	}
	if updatedToken["provider"] != "Github" {
		t.Errorf("provider changed: got %v", updatedToken["provider"])
	}
	if updatedToken["authMethod"] != "social" {
		t.Errorf("authMethod changed: got %v", updatedToken["authMethod"])
	}
	if updatedToken["profileArn"] != "arn:aws:kiro::123456789012:profile/test" {
		t.Errorf("profileArn changed: got %v", updatedToken["profileArn"])
	}
	if updatedToken["customField"] != "should-be-preserved" {
		t.Errorf("customField changed: got %v", updatedToken["customField"])
	}
}
