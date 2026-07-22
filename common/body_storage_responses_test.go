package common

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResponsesRequestBodyForcedToDisk(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originalConfig := GetDiskCacheConfig()
	originalEnabled := constant.ResponsesFastPathEnabled
	originalThreshold := constant.ResponsesSpoolThresholdMB
	originalMax := constant.MaxRequestBodyMB
	t.Cleanup(func() {
		SetDiskCacheConfig(originalConfig)
		constant.ResponsesFastPathEnabled = originalEnabled
		constant.ResponsesSpoolThresholdMB = originalThreshold
		constant.MaxRequestBodyMB = originalMax
	})

	SetDiskCacheConfig(DiskCacheConfig{Enabled: false, ThresholdMB: 10, MaxSizeMB: 64, Path: t.TempDir()})
	constant.ResponsesFastPathEnabled = true
	constant.ResponsesSpoolThresholdMB = 1
	constant.MaxRequestBodyMB = 4

	body := bytes.Repeat([]byte("x"), 2<<20)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	storage, err := GetBodyStorage(c)
	require.NoError(t, err)
	require.True(t, storage.IsDisk())
	assert.Equal(t, int64(len(body)), storage.Size())

	CleanupBodyStorage(c)
	stats := GetDiskCacheStats()
	assert.Zero(t, stats.ActiveDiskFiles)
}

func TestResponsesUnknownLengthForcedToDiskAndRewindable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originalConfig := GetDiskCacheConfig()
	originalEnabled := constant.ResponsesFastPathEnabled
	originalThreshold := constant.ResponsesSpoolThresholdMB
	originalMax := constant.MaxRequestBodyMB
	t.Cleanup(func() {
		SetDiskCacheConfig(originalConfig)
		constant.ResponsesFastPathEnabled = originalEnabled
		constant.ResponsesSpoolThresholdMB = originalThreshold
		constant.MaxRequestBodyMB = originalMax
	})

	SetDiskCacheConfig(DiskCacheConfig{Enabled: false, MaxSizeMB: 64, Path: t.TempDir()})
	constant.ResponsesFastPathEnabled = true
	constant.ResponsesSpoolThresholdMB = 1
	constant.MaxRequestBodyMB = 4

	body := []byte(`{"model":"gpt-5","input":"hello"}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	c.Request.ContentLength = -1
	c.Request.Header.Set("Content-Type", "application/json")

	var first map[string]any
	require.NoError(t, UnmarshalBodyReusable(c, &first))
	storage, err := GetBodyStorage(c)
	require.NoError(t, err)
	require.True(t, storage.IsDisk())
	readBack, err := io.ReadAll(storage)
	require.NoError(t, err)
	assert.Equal(t, body, readBack)
	CleanupBodyStorage(c)
}

func TestDiskBackedReusableJSONRejectsTrailingValue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originalConfig := GetDiskCacheConfig()
	originalEnabled := constant.ResponsesFastPathEnabled
	originalMax := constant.MaxRequestBodyMB
	t.Cleanup(func() {
		SetDiskCacheConfig(originalConfig)
		constant.ResponsesFastPathEnabled = originalEnabled
		constant.MaxRequestBodyMB = originalMax
	})

	SetDiskCacheConfig(DiskCacheConfig{Enabled: false, MaxSizeMB: 64, Path: t.TempDir()})
	constant.ResponsesFastPathEnabled = true
	constant.MaxRequestBodyMB = 4

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"model":"gpt-5"} {"input":"x"}`))
	c.Request.ContentLength = -1
	c.Request.Header.Set("Content-Type", "application/json")

	var value map[string]any
	require.Error(t, UnmarshalBodyReusable(c, &value))
	storage, err := GetBodyStorage(c)
	require.NoError(t, err)
	_, err = storage.Seek(0, io.SeekStart)
	require.NoError(t, err)
	CleanupBodyStorage(c)
}
