package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"golang.org/x/time/rate"
)

func init() {
	// 设置为测试模式
	gin.SetMode(gin.TestMode)
}

// TestIPRateLimiter_GetLimiter 测试获取限制器
func TestIPRateLimiter_GetLimiter(t *testing.T) {
	limiter := NewIPRateLimiter(rate.Limit(10), 10)

	// 测试第一次获取
	ip := "192.168.1.1"
	l1 := limiter.GetLimiter(ip)
	assert.NotNil(t, l1, "应该返回有效的限制器")

	// 测试第二次获取同一 IP（应该返回相同实例）
	l2 := limiter.GetLimiter(ip)
	assert.Equal(t, l1, l2, "同一 IP 应该返回相同的限制器实例")

	// 测试不同 IP（使用指针地址比较）
	l3 := limiter.GetLimiter("192.168.1.2")
	assert.True(t, l1 != l3, "不同 IP 应该返回不同的限制器实例")
}

// TestIPRateLimiter_Concurrency 测试并发安全性
func TestIPRateLimiter_Concurrency(t *testing.T) {
	limiter := NewIPRateLimiter(rate.Limit(100), 100)

	// 并发获取限制器
	done := make(chan bool)
	for i := 0; i < 100; i++ {
		go func(id int) {
			ip := "192.168.1.1"
			l := limiter.GetLimiter(ip)
			assert.NotNil(t, l)
			done <- true
		}(i)
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 100; i++ {
		<-done
	}

	// 验证只创建了一个限制器实例
	limiter.mu.RLock()
	count := len(limiter.ips)
	limiter.mu.RUnlock()
	assert.Equal(t, 1, count, "并发访问同一 IP 应该只创建一个限制器")
}

// TestRateLimitMiddleware_AllowRequest 测试允许的请求
func TestRateLimitMiddleware_AllowRequest(t *testing.T) {
	// 创建一个宽松的限制器（每秒 10 个请求）
	limiter := NewIPRateLimiter(rate.Limit(10), 10)
	middleware := RateLimitMiddleware(limiter)

	// 创建测试路由
	router := gin.New()
	router.Use(middleware)
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// 发送请求
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	router.ServeHTTP(w, req)

	// 验证响应
	assert.Equal(t, http.StatusOK, w.Code, "正常请求应该返回 200")
}

// TestRateLimitMiddleware_BlockRequest 测试阻止的请求
func TestRateLimitMiddleware_BlockRequest(t *testing.T) {
	// 创建一个严格的限制器（每秒 1 个请求，桶容量 1）
	limiter := NewIPRateLimiter(rate.Limit(1), 1)
	middleware := RateLimitMiddleware(limiter)

	// 创建测试路由
	router := gin.New()
	router.Use(middleware)
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// 第一个请求应该成功
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("GET", "/test", nil)
	req1.RemoteAddr = "192.168.1.1:12345"
	router.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code, "第一个请求应该成功")

	// 立即发送第二个请求应该被阻止
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "192.168.1.1:12345"
	router.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusTooManyRequests, w2.Code, "第二个请求应该被限流")

	// 等待 1 秒后应该可以再次请求
	time.Sleep(1100 * time.Millisecond)
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest("GET", "/test", nil)
	req3.RemoteAddr = "192.168.1.1:12345"
	router.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusOK, w3.Code, "等待后的请求应该成功")
}

// TestRateLimitMiddleware_DifferentIPs 测试不同 IP 独立限流
func TestRateLimitMiddleware_DifferentIPs(t *testing.T) {
	// 创建限制器（每秒 1 个请求）
	limiter := NewIPRateLimiter(rate.Limit(1), 1)
	middleware := RateLimitMiddleware(limiter)

	// 创建测试路由
	router := gin.New()
	router.Use(middleware)
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// IP1 的第一个请求
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("GET", "/test", nil)
	req1.RemoteAddr = "192.168.1.1:12345"
	router.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)

	// IP2 的第一个请求（应该不受 IP1 影响）
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "192.168.1.2:12345"
	router.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code, "不同 IP 应该独立限流")

	// IP1 的第二个请求应该被阻止
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest("GET", "/test", nil)
	req3.RemoteAddr = "192.168.1.1:12345"
	router.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusTooManyRequests, w3.Code)
}

// TestAuthRateLimitMiddleware 测试认证限制中间件
func TestAuthRateLimitMiddleware(t *testing.T) {
	middleware := AuthRateLimitMiddleware()

	// 创建测试路由
	router := gin.New()
	router.Use(middleware)
	router.POST("/login", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "login success"})
	})

	// 前 3 次登录尝试应该成功
	for i := 1; i <= 3; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/login", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, fmt.Sprintf("第 %d 次登录应该成功", i))
	}

	// 第 4 次登录应该被阻止（30 秒内只允许 3 次）
	w4 := httptest.NewRecorder()
	req4, _ := http.NewRequest("POST", "/login", nil)
	req4.RemoteAddr = "192.168.1.1:12345"
	router.ServeHTTP(w4, req4)
	assert.Equal(t, http.StatusTooManyRequests, w4.Code, "30 秒内第 4 次登录应该被阻止")
	assert.Contains(t, w4.Body.String(), "登录尝试次数过多", "应该返回正确的错误消息")
}

// TestStrictRateLimitMiddleware 测试严格限制中间件
func TestStrictRateLimitMiddleware(t *testing.T) {
	// 每 5 秒允许 2 次请求
	middleware := StrictRateLimitMiddleware(5, 2)

	// 创建测试路由
	router := gin.New()
	router.Use(middleware)
	router.POST("/sensitive", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "operation success"})
	})

	// 前 2 次请求应该成功
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/sensitive", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "前 2 次请求应该成功")
	}

	// 第 3 次请求应该被阻止
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/sensitive", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code, "第 3 次请求应该被限流")
	assert.Contains(t, w.Body.String(), "操作过于频繁", "应该返回正确的错误消息")
}

// BenchmarkRateLimitMiddleware 性能基准测试
func BenchmarkRateLimitMiddleware(b *testing.B) {
	limiter := NewIPRateLimiter(rate.Limit(10000), 10000) // 高限制以避免影响测试
	middleware := RateLimitMiddleware(limiter)

	router := gin.New()
	router.Use(middleware)
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		router.ServeHTTP(w, req)
	}
}

// BenchmarkRateLimitMiddleware_ConcurrentIPs 并发多 IP 性能测试
func BenchmarkRateLimitMiddleware_ConcurrentIPs(b *testing.B) {
	limiter := NewIPRateLimiter(rate.Limit(10000), 10000)
	middleware := RateLimitMiddleware(limiter)

	router := gin.New()
	router.Use(middleware)
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		i := 0
		for pb.Next() {
			// 模拟不同 IP
			req.RemoteAddr = "192.168.1." + string(rune(i%255)) + ":12345"
			router.ServeHTTP(w, req)
			i++
		}
	})
}
