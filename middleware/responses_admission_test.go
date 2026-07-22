package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResponsesAdmissionRejectsOversizeAndBusyRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originalEnabled := constant.ResponsesFastPathEnabled
	originalMax := constant.MaxRequestBodyMB
	originalHuge := constant.ResponsesHugeMaxConcurrency
	originalTotal := constant.ResponsesTotalMaxConcurrency
	originalWait := constant.ResponsesAdmissionWaitMS
	t.Cleanup(func() {
		constant.ResponsesFastPathEnabled = originalEnabled
		constant.MaxRequestBodyMB = originalMax
		constant.ResponsesHugeMaxConcurrency = originalHuge
		constant.ResponsesTotalMaxConcurrency = originalTotal
		constant.ResponsesAdmissionWaitMS = originalWait
		globalResponsesAdmission = newResponsesAdmissionLimiter()
	})

	constant.ResponsesFastPathEnabled = true
	constant.MaxRequestBodyMB = 32
	constant.ResponsesHugeMaxConcurrency = 1
	constant.ResponsesTotalMaxConcurrency = 1
	constant.ResponsesAdmissionWaitMS = 20
	globalResponsesAdmission = newResponsesAdmissionLimiter()

	entered := make(chan struct{})
	release := make(chan struct{})
	router := gin.New()
	router.Use(ResponsesAdmission())
	router.POST("/v1/responses", func(c *gin.Context) {
		close(entered)
		<-release
		c.Status(http.StatusNoContent)
	})

	firstRecorder := httptest.NewRecorder()
	firstRequest := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	firstRequest.ContentLength = 11 << 20
	firstDone := make(chan struct{})
	go func() {
		router.ServeHTTP(firstRecorder, firstRequest)
		close(firstDone)
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		require.FailNow(t, "first request did not enter handler")
	}

	busyRecorder := httptest.NewRecorder()
	busyRequest := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	busyRequest.ContentLength = 11 << 20
	router.ServeHTTP(busyRecorder, busyRequest)
	assert.Equal(t, http.StatusTooManyRequests, busyRecorder.Code)
	assert.Equal(t, "2", busyRecorder.Header().Get("Retry-After"))

	close(release)
	select {
	case <-firstDone:
	case <-time.After(time.Second):
		require.FailNow(t, "first request did not release admission slot")
	}
	assert.Equal(t, http.StatusNoContent, firstRecorder.Code)

	oversizeRecorder := httptest.NewRecorder()
	oversizeRequest := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	oversizeRequest.ContentLength = 33 << 20
	router.ServeHTTP(oversizeRecorder, oversizeRequest)
	assert.Equal(t, http.StatusRequestEntityTooLarge, oversizeRecorder.Code)
}

func TestResponsesAdmissionBypassesOtherPaths(t *testing.T) {
	originalEnabled := constant.ResponsesFastPathEnabled
	t.Cleanup(func() { constant.ResponsesFastPathEnabled = originalEnabled })
	constant.ResponsesFastPathEnabled = true

	router := gin.New()
	router.Use(ResponsesAdmission())
	router.POST("/v1/responses/compact", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/responses/compact", nil)
	request.ContentLength = 100 << 20
	router.ServeHTTP(recorder, request)
	assert.Equal(t, http.StatusNoContent, recorder.Code)
}
