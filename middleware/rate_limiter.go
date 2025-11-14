package middleware

import (
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// IPRateLimiter IP 级别的速率限制器
type IPRateLimiter struct {
	ips map[string]*rate.Limiter
	mu  *sync.RWMutex
	r   rate.Limit // 每秒允许的请求数
	b   int        // 令牌桶容量
}

// NewIPRateLimiter 创建新的 IP 速率限制器
func NewIPRateLimiter(r rate.Limit, b int) *IPRateLimiter {
	limiter := &IPRateLimiter{
		ips: make(map[string]*rate.Limiter),
		mu:  &sync.RWMutex{},
		r:   r,
		b:   b,
	}

	// 定期清理过期的限制器 (节省内存)
	go limiter.cleanupStaleEntries()

	return limiter
}

// GetLimiter 获取或创建指定 IP 的限制器
func (i *IPRateLimiter) GetLimiter(ip string) *rate.Limiter {
	i.mu.Lock()
	defer i.mu.Unlock()

	limiter, exists := i.ips[ip]
	if !exists {
		limiter = rate.NewLimiter(i.r, i.b)
		i.ips[ip] = limiter
	}

	return limiter
}

// cleanupStaleEntries 定期清理过期的限制器
func (i *IPRateLimiter) cleanupStaleEntries() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		i.mu.Lock()
		// 简单策略: 每小时清空一次 (生产环境可以更智能)
		i.ips = make(map[string]*rate.Limiter)
		i.mu.Unlock()
		log.Printf("🧹 [RATE_LIMITER] 清理限制器缓存 (每小时定期清理)")
	}
}

// RateLimitMiddleware 通用速率限制中间件
// 参数: limiter - 速率限制器实例
// 用途: 限制全局 API 请求频率
func RateLimitMiddleware(limiter *IPRateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()

		l := limiter.GetLimiter(ip)
		if !l.Allow() {
			log.Printf("⚠️ [RATE_LIMIT] IP %s 请求过于频繁 (全局限制)", ip)
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "请求过于频繁，请稍后再试",
				"retry_after": 60,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// AuthRateLimitMiddleware 认证端点专用速率限制 (更严格)
// 用途: 防止暴力破解登录/OTP
// 限制: 每 10 秒最多 1 次登录尝试
func AuthRateLimitMiddleware() gin.HandlerFunc {
	// 每 10 秒允许 1 次登录尝试
	limiter := NewIPRateLimiter(rate.Every(10*time.Second), 1)

	return func(c *gin.Context) {
		ip := c.ClientIP()

		l := limiter.GetLimiter(ip)
		if !l.Allow() {
			log.Printf("🚨 [RATE_LIMIT] IP %s 登录尝试频率过高 (认证限制)", ip)
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "登录尝试次数过多，请 10 秒后重试",
				"retry_after": 10,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// StrictRateLimitMiddleware 严格速率限制 (用于敏感操作)
// 参数: seconds - 时间窗口（秒）, maxRequests - 最大请求数
// 用途: 保护敏感操作（如修改配置、删除数据）
func StrictRateLimitMiddleware(seconds int, maxRequests int) gin.HandlerFunc {
	limiter := NewIPRateLimiter(rate.Every(time.Duration(seconds)*time.Second), maxRequests)

	return func(c *gin.Context) {
		ip := c.ClientIP()

		l := limiter.GetLimiter(ip)
		if !l.Allow() {
			log.Printf("⚠️ [RATE_LIMIT] IP %s 触发严格限制 (%d 秒 %d 次)", ip, seconds, maxRequests)
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "操作过于频繁，请稍后再试",
				"retry_after": seconds,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
