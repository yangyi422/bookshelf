package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "same-origin")
		c.Header("Content-Security-Policy", "default-src 'self'; frame-ancestors 'none'")
		c.Next()
	}
}

type visitor struct {
	count int
	reset time.Time
}

func LoginRateLimit(limit int, window time.Duration) gin.HandlerFunc {
	var mu sync.Mutex
	visitors := map[string]visitor{}
	return func(c *gin.Context) {
		now := time.Now()
		key := c.ClientIP()
		mu.Lock()
		v := visitors[key]
		if now.After(v.reset) {
			v = visitor{reset: now.Add(window)}
		}
		if v.count >= limit {
			mu.Unlock()
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "too many login attempts"})
			return
		}
		v.count++
		visitors[key] = v
		mu.Unlock()
		c.Next()
	}
}
