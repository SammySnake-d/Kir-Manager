package usage

import (
	"math"
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"

	"kiro-manager/awssso"
)

// generateUsageBreakdownList 生成隨機的 UsageBreakdown 列表
// 包含基本額度、免費試用額度和獎勵額度
func generateUsageBreakdownList(rand *rand.Rand, size int) []UsageBreakdown {
	count := rand.Intn(10) + 1 // 1-10 個項目
	list := make([]UsageBreakdown, count)
	for i := 0; i < count; i++ {
		breakdown := UsageBreakdown{
			UsageLimitWithPrecision:   float64(rand.Intn(10000)),
			CurrentUsageWithPrecision: float64(rand.Intn(10000)),
			DisplayName:               "test",
		}

		// 隨機決定是否有 FreeTrialInfo
		if rand.Intn(2) == 1 {
			breakdown.FreeTrialInfo = &FreeTrialInfo{
				UsageLimitWithPrecision:   float64(rand.Intn(1000)),
				CurrentUsageWithPrecision: float64(rand.Intn(1000)),
				FreeTrialStatus:           "ACTIVE",
			}
		}

		// 隨機決定是否有 Bonuses
		bonusCount := rand.Intn(3) // 0-2 個獎勵
		if bonusCount > 0 {
			breakdown.Bonuses = make([]Bonus, bonusCount)
			for j := 0; j < bonusCount; j++ {
				breakdown.Bonuses[j] = Bonus{
					BonusCode:    "test-bonus",
					UsageLimit:   float64(rand.Intn(2000)),
					CurrentUsage: float64(rand.Intn(2000)),
					Status:       "ACTIVE",
				}
			}
		}

		list[i] = breakdown
	}
	return list
}

// **Feature: backup-usage-display, Property 1: Balance Calculation Correctness**
// *For any* usageBreakdownList array, the calculated balance SHALL equal
// the sum of all usageLimit values minus the sum of all currentUsage values.
// **Validates: Requirements 1.3**
func TestProperty_BalanceCalculationCorrectness(t *testing.T) {
	f := func(seed int64) bool {
		rand := rand.New(rand.NewSource(seed))
		breakdownList := generateUsageBreakdownList(rand, 0)

		response := &UsageLimitsResponse{
			SubscriptionInfo: SubscriptionInfo{
				SubscriptionTitle: "TEST",
			},
			UsageBreakdownList: breakdownList,
		}

		result := CalculateBalance(response)

		// 手動計算預期值（包含基本額度、免費試用額度和獎勵額度）
		var expectedUsageLimit float64
		var expectedCurrentUsage float64
		for _, b := range breakdownList {
			// 基本額度
			expectedUsageLimit += b.UsageLimitWithPrecision
			expectedCurrentUsage += b.CurrentUsageWithPrecision

			// 免費試用額度
			if b.FreeTrialInfo != nil {
				expectedUsageLimit += b.FreeTrialInfo.UsageLimitWithPrecision
				expectedCurrentUsage += b.FreeTrialInfo.CurrentUsageWithPrecision
			}

			// 獎勵額度
			for _, bonus := range b.Bonuses {
				expectedUsageLimit += bonus.UsageLimit
				expectedCurrentUsage += bonus.CurrentUsage
			}
		}
		expectedBalance := expectedUsageLimit - expectedCurrentUsage

		// 驗證 Property 1: Balance = Σ(usageLimit) - Σ(currentUsage)
		if math.Abs(result.Balance-expectedBalance) > 0.0001 {
			t.Logf("Balance mismatch: got %f, expected %f", result.Balance, expectedBalance)
			return false
		}

		if math.Abs(result.UsageLimit-expectedUsageLimit) > 0.0001 {
			t.Logf("UsageLimit mismatch: got %f, expected %f", result.UsageLimit, expectedUsageLimit)
			return false
		}

		if math.Abs(result.CurrentUsage-expectedCurrentUsage) > 0.0001 {
			t.Logf("CurrentUsage mismatch: got %f, expected %f", result.CurrentUsage, expectedCurrentUsage)
			return false
		}

		return true
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// **Feature: backup-usage-display, Property 2: Low Balance Detection**
// *For any* calculated balance and total usage limit, the isLowBalance flag
// SHALL be true if and only if balance is less than 20% of the total usage limit.
// **Validates: Requirements 3.2**
func TestProperty_LowBalanceDetection(t *testing.T) {
	f := func(usageLimit, currentUsage float64) bool {
		// 確保輸入值為非負數
		if usageLimit < 0 {
			usageLimit = -usageLimit
		}
		if currentUsage < 0 {
			currentUsage = -currentUsage
		}

		response := &UsageLimitsResponse{
			SubscriptionInfo: SubscriptionInfo{
				SubscriptionTitle: "TEST",
			},
			UsageBreakdownList: []UsageBreakdown{
				{
					UsageLimitWithPrecision:   usageLimit,
					CurrentUsageWithPrecision: currentUsage,
				},
			},
		}

		result := CalculateBalance(response)

		balance := usageLimit - currentUsage

		// Property 2: IsLowBalance = (Balance / TotalUsageLimit) < 0.2
		var expectedIsLowBalance bool
		if usageLimit > 0 {
			expectedIsLowBalance = (balance / usageLimit) < 0.2
		}

		if result.IsLowBalance != expectedIsLowBalance {
			t.Logf("IsLowBalance mismatch: got %v, expected %v (balance=%f, usageLimit=%f, ratio=%f)",
				result.IsLowBalance, expectedIsLowBalance, balance, usageLimit, balance/usageLimit)
			return false
		}

		return true
	}

	config := &quick.Config{
		MaxCount: 100,
		Values: func(values []reflect.Value, rand *rand.Rand) {
			// 生成合理範圍的浮點數
			values[0] = reflect.ValueOf(float64(rand.Intn(10000)))
			values[1] = reflect.ValueOf(float64(rand.Intn(10000)))
		},
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// TestCalculateBalance_NilResponse 測試 nil 輸入
func TestCalculateBalance_NilResponse(t *testing.T) {
	result := CalculateBalance(nil)
	if result == nil {
		t.Error("Expected non-nil result for nil input")
	}
	if result.Balance != 0 || result.UsageLimit != 0 || result.CurrentUsage != 0 {
		t.Error("Expected zero values for nil input")
	}
}

// TestCalculateBalance_EmptyList 測試空列表
func TestCalculateBalance_EmptyList(t *testing.T) {
	response := &UsageLimitsResponse{
		SubscriptionInfo: SubscriptionInfo{
			SubscriptionTitle: "TEST",
		},
		UsageBreakdownList: []UsageBreakdown{},
	}

	result := CalculateBalance(response)
	if result.Balance != 0 || result.UsageLimit != 0 || result.CurrentUsage != 0 {
		t.Error("Expected zero values for empty list")
	}
	if result.IsLowBalance != false {
		t.Error("Expected IsLowBalance to be false for empty list")
	}
}

// **Feature: backup-usage-display, Property 4: Error Handling Graceful Degradation**
// *For any* API request that fails or returns an error, the system SHALL return
// empty/default values without crashing.
// **Validates: Requirements 1.4**
func TestProperty_ErrorHandlingGracefulDegradation(t *testing.T) {
	// 生成隨機的 KiroAuthToken（可能有效或無效）
	generateRandomToken := func(rand *rand.Rand) *awssso.KiroAuthToken {
		// 隨機決定是否返回 nil
		if rand.Intn(4) == 0 {
			return nil
		}

		// 隨機生成 token 欄位（可能為空）
		token := &awssso.KiroAuthToken{}

		// 隨機決定是否填充 AccessToken
		if rand.Intn(2) == 1 {
			token.AccessToken = generateRandomString(rand, rand.Intn(50))
		}

		// 隨機決定是否填充 ProfileArn
		if rand.Intn(2) == 1 {
			token.ProfileArn = generateRandomString(rand, rand.Intn(50))
		}

		// 隨機填充其他欄位
		if rand.Intn(2) == 1 {
			token.Provider = generateRandomString(rand, rand.Intn(20))
		}
		if rand.Intn(2) == 1 {
			token.Region = generateRandomString(rand, rand.Intn(20))
		}

		return token
	}

	f := func(seed int64) bool {
		rand := rand.New(rand.NewSource(seed))
		token := generateRandomToken(rand)

		// 使用 defer/recover 確保不會 panic
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("GetUsageLimitsSafe panicked with token %+v: %v", token, r)
			}
		}()

		// 呼叫 GetUsageLimitsSafe - 這個函數應該永遠不會 panic
		result := GetUsageLimitsSafe(token)

		// Property 4: 必須返回非 nil 的 UsageInfo
		if result == nil {
			t.Logf("GetUsageLimitsSafe returned nil for token %+v", token)
			return false
		}

		// 對於無效的 token，應該返回空的 UsageInfo（零值）
		// 有效的 token 定義：AccessToken 和 ProfileArn 都非空
		isValidToken := token != nil && token.AccessToken != "" && token.ProfileArn != ""

		// 如果 token 無效，結果應該是空的 UsageInfo
		if !isValidToken {
			// 無效 token 應該返回零值的 UsageInfo
			if result.SubscriptionTitle != "" ||
				result.UsageLimit != 0 ||
				result.CurrentUsage != 0 ||
				result.Balance != 0 ||
				result.IsLowBalance != false {
				// 注意：這裡我們只檢查無效 token 的情況
				// 因為有效 token 會嘗試呼叫真實 API，可能會因網路問題失敗
				// 但失敗時也應該返回空的 UsageInfo
				t.Logf("Expected empty UsageInfo for invalid token %+v, got %+v", token, result)
				return false
			}
		}

		// 無論如何，函數都不應該 panic，且必須返回非 nil 結果
		return true
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// generateRandomString 生成指定長度的隨機字串
func generateRandomString(rand *rand.Rand, length int) string {
	if length <= 0 {
		return ""
	}
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[rand.Intn(len(charset))]
	}
	return string(result)
}
