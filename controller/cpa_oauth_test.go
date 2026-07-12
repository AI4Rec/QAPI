package controller

import (
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCPAOAuthCallbackURL(t *testing.T) {
	const state = "0123456789abcdef0123456789abcdef"
	tests := []struct {
		name      string
		callback  string
		wantState string
		wantError string
	}{
		{
			name:      "accepts localhost authorization code callback",
			callback:  "http://localhost:1455/auth/callback?code=oauth-code&state=" + state,
			wantState: state,
		},
		{
			name:      "accepts loopback error callback",
			callback:  "http://127.0.0.1:1455/auth/callback?error=access_denied&state=" + state,
			wantState: state,
		},
		{
			name:      "rejects non-local callback host",
			callback:  "http://example.com/callback?code=oauth-code&state=" + state,
			wantError: "回调地址必须指向 localhost",
		},
		{
			name:      "rejects callback without authorization result",
			callback:  "http://localhost:1455/auth/callback?state=" + state,
			wantError: "回调地址缺少 code 或 error",
		},
		{
			name:      "rejects callback with invalid state",
			callback:  "http://localhost:1455/auth/callback?code=oauth-code&state=short",
			wantError: "回调地址缺少有效的 state",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state, err := parseCPAOAuthCallbackURL(test.callback)
			if test.wantError != "" {
				require.EqualError(t, err, test.wantError)
				assert.Empty(t, state)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.wantState, state)
		})
	}
}

func TestValidCPAOAuthAuthorizationURL(t *testing.T) {
	assert.True(t, validCPAOAuthAuthorizationURL("https://auth.openai.com/authorize?state=test"))
	assert.False(t, validCPAOAuthAuthorizationURL("http://auth.openai.com/authorize"))
	assert.False(t, validCPAOAuthAuthorizationURL("https://user:pass@auth.openai.com/authorize"))
	assert.False(t, validCPAOAuthAuthorizationURL("https://example.com/authorize"))
}

func TestValidateCPARemoteBrowserInput(t *testing.T) {
	tests := []struct {
		name      string
		request   cpaRemoteBrowserInputRequest
		wantError string
	}{
		{
			name:    "accepts click inside viewport",
			request: cpaRemoteBrowserInputRequest{Type: "click", X: 720, Y: 450},
		},
		{
			name:      "rejects click outside viewport",
			request:   cpaRemoteBrowserInputRequest{Type: "click", X: cpaRemoteBrowserWidth + 1, Y: 450},
			wantError: "点击坐标超出范围",
		},
		{
			name:      "rejects non finite scroll delta",
			request:   cpaRemoteBrowserInputRequest{Type: "scroll", X: 720, Y: 450, DeltaY: math.Inf(1)},
			wantError: "滚动输入无效",
		},
		{
			name:      "rejects oversized text",
			request:   cpaRemoteBrowserInputRequest{Type: "text", Text: strings.Repeat("a", 4097)},
			wantError: "输入文本无效",
		},
		{
			name:    "accepts supported special key",
			request: cpaRemoteBrowserInputRequest{Type: "key", Key: "Enter", Code: "Enter", KeyCode: 13},
		},
		{
			name:      "rejects mismatched special key code",
			request:   cpaRemoteBrowserInputRequest{Type: "key", Key: "Enter", Code: "Enter", KeyCode: 9},
			wantError: "按键输入无效",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateCPARemoteBrowserInput(test.request)
			if test.wantError != "" {
				require.EqualError(t, err, test.wantError)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestGetCPARemoteBrowserBindsStateAndUser(t *testing.T) {
	session := &cpaRemoteBrowserSession{state: "0123456789abcdef", userID: 42}
	cpaRemoteBrowserManager.Lock()
	previous := cpaRemoteBrowserManager.active
	cpaRemoteBrowserManager.active = session
	cpaRemoteBrowserManager.Unlock()
	t.Cleanup(func() {
		cpaRemoteBrowserManager.Lock()
		cpaRemoteBrowserManager.active = previous
		cpaRemoteBrowserManager.Unlock()
	})

	got, err := getCPARemoteBrowser(session.state, session.userID)
	require.NoError(t, err)
	assert.Same(t, session, got)

	_, err = getCPARemoteBrowser("fedcba9876543210", session.userID)
	require.EqualError(t, err, "远程浏览器会话不存在")
	_, err = getCPARemoteBrowser(session.state, session.userID+1)
	require.EqualError(t, err, "远程浏览器会话不存在")
}
