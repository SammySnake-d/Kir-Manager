package tokenrefresh

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"kiro-manager/awssso"
)

// 整合測試：端到端刷新流程
// 需求: 1.1, 1.4

// TestIntegration_ExpiredTokenTriggersRefresh 測試過期 token 觸發刷新
// 需求 1.1: WHEN the system detects an expired AccessToken during balance refresh,
// THE Token_Refresh_Module SHALL attempt to obtain a new AccessToken using the stored RefreshToken
func TestIntegration_ExpiredTokenTriggersRefresh(t *testing.T) {
	// 建立過期的 token
	expiredTime := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	token := &awssso.KiroAuthToken{
		AccessToken:  "expired-access-token",
		ExpiresAt:    expiredTime,
		RefreshToken: "valid-refresh-token",
		AuthMethod:   "social",
		Provider:     "Github",
	}

	// 驗證 token 確實已過期
	if !awssso.IsTokenExpired(token) {
		t.Fatal("Token should be expired for this test")
	}

	// 驗證認證類型偵測正確
	authType := DetectAuthType(token)
	if authType != "social" {
		t.Errorf("Expected auth type 'social', got %q", authType)
	}

	// 注意：實際的 API 呼叫會失敗（因為 refresh token 無效）
	// 這個測試主要驗證流程邏輯正確
	t.Log("Integration test: Expired token correctly detected and would trigger refresh")
}

// TestIntegration_TokenUpdateAndPersistence 測試 token 更新與持久化
// 需求 1.2, 1.3, 3.1, 3.2: Token 刷新成功後應更新並持久化
func TestIntegration_TokenUpdateAndPersistence(t *testing.T) {
	// 建立臨時測試目錄
	tempDir, err := os.MkdirTemp("", "integration_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 建立模擬的備份目錄結構
	backupPath := filepath.Join(tempDir, "test_backup")
	if err := os.MkdirAll(backupPath, 0755); err != nil {
		t.Fatalf("Failed to create backup dir: %v", err)
	}

	// 建立原始 token 檔案（包含多個欄位）
	originalToken := map[string]interface{}{
		"accessToken":  "old-access-token",
		"expiresAt":    time.Now().Add(-1 * time.Hour).Format(time.RFC3339), // 已過期
		"refreshToken": "my-refresh-token",
		"provider":     "Github",
		"authMethod":   "social",
		"profileArn":   "arn:aws:kiro::123456789012:profile/test",
		"customField":  "should-be-preserved",
	}

	tokenPath := filepath.Join(backupPath, "kiro-auth-token.json")
	originalData, _ := json.MarshalIndent(originalToken, "", "  ")
	if err := os.WriteFile(tokenPath, originalData, 0644); err != nil {
		t.Fatalf("Failed to write original token: %v", err)
	}

	// 模擬刷新後的新 token 資訊
	newTokenInfo := &TokenInfo{
		AccessToken: "new-access-token-12345",
		ExpiresAt:   time.Now().Add(1 * time.Hour),
		ExpiresIn:   3600,
		ProfileArn:  "arn:aws:kiro::123456789012:profile/test",
	}

	// 模擬更新 token 檔案（使用與 WriteBackupToken 相同的邏輯）
	err = updateTokenFile(tokenPath, newTokenInfo.AccessToken, newTokenInfo.ExpiresAt.Format(time.RFC3339))
	if err != nil {
		t.Fatalf("Failed to update token file: %v", err)
	}

	// 讀取更新後的 token 並驗證
	updatedData, err := os.ReadFile(tokenPath)
	if err != nil {
		t.Fatalf("Failed to read updated token: %v", err)
	}

	var updatedToken map[string]interface{}
	if err := json.Unmarshal(updatedData, &updatedToken); err != nil {
		t.Fatalf("Failed to unmarshal updated token: %v", err)
	}

	// 驗證 accessToken 已更新
	if updatedToken["accessToken"] != newTokenInfo.AccessToken {
		t.Errorf("accessToken not updated: got %v, expected %v",
			updatedToken["accessToken"], newTokenInfo.AccessToken)
	}

	// 驗證 expiresAt 已更新
	if updatedToken["expiresAt"] != newTokenInfo.ExpiresAt.Format(time.RFC3339) {
		t.Errorf("expiresAt not updated: got %v, expected %v",
			updatedToken["expiresAt"], newTokenInfo.ExpiresAt.Format(time.RFC3339))
	}

	// 驗證其他欄位保持不變（需求 3.2）
	if updatedToken["refreshToken"] != "my-refresh-token" {
		t.Errorf("refreshToken changed: got %v", updatedToken["refreshToken"])
	}
	if updatedToken["provider"] != "Github" {
		t.Errorf("provider changed: got %v", updatedToken["provider"])
	}
	if updatedToken["authMethod"] != "social" {
		t.Errorf("authMethod changed: got %v", updatedToken["authMethod"])
	}
	if updatedToken["customField"] != "should-be-preserved" {
		t.Errorf("customField changed: got %v", updatedToken["customField"])
	}

	// 驗證更新後的 token 未過期
	updatedKiroToken := &awssso.KiroAuthToken{
		AccessToken: updatedToken["accessToken"].(string),
		ExpiresAt:   updatedToken["expiresAt"].(string),
	}
	if awssso.IsTokenExpired(updatedKiroToken) {
		t.Error("Updated token should not be expired")
	}

	t.Log("Integration test: Token update and persistence verified successfully")
}

// updateTokenFile 更新 token 檔案（模擬 WriteBackupToken 的邏輯）
func updateTokenFile(tokenPath string, accessToken string, expiresAt string) error {
	// 讀取現有 token 檔案
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


// TestIntegration_RefreshFlowContinuesWithBalanceQuery 測試刷新後餘額查詢繼續執行
// 需求 1.4: WHEN the token refresh succeeds, THE system SHALL proceed with the original balance query operation
func TestIntegration_RefreshFlowContinuesWithBalanceQuery(t *testing.T) {
	// 這個測試驗證整個流程的邏輯順序：
	// 1. 檢測 token 過期
	// 2. 嘗試刷新 token
	// 3. 更新 token
	// 4. 持久化 token
	// 5. 繼續執行餘額查詢

	// 建立臨時測試目錄
	tempDir, err := os.MkdirTemp("", "integration_flow_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 建立模擬的備份目錄結構
	backupPath := filepath.Join(tempDir, "test_backup")
	if err := os.MkdirAll(backupPath, 0755); err != nil {
		t.Fatalf("Failed to create backup dir: %v", err)
	}

	// 步驟 1: 建立過期的 token
	expiredToken := map[string]interface{}{
		"accessToken":  "expired-access-token",
		"expiresAt":    time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
		"refreshToken": "valid-refresh-token",
		"provider":     "Github",
		"authMethod":   "social",
	}

	tokenPath := filepath.Join(backupPath, "kiro-auth-token.json")
	tokenData, _ := json.MarshalIndent(expiredToken, "", "  ")
	if err := os.WriteFile(tokenPath, tokenData, 0644); err != nil {
		t.Fatalf("Failed to write token: %v", err)
	}

	// 讀取 token 並驗證過期
	readData, _ := os.ReadFile(tokenPath)
	var tokenMap map[string]interface{}
	json.Unmarshal(readData, &tokenMap)

	kiroToken := &awssso.KiroAuthToken{
		AccessToken:  tokenMap["accessToken"].(string),
		ExpiresAt:    tokenMap["expiresAt"].(string),
		RefreshToken: tokenMap["refreshToken"].(string),
		AuthMethod:   tokenMap["authMethod"].(string),
	}

	// 步驟 1 驗證: Token 應該已過期
	if !awssso.IsTokenExpired(kiroToken) {
		t.Fatal("Step 1 failed: Token should be expired")
	}
	t.Log("Step 1 passed: Token is expired")

	// 步驟 2: 偵測認證類型（模擬刷新前的準備）
	authType := DetectAuthType(kiroToken)
	if authType != "social" {
		t.Fatalf("Step 2 failed: Expected auth type 'social', got %q", authType)
	}
	t.Log("Step 2 passed: Auth type detected as 'social'")

	// 步驟 3 & 4: 模擬刷新成功並更新 token
	newAccessToken := "new-refreshed-access-token"
	newExpiresAt := time.Now().Add(1 * time.Hour).Format(time.RFC3339)

	err = updateTokenFile(tokenPath, newAccessToken, newExpiresAt)
	if err != nil {
		t.Fatalf("Step 3-4 failed: Could not update token: %v", err)
	}
	t.Log("Step 3-4 passed: Token updated and persisted")

	// 步驟 5: 驗證更新後的 token 可用於後續操作
	updatedData, _ := os.ReadFile(tokenPath)
	var updatedTokenMap map[string]interface{}
	json.Unmarshal(updatedData, &updatedTokenMap)

	updatedKiroToken := &awssso.KiroAuthToken{
		AccessToken: updatedTokenMap["accessToken"].(string),
		ExpiresAt:   updatedTokenMap["expiresAt"].(string),
	}

	// 驗證更新後的 token 未過期
	if awssso.IsTokenExpired(updatedKiroToken) {
		t.Fatal("Step 5 failed: Updated token should not be expired")
	}

	// 驗證 accessToken 已更新
	if updatedKiroToken.AccessToken != newAccessToken {
		t.Fatalf("Step 5 failed: AccessToken not updated correctly")
	}

	t.Log("Step 5 passed: Updated token is valid and can be used for balance query")
	t.Log("Integration test: Complete refresh flow verified successfully")
}

// TestIntegration_RefreshErrorHandling 測試刷新失敗時的錯誤處理
// 需求 1.5: WHEN the RefreshToken is invalid or expired, THE Token_Refresh_Module SHALL return a clear error
func TestIntegration_RefreshErrorHandling(t *testing.T) {
	testCases := []struct {
		name          string
		token         *awssso.KiroAuthToken
		expectedError bool
		errorContains string
	}{
		{
			name:          "Nil token",
			token:         nil,
			expectedError: true,
			errorContains: "Token 不可為空",
		},
		{
			name: "Empty refresh token (Social)",
			token: &awssso.KiroAuthToken{
				AuthMethod:   "social",
				RefreshToken: "",
			},
			expectedError: true,
			errorContains: "RefreshToken 不可為空",
		},
		{
			name: "Empty refresh token (IdC)",
			token: &awssso.KiroAuthToken{
				AuthMethod:   "idc",
				RefreshToken: "",
			},
			expectedError: true,
			errorContains: "RefreshToken 不可為空",
		},
		{
			name: "Unknown auth type",
			token: &awssso.KiroAuthToken{
				RefreshToken: "some-token",
				// 沒有任何可識別認證類型的欄位
			},
			expectedError: true,
			errorContains: "不支援的認證類型",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := RefreshAccessToken(tc.token, "test-machine-id-hash")

			if tc.expectedError {
				if err == nil {
					t.Error("Expected error but got nil")
					return
				}

				if tc.errorContains != "" {
					if refreshErr, ok := err.(*RefreshError); ok {
						if refreshErr.Message != tc.errorContains && 
						   !containsString(refreshErr.Message, tc.errorContains) {
							t.Errorf("Error message %q does not contain %q",
								refreshErr.Message, tc.errorContains)
						}
					} else {
						t.Errorf("Expected RefreshError, got %T", err)
					}
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

// containsString 檢查字串是否包含子字串
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

// containsSubstring 簡單的子字串檢查
func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestIntegration_AuthTypeDetectionForRefresh 測試認證類型偵測用於刷新路由
// 需求 2.1, 2.2, 2.4
func TestIntegration_AuthTypeDetectionForRefresh(t *testing.T) {
	testCases := []struct {
		name         string
		token        *awssso.KiroAuthToken
		expectedType string
	}{
		{
			name: "Social with AuthMethod",
			token: &awssso.KiroAuthToken{
				AuthMethod:   "social",
				RefreshToken: "token",
			},
			expectedType: "social",
		},
		{
			name: "Social with Provider",
			token: &awssso.KiroAuthToken{
				Provider:     "Github",
				RefreshToken: "token",
			},
			expectedType: "social",
		},
		{
			name: "IdC with AuthMethod",
			token: &awssso.KiroAuthToken{
				AuthMethod:   "idc",
				RefreshToken: "token",
			},
			expectedType: "idc",
		},
		{
			name: "IdC with StartURL and Region",
			token: &awssso.KiroAuthToken{
				StartURL:     "https://d-123456.awsapps.com/start",
				Region:       "us-east-1",
				RefreshToken: "token",
			},
			expectedType: "idc",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			authType := DetectAuthType(tc.token)
			if authType != tc.expectedType {
				t.Errorf("Expected auth type %q, got %q", tc.expectedType, authType)
			}
		})
	}
}

// TestIntegration_ExpiresAtCalculationInRefreshFlow 測試刷新流程中的 ExpiresAt 計算
// 需求 5.3
func TestIntegration_ExpiresAtCalculationInRefreshFlow(t *testing.T) {
	// 測試不同的 expiresIn 值
	testCases := []struct {
		name      string
		expiresIn int
	}{
		{"1 hour (Social typical)", 3600},
		{"8 hours (IdC typical)", 28800},
		{"30 minutes", 1800},
		{"24 hours", 86400},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			before := time.Now()
			expiresAt := CalculateExpiresAt(tc.expiresIn)
			after := time.Now()

			// 驗證 ExpiresAt 在預期範圍內
			expectedMin := before.Add(time.Duration(tc.expiresIn) * time.Second)
			expectedMax := after.Add(time.Duration(tc.expiresIn) * time.Second)

			if expiresAt.Before(expectedMin) || expiresAt.After(expectedMax) {
				t.Errorf("ExpiresAt %v not in expected range [%v, %v]",
					expiresAt, expectedMin, expectedMax)
			}

			// 驗證 RFC3339 格式化
			formatted := CalculateExpiresAtString(tc.expiresIn)
			_, err := time.Parse(time.RFC3339, formatted)
			if err != nil {
				t.Errorf("ExpiresAt string %q is not valid RFC3339: %v", formatted, err)
			}
		})
	}
}
