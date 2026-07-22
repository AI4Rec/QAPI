package middleware

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
)

type responsesSizeClass int

const (
	responsesSmall responsesSizeClass = iota
	responsesMedium
	responsesLarge
	responsesHuge
)

type responsesAdmissionLimiter struct {
	mu     sync.Mutex
	total  int
	active [4]int
	change chan struct{}
}

func newResponsesAdmissionLimiter() *responsesAdmissionLimiter {
	return &responsesAdmissionLimiter{change: make(chan struct{})}
}

var globalResponsesAdmission = newResponsesAdmissionLimiter()

const responsesAdmissionLeaseSeconds = 10 * 60

func classifyResponsesRequest(contentLength int64) responsesSizeClass {
	if contentLength < 0 {
		return responsesHuge
	}
	switch {
	case contentLength <= 1<<20:
		return responsesSmall
	case contentLength <= 5<<20:
		return responsesMedium
	case contentLength <= 10<<20:
		return responsesLarge
	default:
		return responsesHuge
	}
}

func responsesClassLimit(class responsesSizeClass) int {
	switch class {
	case responsesSmall:
		return constant.ResponsesSmallMaxConcurrency
	case responsesMedium:
		return constant.ResponsesMediumMaxConcurrency
	case responsesLarge:
		return constant.ResponsesLargeMaxConcurrency
	default:
		return constant.ResponsesHugeMaxConcurrency
	}
}

func (l *responsesAdmissionLimiter) acquire(ctx context.Context, class responsesSizeClass, wait time.Duration) bool {
	var timer *time.Timer
	var timeout <-chan time.Time
	if wait > 0 {
		timer = time.NewTimer(wait)
		defer timer.Stop()
		timeout = timer.C
	}

	for {
		l.mu.Lock()
		classLimit := responsesClassLimit(class)
		totalLimit := constant.ResponsesTotalMaxConcurrency
		if classLimit > 0 && totalLimit > 0 && l.active[class] < classLimit && l.total < totalLimit {
			l.active[class]++
			l.total++
			l.mu.Unlock()
			return true
		}
		change := l.change
		l.mu.Unlock()

		if wait <= 0 {
			return false
		}
		select {
		case <-ctx.Done():
			return false
		case <-timeout:
			return false
		case <-change:
		}
	}
}

func (l *responsesAdmissionLimiter) release(class responsesSizeClass) {
	l.mu.Lock()
	if l.active[class] > 0 {
		l.active[class]--
		l.total--
	}
	close(l.change)
	l.change = make(chan struct{})
	l.mu.Unlock()
}

func acquireResponsesAdmission(c *gin.Context, class responsesSizeClass, wait time.Duration) (func(), bool) {
	if !common.RedisEnabled || common.RDB == nil {
		if !globalResponsesAdmission.acquire(c.Request.Context(), class, wait) {
			return func() {}, false
		}
		return func() { globalResponsesAdmission.release(class) }, true
	}

	classKey := common.ResponsesAdmissionRedisClassKey(int(class))
	totalKey := common.ResponsesAdmissionRedisTotalKey
	deadline := time.Now().Add(wait)
	for {
		result, err := common.RDB.Eval(c.Request.Context(), `
local class_current = tonumber(redis.call('GET', KEYS[1]) or '0')
local total_current = tonumber(redis.call('GET', KEYS[2]) or '0')
if class_current >= tonumber(ARGV[1]) or total_current >= tonumber(ARGV[2]) then
  return 0
end
redis.call('INCR', KEYS[1])
redis.call('INCR', KEYS[2])
redis.call('EXPIRE', KEYS[1], ARGV[3])
redis.call('EXPIRE', KEYS[2], ARGV[3])
return 1
`, []string{classKey, totalKey}, responsesClassLimit(class), constant.ResponsesTotalMaxConcurrency, responsesAdmissionLeaseSeconds).Int64()
		if err != nil {
			common.SysError("failed to acquire Redis Responses admission slot, using memory fallback: " + err.Error())
			if !globalResponsesAdmission.acquire(c.Request.Context(), class, wait) {
				return func() {}, false
			}
			return func() { globalResponsesAdmission.release(class) }, true
		}
		if result == 1 {
			return func() {
				releaseCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				if err := common.RDB.Eval(releaseCtx, `
local class_current = tonumber(redis.call('GET', KEYS[1]) or '0')
if class_current <= 1 then redis.call('DEL', KEYS[1]) else redis.call('DECR', KEYS[1]) end
local total_current = tonumber(redis.call('GET', KEYS[2]) or '0')
if total_current <= 1 then redis.call('DEL', KEYS[2]) else redis.call('DECR', KEYS[2]) end
return 1
`, []string{classKey, totalKey}).Err(); err != nil {
					common.SysError("failed to release Redis Responses admission slot: " + err.Error())
				}
			}, true
		}
		if wait <= 0 || time.Now().Add(25*time.Millisecond).After(deadline) {
			return func() {}, false
		}
		select {
		case <-c.Request.Context().Done():
			return func() {}, false
		case <-time.After(25 * time.Millisecond):
		}
	}
}

// ResponsesAdmission bounds the number of large Responses requests that may
// be parsed and relayed concurrently. It intentionally runs before Distribute,
// so rejected requests never enter body storage or channel selection.
func ResponsesAdmission() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !constant.ResponsesFastPathEnabled || c.Request.URL.Path != "/v1/responses" {
			c.Next()
			return
		}

		maxMB := constant.MaxRequestBodyMB
		if maxMB <= 0 {
			maxMB = 128
		}
		if c.Request.ContentLength > int64(maxMB)<<20 {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{
				"error": gin.H{
					"message": "request body is too large",
					"type":    "invalid_request_error",
					"code":    "request_too_large",
				},
			})
			return
		}

		class := classifyResponsesRequest(c.Request.ContentLength)
		wait := time.Duration(constant.ResponsesAdmissionWaitMS) * time.Millisecond
		release, ok := acquireResponsesAdmission(c, class, wait)
		if !ok {
			c.Header("Retry-After", "2")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": gin.H{
					"message": "server is busy, retry shortly",
					"type":    "server_error",
					"code":    "responses_concurrency_limit",
				},
			})
			return
		}
		defer release()
		c.Next()
	}
}
