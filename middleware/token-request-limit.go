package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

const tokenConcurrencyLeaseSeconds = 6 * 60 * 60

type tokenRPMWindow struct {
	minute int64
	count  int
}

var tokenRequestLimitState = struct {
	sync.Mutex
	concurrency map[int]int
	rpm         map[int]tokenRPMWindow
}{
	concurrency: make(map[int]int),
	rpm:         make(map[int]tokenRPMWindow),
}

func acquireTokenRequestLimit(c *gin.Context, token *model.Token) (func(), bool) {
	if token == nil || (token.MaxConcurrency <= 0 && token.RPMRateLimit <= 0) {
		return func() {}, true
	}

	if common.RedisEnabled {
		return acquireRedisTokenRequestLimit(c, token)
	}
	return acquireMemoryTokenRequestLimit(c, token)
}

func acquireRedisTokenRequestLimit(c *gin.Context, token *model.Token) (func(), bool) {
	ctx := c.Request.Context()
	if token.RPMRateLimit > 0 {
		key := fmt.Sprintf("token:rpm:%d:%d", token.Id, time.Now().Unix()/60)
		result, err := common.RDB.Eval(ctx, `
local current = redis.call('INCR', KEYS[1])
if current == 1 then redis.call('EXPIRE', KEYS[1], 61) end
if current > tonumber(ARGV[1]) then return 0 end
return current
`, []string{key}, token.RPMRateLimit).Int64()
		if err != nil {
			common.SysError("failed to enforce token RPM limit: " + err.Error())
			abortWithOpenAiMessage(c, http.StatusInternalServerError, "令牌请求限制检查失败")
			return func() {}, false
		}
		if result == 0 {
			abortWithOpenAiMessage(c, http.StatusTooManyRequests, fmt.Sprintf("当前 API Key 每分钟最多请求 %d 次", token.RPMRateLimit))
			return func() {}, false
		}
	}

	if token.MaxConcurrency <= 0 {
		return func() {}, true
	}

	key := "token:concurrency:" + strconv.Itoa(token.Id)
	result, err := common.RDB.Eval(ctx, `
local current = redis.call('INCR', KEYS[1])
if current == 1 then redis.call('EXPIRE', KEYS[1], ARGV[2]) end
if current > tonumber(ARGV[1]) then
  redis.call('DECR', KEYS[1])
  return 0
end
return current
`, []string{key}, token.MaxConcurrency, tokenConcurrencyLeaseSeconds).Int64()
	if err != nil {
		common.SysError("failed to enforce token concurrency limit: " + err.Error())
		abortWithOpenAiMessage(c, http.StatusInternalServerError, "令牌并发限制检查失败")
		return func() {}, false
	}
	if result == 0 {
		abortWithOpenAiMessage(c, http.StatusTooManyRequests, fmt.Sprintf("当前 API Key 最大并发为 %d", token.MaxConcurrency))
		return func() {}, false
	}

	return func() {
		releaseCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := common.RDB.Eval(releaseCtx, `
local current = tonumber(redis.call('GET', KEYS[1]) or '0')
if current <= 1 then
  redis.call('DEL', KEYS[1])
  return 0
end
return redis.call('DECR', KEYS[1])
`, []string{key}).Err(); err != nil {
			common.SysError("failed to release token concurrency slot: " + err.Error())
		}
	}, true
}

func acquireMemoryTokenRequestLimit(c *gin.Context, token *model.Token) (func(), bool) {
	nowMinute := time.Now().Unix() / 60
	tokenRequestLimitState.Lock()
	defer tokenRequestLimitState.Unlock()

	if token.RPMRateLimit > 0 {
		window := tokenRequestLimitState.rpm[token.Id]
		if window.minute != nowMinute {
			window = tokenRPMWindow{minute: nowMinute}
		}
		window.count++
		tokenRequestLimitState.rpm[token.Id] = window
		if window.count > token.RPMRateLimit {
			abortWithOpenAiMessage(c, http.StatusTooManyRequests, fmt.Sprintf("当前 API Key 每分钟最多请求 %d 次", token.RPMRateLimit))
			return func() {}, false
		}
	}

	if token.MaxConcurrency <= 0 {
		return func() {}, true
	}
	current := tokenRequestLimitState.concurrency[token.Id]
	if current >= token.MaxConcurrency {
		abortWithOpenAiMessage(c, http.StatusTooManyRequests, fmt.Sprintf("当前 API Key 最大并发为 %d", token.MaxConcurrency))
		return func() {}, false
	}
	tokenRequestLimitState.concurrency[token.Id] = current + 1

	return func() {
		tokenRequestLimitState.Lock()
		defer tokenRequestLimitState.Unlock()
		current := tokenRequestLimitState.concurrency[token.Id]
		if current <= 1 {
			delete(tokenRequestLimitState.concurrency, token.Id)
			return
		}
		tokenRequestLimitState.concurrency[token.Id] = current - 1
	}, true
}
