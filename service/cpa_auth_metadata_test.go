package service

import (
	"encoding/base64"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractCPAAuthMetadataFromFlattenedIDToken(t *testing.T) {
	metadata := ExtractCPAAuthMetadata(map[string]any{
		"id_token": map[string]any{
			"chatgpt_account_id": "acct_123",
			"plan_type":          "k12",
		},
	})

	assert.Equal(t, "acct_123", metadata.AccountID)
	assert.Equal(t, "k12", metadata.PlanType)
}

func TestExtractCPAAuthMetadataFromAccessToken(t *testing.T) {
	token := cpaTestJWT(t, map[string]any{
		"aud": []any{"codex-cli"},
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id": "acct_access",
			"chatgpt_plan_type":  "k12",
		},
		"https://api.openai.com/profile": map[string]any{
			"email": "user@example.com",
		},
	})

	metadata := ExtractCPAAuthMetadata(map[string]any{"access_token": token})

	assert.Equal(t, "acct_access", metadata.AccountID)
	assert.Equal(t, "k12", metadata.PlanType)
	assert.Equal(t, "user@example.com", metadata.Email)
}

func TestIsCLIProxyCompatibleCPAIDTokenRejectsStringAudience(t *testing.T) {
	token := cpaTestJWT(t, map[string]any{
		"aud": "chatgpt2api-export",
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id": "acct_bad",
			"chatgpt_plan_type":  "plus",
		},
	})

	assert.False(t, IsCLIProxyCompatibleCPAIDToken(token))
}

func TestIsCLIProxyCompatibleCPAIDTokenAcceptsArrayAudience(t *testing.T) {
	token := cpaTestJWT(t, map[string]any{
		"aud": []any{"codex-cli"},
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id": "acct_good",
			"chatgpt_plan_type":  "k12",
		},
	})

	assert.True(t, IsCLIProxyCompatibleCPAIDToken(token))
}

func TestIsCLIProxyCompatibleCPAToken(t *testing.T) {
	tests := []struct {
		name  string
		token string
		want  bool
	}{
		{
			name: "array audience without embedded account id",
			token: cpaTestJWT(t, map[string]any{
				"aud": []any{"https://api.openai.com/v1"},
			}),
			want: true,
		},
		{
			name: "string audience",
			token: cpaTestJWT(t, map[string]any{
				"aud": "chatgpt2api-export",
			}),
			want: false,
		},
		{
			name:  "malformed jwt",
			token: "not-a-jwt",
			want:  false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, IsCLIProxyCompatibleCPAToken(test.token))
		})
	}
}

func TestBuildCPAAuthMetadataPatchReplacesBrokenIDTokenWithAccessToken(t *testing.T) {
	brokenIDToken := cpaTestJWT(t, map[string]any{
		"aud": "chatgpt2api-export",
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id": "acct_bad",
		},
	})
	accessToken := cpaTestJWT(t, map[string]any{
		"aud": []any{"codex-cli"},
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id": "acct_good",
			"chatgpt_plan_type":  "k12",
		},
		"https://api.openai.com/profile": map[string]any{
			"email": "user@example.com",
		},
	})

	patch := BuildCPAAuthMetadataPatch("codex-user.json", map[string]any{
		"id_token":      brokenIDToken,
		"access_token":  accessToken,
		"password":      "secret",
		"session_token": "session",
	}, CPAAuthMetadata{AccountID: "acct_good", PlanType: "k12", Email: "user@example.com"})

	require.NotNil(t, patch)
	assert.Equal(t, "codex-user.json", patch["name"])
	assert.Equal(t, accessToken, patch["id_token"])
	assert.Equal(t, "acct_good", patch["account_id"])
	assert.Equal(t, "acct_good", patch["chatgpt_account_id"])
	assert.Equal(t, "k12", patch["plan_type"])
	assert.Equal(t, "k12", patch["chatgpt_plan_type"])
	assert.Equal(t, "user@example.com", patch["email"])
	_, hasPassword := patch["password"]
	_, hasSessionToken := patch["session_token"]
	assert.False(t, hasPassword)
	assert.False(t, hasSessionToken)
}

func TestBuildCPAAuthMetadataPatchKeepsCompatibleIDToken(t *testing.T) {
	idToken := cpaTestJWT(t, map[string]any{
		"aud": []any{"codex-cli"},
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id": "acct_good",
			"chatgpt_plan_type":  "plus",
		},
	})

	patch := BuildCPAAuthMetadataPatch("codex-user.json", map[string]any{
		"id_token":     idToken,
		"access_token": cpaTestJWT(t, map[string]any{"aud": []any{"codex-cli"}}),
	}, CPAAuthMetadata{AccountID: "acct_good", PlanType: "plus"})

	require.NotNil(t, patch)
	_, hasIDToken := patch["id_token"]
	assert.False(t, hasIDToken)
	assert.Equal(t, "acct_good", patch["account_id"])
	assert.Equal(t, "plus", patch["plan_type"])
}

func cpaTestJWT(t *testing.T, claims map[string]any) string {
	t.Helper()
	header, err := common.Marshal(map[string]any{"alg": "none", "typ": "JWT"})
	require.NoError(t, err)
	payload, err := common.Marshal(claims)
	require.NoError(t, err)
	return base64.RawURLEncoding.EncodeToString(header) + "." +
		base64.RawURLEncoding.EncodeToString(payload) + ".sig"
}
