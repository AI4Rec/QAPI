package controller

import (
	"bytes"
	"encoding/base64"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadCPAAccountOutputsConvertsQuotaToUSD(t *testing.T) {
	originalQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 500_000
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })

	path := filepath.Join(t.TempDir(), "status.json")
	require.NoError(t, os.WriteFile(path, []byte(`{
  "account_outputs": [
    {"asset_key":"cpa:valid","cumulative_output_quota":750000},
    {"asset_key":"cpa:negative","cumulative_output_quota":-1},
    {"asset_key":"","cumulative_output_quota":500000}
  ]
}`), 0o600))
	t.Setenv("QAPI_CAPACITY_SNAPSHOT_PATH", path)

	outputs := readCPAAccountOutputs()
	require.Len(t, outputs, 1)
	assert.Equal(t, 1.5, outputs["cpa:valid"])
}

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
			name:     "temporary account prefix",
			fileName: "codex-temp-user-example.com-141d741e.json",
			want:     "temporary",
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

func TestResolveCPAAccountPoolTypeUsesStoredAssignment(t *testing.T) {
	assert.Equal(
		t,
		model.CPAAccountPoolTemporary,
		resolveCPAAccountPoolType("codex-user-example.com-141d741e.json", model.CPAAccountPoolTemporary),
	)
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
	}, model.CPAAccountPoolImported)

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
	}, model.CPAAccountPoolImported)

	require.NoError(t, err)
	assert.Equal(t, idToken, normalized["id_token"])
	assert.Equal(t, "acct_access", normalized["account_id"])
}

func TestNormalizeCPAAccountAcceptsExternalAccountID(t *testing.T) {
	accessToken := cpaImportTestJWT(t, map[string]any{
		"aud": []any{"https://api.openai.com/v1"},
		"https://api.openai.com/profile": map[string]any{
			"email": "external-account@example.com",
		},
	})
	idToken := cpaImportTestJWT(t, map[string]any{
		"aud":   []any{"app_EMoamEEZ73f0CkXaXp7hrann"},
		"email": "external-account@example.com",
	})

	_, normalized, err := normalizeCPAAccount(map[string]any{
		"access_token": accessToken,
		"id_token":     idToken,
		"account_id":   "397b7c47-adf3-43c9-89f5-4277828f8681",
	}, model.CPAAccountPoolImported)

	require.NoError(t, err)
	assert.Equal(t, idToken, normalized["id_token"])
	assert.Equal(t, "397b7c47-adf3-43c9-89f5-4277828f8681", normalized["account_id"])
	assert.Equal(t, "397b7c47-adf3-43c9-89f5-4277828f8681", normalized["chatgpt_account_id"])
}

func TestNormalizeCPAAccountRejectsMissingAccountID(t *testing.T) {
	accessToken := cpaImportTestJWT(t, map[string]any{
		"aud": []any{"codex-cli"},
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_plan_type": "plus",
		},
	})

	_, _, err := normalizeCPAAccount(map[string]any{"access_token": accessToken}, model.CPAAccountPoolImported)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "chatgpt_account_id")
	assert.NotContains(t, err.Error(), accessToken)
}

func TestNormalizeCPAAccountUsesTemporaryPoolPrefix(t *testing.T) {
	accessToken := cpaImportTestJWT(t, map[string]any{
		"aud": []any{"codex-cli"},
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id": "acct_temp",
		},
		"https://api.openai.com/profile": map[string]any{
			"email": "temporary@example.com",
		},
	})

	name, _, err := normalizeCPAAccount(
		map[string]any{"access_token": accessToken},
		model.CPAAccountPoolTemporary,
	)

	require.NoError(t, err)
	assert.Contains(t, name, "codex-temp-temporary-example.com-")
}

func TestSortCPAAccountCandidates(t *testing.T) {
	candidates := []cpaAccountCandidate{
		{file: map[string]any{"name": "b.json"}, email: "beta@example.com", sortName: "beta@example.com", costMinor: 0, success: 10},
		{file: map[string]any{"name": "a.json"}, email: "alpha@example.com", sortName: "alpha@example.com", costMinor: 200, success: 5},
	}

	sortCPAAccountCandidates(candidates, "name", "asc")
	assert.Equal(t, "alpha@example.com", candidates[0].email)

	sortCPAAccountCandidates(candidates, "cost", "desc")
	assert.Equal(t, int64(200), candidates[0].costMinor)

	sortCPAAccountCandidates(candidates, "success", "desc")
	assert.Equal(t, float64(10), candidates[0].success)
}

func TestSortCPAAccountCandidatesKeepsMissingTimesLast(t *testing.T) {
	candidates := []cpaAccountCandidate{
		{file: map[string]any{"name": "missing.json"}, sortName: "missing", createdOK: false},
		{file: map[string]any{"name": "older.json"}, sortName: "older", createdAt: 100, createdOK: true},
		{file: map[string]any{"name": "newer.json"}, sortName: "newer", createdAt: 200, createdOK: true},
	}

	sortCPAAccountCandidates(candidates, "created_at", "asc")
	assert.Equal(t, []string{"older.json", "newer.json", "missing.json"}, []string{
		stringValue(candidates[0].file["name"]),
		stringValue(candidates[1].file["name"]),
		stringValue(candidates[2].file["name"]),
	})

	sortCPAAccountCandidates(candidates, "created_at", "desc")
	assert.Equal(t, []string{"newer.json", "older.json", "missing.json"}, []string{
		stringValue(candidates[0].file["name"]),
		stringValue(candidates[1].file["name"]),
		stringValue(candidates[2].file["name"]),
	})
}

func TestCPAAccountSortDefaults(t *testing.T) {
	assert.Equal(t, "asc", defaultCPAAccountSortOrder("name"))
	assert.Equal(t, "asc", defaultCPAAccountSortOrder("status"))
	assert.Equal(t, "desc", defaultCPAAccountSortOrder("cost"))
	assert.Equal(t, "desc", defaultCPAAccountSortOrder("updated_at"))
}

func TestCPATimestampValue(t *testing.T) {
	unix, ok := cpaTimestampValue("2026-07-18T12:34:56Z")
	require.True(t, ok)
	assert.Equal(t, int64(1784378096), unix)

	unix, ok = cpaTimestampValue("1784378096000")
	require.True(t, ok)
	assert.Equal(t, int64(1784378096), unix)

	_, ok = cpaTimestampValue("")
	assert.False(t, ok)
}

func TestCPANumberValueRejectsNonFiniteValues(t *testing.T) {
	assert.Zero(t, cpaNumberValue(math.NaN()))
	assert.Zero(t, cpaNumberValue("+Inf"))
}

func TestBatchManageCPAAccountsRejectsWrongDeleteConfirmation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/cpa/accounts/batch", BatchManageCPAAccounts)
	payload, err := common.Marshal(map[string]any{
		"names":               []string{"first.json", "second.json"},
		"action":              cpaBatchActionDelete,
		"delete_confirmation": "DELETE 1",
	})
	require.NoError(t, err)

	request := httptest.NewRequest(http.MethodPost, "/api/cpa/accounts/batch", bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	assert.Equal(t, http.StatusBadRequest, response.Code)
	var body map[string]any
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &body))
	assert.Equal(t, "永久删除确认文本无效", body["message"])
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
