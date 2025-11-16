package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func init() {
	// 设置测试用的 JWT Secret
	SetJWTSecret("test-secret-key-for-refresh-token-testing")
}

// TestGenerateTokenPair 测试生成 Token 对
func TestGenerateTokenPair(t *testing.T) {
	userID := "test-user-123"
	email := "test@example.com"

	tokenPair, err := GenerateTokenPair(userID, email)
	assert.NoError(t, err, "生成 Token 对应该成功")
	assert.NotNil(t, tokenPair, "Token 对不应为空")
	assert.NotEmpty(t, tokenPair.AccessToken, "Access Token 不应为空")
	assert.NotEmpty(t, tokenPair.RefreshToken, "Refresh Token 不应为空")
	assert.Equal(t, int64(604800), tokenPair.ExpiresIn, "Access Token 过期时间应为 7 天（604800 秒）")
	assert.Equal(t, int64(2592000), tokenPair.RefreshExpiresIn, "Refresh Token 过期时间应为 30 天（2592000 秒）")
}

// TestValidateRefreshToken 测试验证 Refresh Token
func TestValidateRefreshToken(t *testing.T) {
	userID := "test-user-456"
	email := "refresh@example.com"

	// 生成 Token 对
	tokenPair, err := GenerateTokenPair(userID, email)
	assert.NoError(t, err)

	// 验证 Refresh Token
	claims, err := ValidateRefreshToken(tokenPair.RefreshToken)
	assert.NoError(t, err, "验证 Refresh Token 应该成功")
	assert.NotNil(t, claims, "Claims 不应为空")
	assert.Equal(t, userID, claims.UserID, "UserID 应该匹配")
	assert.Equal(t, email, claims.Email, "Email 应该匹配")
	assert.Equal(t, "refresh", claims.TokenType, "Token 类型应该是 refresh")
}

// TestValidateRefreshToken_InvalidToken 测试验证无效的 Refresh Token
func TestValidateRefreshToken_InvalidToken(t *testing.T) {
	invalidToken := "invalid.token.string"

	claims, err := ValidateRefreshToken(invalidToken)
	assert.Error(t, err, "验证无效 Token 应该失败")
	assert.Nil(t, claims, "Claims 应该为空")
}

// TestRefreshAccessToken 测试刷新 Access Token
func TestRefreshAccessToken(t *testing.T) {
	userID := "test-user-789"
	email := "refresh-test@example.com"

	// 生成初始 Token 对
	initialTokenPair, err := GenerateTokenPair(userID, email)
	assert.NoError(t, err)

	// 使用 Refresh Token 刷新
	newTokenPair, err := RefreshAccessToken(initialTokenPair.RefreshToken)
	assert.NoError(t, err, "刷新 Access Token 应该成功")
	assert.NotNil(t, newTokenPair, "新 Token 对不应为空")
	assert.NotEqual(t, initialTokenPair.AccessToken, newTokenPair.AccessToken, "新 Access Token 应该与旧的不同")
	assert.NotEqual(t, initialTokenPair.RefreshToken, newTokenPair.RefreshToken, "新 Refresh Token 应该与旧的不同")

	// 验证新的 Access Token
	accessClaims, err := ValidateJWT(newTokenPair.AccessToken)
	assert.NoError(t, err)
	assert.Equal(t, userID, accessClaims.UserID)
	assert.Equal(t, email, accessClaims.Email)
}

// TestRefreshTokenRotation 测试 Refresh Token 轮换
func TestRefreshTokenRotation(t *testing.T) {
	userID := "test-user-rotation"
	email := "rotation@example.com"

	// 生成初始 Token 对
	tokenPair, err := GenerateTokenPair(userID, email)
	assert.NoError(t, err)

	// 第一次刷新
	newTokenPair, err := RefreshAccessToken(tokenPair.RefreshToken)
	assert.NoError(t, err)

	// 尝试再次使用旧的 Refresh Token（应该失败，因为已被撤销）
	_, err = RefreshAccessToken(tokenPair.RefreshToken)
	assert.Error(t, err, "使用已撤销的 Refresh Token 应该失败")
	assert.Contains(t, err.Error(), "已被撤销", "错误消息应该包含 '已被撤销'")

	// 使用新的 Refresh Token 应该成功
	finalTokenPair, err := RefreshAccessToken(newTokenPair.RefreshToken)
	assert.NoError(t, err)
	assert.NotNil(t, finalTokenPair)
}

// TestBlacklistRefreshToken 测试 Refresh Token 黑名单
func TestBlacklistRefreshToken(t *testing.T) {
	userID := "test-user-blacklist"
	email := "blacklist@example.com"

	tokenPair, err := GenerateTokenPair(userID, email)
	assert.NoError(t, err)

	// 验证 Refresh Token（应该有效）
	claims, err := ValidateRefreshToken(tokenPair.RefreshToken)
	assert.NoError(t, err)
	assert.NotNil(t, claims)

	// 手动将 Refresh Token 加入黑名单
	BlacklistRefreshToken(tokenPair.RefreshToken, time.Now().Add(7*24*time.Hour))

	// 再次验证（应该失败）
	claims, err = ValidateRefreshToken(tokenPair.RefreshToken)
	assert.Error(t, err)
	assert.Nil(t, claims)
	assert.Contains(t, err.Error(), "已被撤销")
}

// TestIsRefreshTokenBlacklisted 测试黑名单检查
func TestIsRefreshTokenBlacklisted(t *testing.T) {
	token := "test-token-12345"

	// 初始状态：不在黑名单中
	assert.False(t, IsRefreshTokenBlacklisted(token), "初始状态应该不在黑名单中")

	// 加入黑名单
	BlacklistRefreshToken(token, time.Now().Add(1*time.Hour))

	// 应该在黑名单中
	assert.True(t, IsRefreshTokenBlacklisted(token), "应该在黑名单中")
}

// TestRefreshTokenExpiration 测试过期的 Refresh Token
func TestRefreshTokenExpiration(t *testing.T) {
	// 注意：这个测试无法真实测试过期，因为需要等待 7 天
	// 但我们可以测试黑名单的过期清理逻辑

	token := "expired-token"
	pastTime := time.Now().Add(-1 * time.Hour) // 1 小时前过期

	// 加入黑名单（已过期）
	BlacklistRefreshToken(token, pastTime)

	// 检查时应该自动清理并返回 false
	assert.False(t, IsRefreshTokenBlacklisted(token), "过期的黑名单条目应该被自动清理")
}

// TestAccessTokenAndRefreshTokenDifferent 测试 Access Token 和 Refresh Token 不同
func TestAccessTokenAndRefreshTokenDifferent(t *testing.T) {
	userID := "test-user-different"
	email := "different@example.com"

	tokenPair, err := GenerateTokenPair(userID, email)
	assert.NoError(t, err)

	// Access Token 和 Refresh Token 应该不同
	assert.NotEqual(t, tokenPair.AccessToken, tokenPair.RefreshToken, "Access Token 和 Refresh Token 应该不同")

	// Access Token 应该无法作为 Refresh Token 使用
	_, err = ValidateRefreshToken(tokenPair.AccessToken)
	assert.Error(t, err, "Access Token 不应该能作为 Refresh Token 使用")
}

// TestTokenPairUserInfo 测试 Token 对中的用户信息一致性
func TestTokenPairUserInfo(t *testing.T) {
	userID := "test-user-consistency"
	email := "consistency@example.com"

	tokenPair, err := GenerateTokenPair(userID, email)
	assert.NoError(t, err)

	// 验证 Access Token
	accessClaims, err := ValidateJWT(tokenPair.AccessToken)
	assert.NoError(t, err)
	assert.Equal(t, userID, accessClaims.UserID)
	assert.Equal(t, email, accessClaims.Email)

	// 验证 Refresh Token
	refreshClaims, err := ValidateRefreshToken(tokenPair.RefreshToken)
	assert.NoError(t, err)
	assert.Equal(t, userID, refreshClaims.UserID)
	assert.Equal(t, email, refreshClaims.Email)

	// 两者的用户信息应该一致
	assert.Equal(t, accessClaims.UserID, refreshClaims.UserID)
	assert.Equal(t, accessClaims.Email, refreshClaims.Email)
}

// BenchmarkGenerateTokenPair 性能基准测试
func BenchmarkGenerateTokenPair(b *testing.B) {
	SetJWTSecret("benchmark-secret-key")
	userID := "bench-user"
	email := "bench@example.com"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = GenerateTokenPair(userID, email)
	}
}

// BenchmarkValidateRefreshToken 验证性能基准测试
func BenchmarkValidateRefreshToken(b *testing.B) {
	SetJWTSecret("benchmark-secret-key")
	tokenPair, _ := GenerateTokenPair("bench-user", "bench@example.com")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ValidateRefreshToken(tokenPair.RefreshToken)
	}
}

// BenchmarkRefreshAccessToken 刷新性能基准测试
func BenchmarkRefreshAccessToken(b *testing.B) {
	SetJWTSecret("benchmark-secret-key")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		tokenPair, _ := GenerateTokenPair("bench-user", "bench@example.com")
		b.StartTimer()

		_, _ = RefreshAccessToken(tokenPair.RefreshToken)
	}
}
