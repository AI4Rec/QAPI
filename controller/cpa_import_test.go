package controller

import (
	"encoding/base64"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCPAAccountPoolType(t *testing.T) {
	tests := []struct {
		name     string
		fileName string
		want     string
	}{
		{
			name:     "imported account hash suffix",
			fileName: "codex-user-example.com-141d741e.json",
			want:     "cpa_import",
		},
		{
			name:     "official login plan suffix",
			fileName: "codex-user@example.com-plus.json",
			want:     "official_login",
		},
		{
			name:     "official login generic name",
			fileName: "codex-user@example.com.json",
			want:     "official_login",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, cpaAccountPoolType(test.fileName))
		})
	}
}

func TestNormalizeCPAAccountReplacesStringAudienceIDToken(t *testing.T) {
	accessToken := cpaImportTestJWT(t, map[string]any{
		"aud": []any{"codex-cli"},
		"exp": float64(1893456000),
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id": "fc4f8db5-72cd-44cb-ae0d-fef1370a16c8",
			"chatgpt_plan_type":  "k12",
		},
		"https://api.openai.com/profile": map[string]any{
			"email": "MarjorieJayden7563+suy4@outlook.com",
		},
	})
	brokenIDToken := cpaImportTestJWT(t, map[string]any{
		"aud": "chatgpt2api-export",
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id": "broken",
		},
	})

	name, normalized, err := normalizeCPAAccount(map[string]any{
		"access_token": accessToken,
		"id_token":     brokenIDToken,
	})

	require.NoError(t, err)
	assert.Contains(t, name, "MarjorieJayden7563+suy4-outlook.com")
	assert.Equal(t, accessToken, normalized["id_token"])
	assert.Equal(t, "fc4f8db5-72cd-44cb-ae0d-fef1370a16c8", normalized["account_id"])
	assert.Equal(t, "fc4f8db5-72cd-44cb-ae0d-fef1370a16c8", normalized["chatgpt_account_id"])
	assert.Equal(t, "k12", normalized["plan_type"])
	assert.Equal(t, "k12", normalized["chatgpt_plan_type"])
	assert.Equal(t, "MarjorieJayden7563+suy4@outlook.com", normalized["email"])
	assert.Equal(t, false, normalized["disabled"])
}

func TestNormalizeCPAAccountKeepsCompatibleIDToken(t *testing.T) {
	accessToken := cpaImportTestJWT(t, map[string]any{
		"aud": []any{"codex-cli"},
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id": "acct_access",
			"chatgpt_plan_type":  "plus",
		},
	})
	idToken := cpaImportTestJWT(t, map[string]any{
		"aud": []any{"codex-cli"},
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id": "acct_access",
			"chatgpt_plan_type":  "plus",
		},
	})

	_, normalized, err := normalizeCPAAccount(map[string]any{
		"access_token": accessToken,
		"id_token":     idToken,
	})

	require.NoError(t, err)
	assert.Equal(t, idToken, normalized["id_token"])
	assert.Equal(t, "acct_access", normalized["account_id"])
}

func TestNormalizeCPAAccountRejectsMissingAccountID(t *testing.T) {
	accessToken := cpaImportTestJWT(t, map[string]any{
		"aud": []any{"codex-cli"},
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_plan_type": "plus",
		},
	})

	_, _, err := normalizeCPAAccount(map[string]any{"access_token": accessToken})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "chatgpt_account_id")
	assert.NotContains(t, err.Error(), accessToken)
}

func TestDecodeCPAAccountsAcceptsJsonSequence(t *testing.T) {
	accounts, err := decodeCPAAccounts(`{"access_token":"a"}
{"access_token":"b"}`)

	require.NoError(t, err)
	require.Len(t, accounts, 2)
	assert.Equal(t, "a", accounts[0]["access_token"])
	assert.Equal(t, "b", accounts[1]["access_token"])
}

func cpaImportTestJWT(t *testing.T, claims map[string]any) string {
	t.Helper()
	header, err := common.Marshal(map[string]any{"alg": "none", "typ": "JWT"})
	require.NoError(t, err)
	payload, err := common.Marshal(claims)
	require.NoError(t, err)
	return base64.RawURLEncoding.EncodeToString(header) + "." +
		base64.RawURLEncoding.EncodeToString(payload) + ".sig"
}
