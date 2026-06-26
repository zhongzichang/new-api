package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
)

const (
	EmailVerificationRateLimitMark       = "EV"
	EmailVerificationMaxRequests         = 2    // 30秒内最多2次/IP
	EmailVerificationDuration            = 30   // 30秒时间窗口
	EmailVerificationPerEmailMaxRequests = 5    // 每小时最多5次/邮箱
	EmailVerificationPerEmailDuration    = 3600 // 1小时时间窗口
)

func redisEmailVerificationRateLimiter(c *gin.Context) {
	ctx := context.Background()
	rdb := common.RDB

	// IP 限流
	ipKey := "emailVerification:" + EmailVerificationRateLimitMark + ":" + c.ClientIP()
	count, err := rdb.Incr(ctx, ipKey).Result()
	if err != nil {
		memoryEmailVerificationRateLimiter(c)
		return
	}
	if count == 1 {
		_ = rdb.Expire(ctx, ipKey, time.Duration(EmailVerificationDuration)*time.Second).Err()
	}
	if count > int64(EmailVerificationMaxRequests) {
		ttl, _ := rdb.TTL(ctx, ipKey).Result()
		waitSeconds := int64(EmailVerificationDuration)
		if ttl > 0 {
			waitSeconds = int64(ttl.Seconds())
		}
		c.JSON(http.StatusTooManyRequests, gin.H{
			"success": false,
			"message": fmt.Sprintf("发送过于频繁，请等待 %d 秒后再试", waitSeconds),
		})
		c.Abort()
		return
	}

	// 邮箱限流
	email := c.Query("email")
	if email != "" {
		emailKey := "emailVerification:email:" + email
		emailCount, err := rdb.Incr(ctx, emailKey).Result()
		if err == nil {
			if emailCount == 1 {
				_ = rdb.Expire(ctx, emailKey, time.Duration(EmailVerificationPerEmailDuration)*time.Second).Err()
			}
			if emailCount > int64(EmailVerificationPerEmailMaxRequests) {
				ttl, _ := rdb.TTL(ctx, emailKey).Result()
				waitMinutes := int64(EmailVerificationPerEmailDuration / 60)
				if ttl > 0 {
					waitMinutes = int64(ttl.Minutes())
				}
				c.JSON(http.StatusTooManyRequests, gin.H{
					"success": false,
					"message": fmt.Sprintf("该邮箱发送过于频繁，请等待 %d 分钟后再试", waitMinutes),
				})
				c.Abort()
				return
			}
		}
	}

	c.Next()
}

func memoryEmailVerificationRateLimiter(c *gin.Context) {
	// IP 限流
	ipKey := EmailVerificationRateLimitMark + ":" + c.ClientIP()
	if !inMemoryRateLimiter.Request(ipKey, EmailVerificationMaxRequests, EmailVerificationDuration) {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"success": false,
			"message": "发送过于频繁，请稍后再试",
		})
		c.Abort()
		return
	}

	// 邮箱限流
	email := c.Query("email")
	if email != "" {
		emailKey := "email:" + email
		if !inMemoryRateLimiter.Request(emailKey, EmailVerificationPerEmailMaxRequests, EmailVerificationPerEmailDuration) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"message": "该邮箱发送过于频繁，请稍后再试",
			})
			c.Abort()
			return
		}
	}

	c.Next()
}

func EmailVerificationRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		if common.RedisEnabled {
			redisEmailVerificationRateLimiter(c)
		} else {
			inMemoryRateLimiter.Init(common.RateLimitKeyExpirationDuration)
			memoryEmailVerificationRateLimiter(c)
		}
	}
}
