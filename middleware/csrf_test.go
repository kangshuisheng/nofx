package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// TestGenerateCSRFToken 测试 CSRF Token 生成
func TestGenerateCSRFToken(t *testing.T) {
	token1, err := generateCSRFToken(32)
	assert.NoError(t, err, "生成 Token 应该成功")
	assert.NotEmpty(t, token1, "Token 不应为空")
	assert.Greater(t, len(token1), 40, "Token 长度应该足够长")

	// 生成第二个 Token，应该与第一个不同
	token2, err := generateCSRFToken(32)
	assert.NoError(t, err)
	assert.NotEqual(t, token1, token2, "每次生成的 Token 应该不同")
}

// TestCSRFMiddleware_GETRequest 测试 GET 请求（应该自动生成 Token）
func TestCSRFMiddleware_GETRequest(t *testing.T) {
	config := DefaultCSRFConfig()
	middleware := CSRFMiddleware(config)

	router := gin.New()
	router.Use(middleware)
	router.GET("/api/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/test", nil)
	router.ServeHTTP(w, req)

	// GET 请求应该成功
	assert.Equal(t, http.StatusOK, w.Code, "GET 请求应该成功")

	// 应该设置 CSRF Cookie
	cookies := w.Result().Cookies()
	assert.NotEmpty(t, cookies, "应该设置 Cookie")

	var csrfCookie *http.Cookie
	for _, cookie := range cookies {
		if cookie.Name == config.CookieName {
			csrfCookie = cookie
			break
		}
	}
	assert.NotNil(t, csrfCookie, "应该设置 CSRF Cookie")
	assert.NotEmpty(t, csrfCookie.Value, "CSRF Token 不应为空")
}

// TestCSRFMiddleware_POSTWithoutToken 测试没有 Token 的 POST 请求（应该被拒绝）
func TestCSRFMiddleware_POSTWithoutToken(t *testing.T) {
	config := DefaultCSRFConfig()
	middleware := CSRFMiddleware(config)

	router := gin.New()
	router.Use(middleware)
	router.POST("/api/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/test", nil)
	router.ServeHTTP(w, req)

	// 没有 Token 的 POST 请求应该被拒绝
	assert.Equal(t, http.StatusForbidden, w.Code, "没有 Token 应该返回 403")
	assert.Contains(t, w.Body.String(), "CSRF token missing", "应该返回 CSRF 错误")
}

// TestCSRFMiddleware_POSTWithValidToken 测试带有有效 Token 的 POST 请求
func TestCSRFMiddleware_POSTWithValidToken(t *testing.T) {
	config := DefaultCSRFConfig()
	middleware := CSRFMiddleware(config)

	router := gin.New()
	router.Use(middleware)
	router.POST("/api/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// 生成 Token
	csrfToken, err := generateCSRFToken(config.TokenLength)
	assert.NoError(t, err)
	assert.NotEmpty(t, csrfToken, "应该生成 CSRF Token")

	// 发送带有 Token 的 POST 请求
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/test", nil)
	req.Header.Set(config.HeaderName, csrfToken)
	req.AddCookie(&http.Cookie{
		Name:  config.CookieName,
		Value: csrfToken,
	})
	router.ServeHTTP(w, req)

	// 应该成功
	assert.Equal(t, http.StatusOK, w.Code, "带有有效 Token 的 POST 应该成功")
}

// TestCSRFMiddleware_POSTWithMismatchedToken 测试 Token 不匹配的 POST 请求
func TestCSRFMiddleware_POSTWithMismatchedToken(t *testing.T) {
	config := DefaultCSRFConfig()
	middleware := CSRFMiddleware(config)

	router := gin.New()
	router.Use(middleware)
	router.POST("/api/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// 使用不匹配的 Token
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/test", nil)
	req.Header.Set(config.HeaderName, "wrong-token-in-header")
	req.AddCookie(&http.Cookie{
		Name:  config.CookieName,
		Value: "different-token-in-cookie",
	})
	router.ServeHTTP(w, req)

	// 应该被拒绝
	assert.Equal(t, http.StatusForbidden, w.Code, "Token 不匹配应该返回 403")
	assert.Contains(t, w.Body.String(), "mismatch", "应该返回 Token 不匹配错误")
}

// TestCSRFMiddleware_ExemptPaths 测试豁免路径
func TestCSRFMiddleware_ExemptPaths(t *testing.T) {
	config := DefaultCSRFConfig()
	middleware := CSRFMiddleware(config)

	router := gin.New()
	router.Use(middleware)
	router.POST("/api/login", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "login success"})
	})
	router.POST("/api/register", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "register success"})
	})

	// 测试 /api/login（豁免路径）
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("POST", "/api/login", nil)
	router.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code, "豁免路径应该不检查 CSRF")

	// 测试 /api/register（豁免路径）
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("POST", "/api/register", nil)
	router.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code, "豁免路径应该不检查 CSRF")
}

// TestCSRFMiddleware_OPTIONSRequest 测试 OPTIONS 请求（CORS 预检）
func TestCSRFMiddleware_OPTIONSRequest(t *testing.T) {
	config := DefaultCSRFConfig()
	middleware := CSRFMiddleware(config)

	router := gin.New()
	router.Use(middleware)
	router.OPTIONS("/api/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("OPTIONS", "/api/test", nil)
	router.ServeHTTP(w, req)

	// OPTIONS 请求应该直接放行
	assert.Equal(t, http.StatusOK, w.Code, "OPTIONS 请求应该放行")
}

// TestCSRFMiddleware_PUTAndDELETE 测试 PUT 和 DELETE 请求
func TestCSRFMiddleware_PUTAndDELETE(t *testing.T) {
	config := DefaultCSRFConfig()
	middleware := CSRFMiddleware(config)

	router := gin.New()
	router.Use(middleware)
	router.PUT("/api/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "updated"})
	})
	router.DELETE("/api/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "deleted"})
	})

	// 生成 Token
	token, _ := generateCSRFToken(32)

	// 测试 PUT（应该需要 CSRF Token）
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("PUT", "/api/test", nil)
	req1.Header.Set(config.HeaderName, token)
	req1.AddCookie(&http.Cookie{Name: config.CookieName, Value: token})
	router.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code, "带 Token 的 PUT 应该成功")

	// 测试 DELETE（应该需要 CSRF Token）
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("DELETE", "/api/test", nil)
	req2.Header.Set(config.HeaderName, token)
	req2.AddCookie(&http.Cookie{Name: config.CookieName, Value: token})
	router.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code, "带 Token 的 DELETE 应该成功")
}

// TestGetCSRFToken 测试获取 CSRF Token 的辅助函数
func TestGetCSRFToken(t *testing.T) {
	config := DefaultCSRFConfig()

	// 测试从现有 Cookie 获取
	router := gin.New()
	router.GET("/api/get-token", func(c *gin.Context) {
		token := GetCSRFToken(c, config)
		c.JSON(http.StatusOK, gin.H{"csrf_token": token})
	})

	// 模拟已有 Cookie 的请求
	existingToken := "existing-token-value"
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("GET", "/api/get-token", nil)
	req1.AddCookie(&http.Cookie{Name: config.CookieName, Value: existingToken})
	router.ServeHTTP(w1, req1)

	assert.Equal(t, http.StatusOK, w1.Code)
	assert.Contains(t, w1.Body.String(), existingToken, "应该返回现有 Token")

	// 测试自动生成新 Token
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/api/get-token", nil)
	router.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusOK, w2.Code)
	assert.NotContains(t, w2.Body.String(), "\"\"", "应该生成新 Token")
}

// BenchmarkCSRFMiddleware 性能基准测试
func BenchmarkCSRFMiddleware(b *testing.B) {
	config := DefaultCSRFConfig()
	middleware := CSRFMiddleware(config)

	router := gin.New()
	router.Use(middleware)
	router.GET("/api/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/test", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		router.ServeHTTP(w, req)
	}
}

// BenchmarkCSRFValidation 验证性能基准测试
func BenchmarkCSRFValidation(b *testing.B) {
	config := DefaultCSRFConfig()
	middleware := CSRFMiddleware(config)

	router := gin.New()
	router.Use(middleware)
	router.POST("/api/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	token, _ := generateCSRFToken(32)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/test", nil)
	req.Header.Set(config.HeaderName, token)
	req.AddCookie(&http.Cookie{Name: config.CookieName, Value: token})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		router.ServeHTTP(w, req)
	}
}
