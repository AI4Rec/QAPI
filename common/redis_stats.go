package common

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
)

const ResponsesAdmissionRedisTotalKey = "new-api:responses:admission:total"

func ResponsesAdmissionRedisClassKey(class int) string {
	return fmt.Sprintf("new-api:responses:admission:class:%d", class)
}

type RedisPoolStats struct {
	Hits       uint32 `json:"hits"`
	Misses     uint32 `json:"misses"`
	Timeouts   uint32 `json:"timeouts"`
	TotalConns uint32 `json:"total_conns"`
	IdleConns  uint32 `json:"idle_conns"`
	StaleConns uint32 `json:"stale_conns"`
}

type RedisAdmissionStats struct {
	Total  int64 `json:"total"`
	Small  int64 `json:"small"`
	Medium int64 `json:"medium"`
	Large  int64 `json:"large"`
	Huge   int64 `json:"huge"`
}

type RedisStats struct {
	Enabled                bool                `json:"enabled"`
	Healthy                bool                `json:"healthy"`
	LatencyMS              float64             `json:"latency_ms"`
	Version                string              `json:"version"`
	UptimeSeconds          int64               `json:"uptime_seconds"`
	UsedMemoryBytes        int64               `json:"used_memory_bytes"`
	PeakMemoryBytes        int64               `json:"peak_memory_bytes"`
	MaxMemoryBytes         int64               `json:"max_memory_bytes"`
	MemoryUsagePercent     float64             `json:"memory_usage_percent"`
	MaxMemoryPolicy        string              `json:"max_memory_policy"`
	ConnectedClients       int64               `json:"connected_clients"`
	BlockedClients         int64               `json:"blocked_clients"`
	TotalCommandsProcessed int64               `json:"total_commands_processed"`
	InstantaneousOpsPerSec int64               `json:"instantaneous_ops_per_sec"`
	KeyspaceHits           int64               `json:"keyspace_hits"`
	KeyspaceMisses         int64               `json:"keyspace_misses"`
	HitRatePercent         float64             `json:"hit_rate_percent"`
	DBSize                 int64               `json:"db_size"`
	Pool                   RedisPoolStats      `json:"pool"`
	Admission              RedisAdmissionStats `json:"admission"`
	Error                  string              `json:"error,omitempty"`
}

func GetRedisStats(ctx context.Context) RedisStats {
	stats := RedisStats{Enabled: RedisEnabled && RDB != nil}
	if !stats.Enabled {
		return stats
	}

	queryCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	pipe := RDB.Pipeline()
	infoCmd := pipe.Info(queryCtx)
	dbSizeCmd := pipe.DBSize(queryCtx)
	admissionCmd := pipe.MGet(
		queryCtx,
		ResponsesAdmissionRedisTotalKey,
		ResponsesAdmissionRedisClassKey(0),
		ResponsesAdmissionRedisClassKey(1),
		ResponsesAdmissionRedisClassKey(2),
		ResponsesAdmissionRedisClassKey(3),
	)
	startedAt := time.Now()
	_, err := pipe.Exec(queryCtx)
	stats.LatencyMS = float64(time.Since(startedAt).Microseconds()) / 1000
	if err != nil && err != redis.Nil {
		stats.Error = err.Error()
		return stats
	}

	info, err := infoCmd.Result()
	if err != nil {
		stats.Error = err.Error()
		return stats
	}
	values := parseRedisInfo(info)
	stats.Healthy = true
	stats.Version = values["redis_version"]
	stats.UptimeSeconds = redisInfoInt(values, "uptime_in_seconds")
	stats.UsedMemoryBytes = redisInfoInt(values, "used_memory")
	stats.PeakMemoryBytes = redisInfoInt(values, "used_memory_peak")
	stats.MaxMemoryBytes = redisInfoInt(values, "maxmemory")
	stats.MaxMemoryPolicy = values["maxmemory_policy"]
	stats.ConnectedClients = redisInfoInt(values, "connected_clients")
	stats.BlockedClients = redisInfoInt(values, "blocked_clients")
	stats.TotalCommandsProcessed = redisInfoInt(values, "total_commands_processed")
	stats.InstantaneousOpsPerSec = redisInfoInt(values, "instantaneous_ops_per_sec")
	stats.KeyspaceHits = redisInfoInt(values, "keyspace_hits")
	stats.KeyspaceMisses = redisInfoInt(values, "keyspace_misses")
	if stats.MaxMemoryBytes > 0 {
		stats.MemoryUsagePercent = float64(stats.UsedMemoryBytes) / float64(stats.MaxMemoryBytes) * 100
	}
	lookupTotal := stats.KeyspaceHits + stats.KeyspaceMisses
	if lookupTotal > 0 {
		stats.HitRatePercent = float64(stats.KeyspaceHits) / float64(lookupTotal) * 100
	}
	stats.DBSize, _ = dbSizeCmd.Result()
	if admissionValues, admissionErr := admissionCmd.Result(); admissionErr == nil && len(admissionValues) == 5 {
		stats.Admission = RedisAdmissionStats{
			Total:  redisResultInt(admissionValues[0]),
			Small:  redisResultInt(admissionValues[1]),
			Medium: redisResultInt(admissionValues[2]),
			Large:  redisResultInt(admissionValues[3]),
			Huge:   redisResultInt(admissionValues[4]),
		}
	}
	pool := RDB.PoolStats()
	stats.Pool = RedisPoolStats{
		Hits:       pool.Hits,
		Misses:     pool.Misses,
		Timeouts:   pool.Timeouts,
		TotalConns: pool.TotalConns,
		IdleConns:  pool.IdleConns,
		StaleConns: pool.StaleConns,
	}
	return stats
}

func parseRedisInfo(info string) map[string]string {
	values := make(map[string]string)
	for _, line := range strings.Split(info, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, ":")
		if found {
			values[key] = value
		}
	}
	return values
}

func redisInfoInt(values map[string]string, key string) int64 {
	value, _ := strconv.ParseInt(values[key], 10, 64)
	return value
}

func redisResultInt(value any) int64 {
	switch typed := value.(type) {
	case string:
		parsed, _ := strconv.ParseInt(typed, 10, 64)
		return parsed
	case []byte:
		parsed, _ := strconv.ParseInt(string(typed), 10, 64)
		return parsed
	default:
		return 0
	}
}
