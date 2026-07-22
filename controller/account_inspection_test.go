package controller

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectAccountJSONProvider(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{name: "CPA", content: `{"type":"codex","access_token":"token"}`, want: "cpa"},
		{name: "Sub2API export", content: `{"type":"sub2api-data","accounts":[]}`, want: "sub2api"},
		{name: "Sub2API account", content: `{"platform":"openai","credentials":{}}`, want: "sub2api"},
		{name: "invalid", content: `{`, want: ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, detectAccountJSONProvider(test.content))
		})
	}
}

func TestParseSub2APIAccountTest(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		success, message, status := parseSub2APIAccountTest([]byte("data: {\"type\":\"test_start\"}\n\ndata: {\"type\":\"test_complete\",\"success\":true,\"text\":\"OK\"}\n"))
		require.True(t, success)
		assert.Equal(t, "OK", message)
		assert.Equal(t, http.StatusOK, status)
	})

	t.Run("upstream failure", func(t *testing.T) {
		success, message, status := parseSub2APIAccountTest([]byte("data: {\"type\":\"error\",\"error\":\"API returned 403: forbidden\"}\n"))
		require.False(t, success)
		assert.Equal(t, "API returned 403: forbidden", message)
		assert.Equal(t, http.StatusForbidden, status)
	})
}
