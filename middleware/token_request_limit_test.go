package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTokenLimitTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	return ctx, recorder
}

func TestMemoryTokenConcurrencyLimitReleasesSlot(t *testing.T) {
	tokenRequestLimitState.Lock()
	tokenRequestLimitState.concurrency = make(map[int]int)
	tokenRequestLimitState.rpm = make(map[int]tokenRPMWindow)
	tokenRequestLimitState.Unlock()
	token := &model.Token{Id: 991, MaxConcurrency: 1}

	firstContext, _ := newTokenLimitTestContext()
	release, allowed := acquireMemoryTokenRequestLimit(firstContext, token)
	require.True(t, allowed)

	secondContext, secondRecorder := newTokenLimitTestContext()
	_, allowed = acquireMemoryTokenRequestLimit(secondContext, token)
	assert.False(t, allowed)
	assert.Equal(t, 429, secondRecorder.Code)

	release()
	thirdContext, _ := newTokenLimitTestContext()
	thirdRelease, allowed := acquireMemoryTokenRequestLimit(thirdContext, token)
	require.True(t, allowed)
	thirdRelease()
}
