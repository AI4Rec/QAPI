package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseRedisInfo(t *testing.T) {
	values := parseRedisInfo("# Server\r\nredis_version:7.0.15\r\n# Memory\r\nused_memory:1415576\r\nmaxmemory:100663296\r\n")

	assert.Equal(t, "7.0.15", values["redis_version"])
	assert.Equal(t, int64(1415576), redisInfoInt(values, "used_memory"))
	assert.Equal(t, int64(100663296), redisInfoInt(values, "maxmemory"))
	assert.Zero(t, redisInfoInt(values, "missing"))
}

func TestRedisResultInt(t *testing.T) {
	assert.Equal(t, int64(3), redisResultInt("3"))
	assert.Equal(t, int64(4), redisResultInt([]byte("4")))
	assert.Zero(t, redisResultInt(nil))
}
