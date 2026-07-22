package controller

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestBuildSub2APIImportPayload(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		wantPath    string
		wantAccount bool
	}{
		{
			name:        "native bundle",
			content:     `{"exported_at":"2026-07-21T00:00:00Z","proxies":[],"accounts":[{"name":"a","platform":"openai","type":"oauth","credentials":{"access_token":"token"}}]}`,
			wantPath:    "/api/v1/admin/accounts/data",
			wantAccount: true,
		},
		{
			name:        "native account array",
			content:     `[{"name":"a","platform":"openai","type":"oauth","credentials":{"access_token":"token"}}]`,
			wantPath:    "/api/v1/admin/accounts/data",
			wantAccount: true,
		},
		{
			name:     "CPA account",
			content:  `{"type":"codex","access_token":"token","refresh_token":"refresh"}`,
			wantPath: "/api/v1/admin/accounts/import/codex-session",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, payload, err := buildSub2APIImportPayload(tt.content)
			require.NoError(t, err)
			assert.Equal(t, tt.wantPath, path)
			payloadMap, ok := payload.(map[string]any)
			require.True(t, ok)
			if tt.wantAccount {
				data, ok := payloadMap["data"].(map[string]any)
				require.True(t, ok)
				assert.NotNil(t, data["accounts"])
			} else {
				assert.Equal(t, tt.content, payloadMap["content"])
			}
		})
	}
}

func TestImportSub2APIAccountsForwardsCredentialsServerSide(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var receivedPath string
	var receivedKey string
	var receivedBody []byte
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		receivedKey = r.Header.Get("x-api-key")
		var err error
		receivedBody, err = io.ReadAll(r.Body)
		require.NoError(t, err)
		w.Header().Set("Content-Type", "application/json")
		_, err = w.Write([]byte(`{"code":0,"message":"success","data":{"created":1}}`))
		require.NoError(t, err)
	}))
	t.Cleanup(upstream.Close)

	keyFile := filepath.Join(t.TempDir(), "admin-key")
	require.NoError(t, os.WriteFile(keyFile, []byte("admin-test-key\n"), 0o600))
	t.Setenv("SUB2API_BASE_URL", upstream.URL)
	t.Setenv("SUB2API_ADMIN_KEY_FILE", keyFile)

	router := gin.New()
	router.POST("/api/sub2api/import", ImportSub2APIAccounts)
	body := `{"content":"{\"type\":\"codex\",\"access_token\":\"token\"}"}`
	req := httptest.NewRequest(http.MethodPost, "/api/sub2api/import", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "/api/v1/admin/accounts/import/codex-session", receivedPath)
	assert.Equal(t, "admin-test-key", receivedKey)
	assert.Contains(t, string(receivedBody), `"update_existing":true`)
	assert.Contains(t, recorder.Body.String(), `"success":true`)
}

func TestSetSub2APIQuotaWindowClassifiesWeeklyOnlySnapshot(t *testing.T) {
	usage := &sub2APIAccountUsage{}
	setSub2APIQuotaWindow(usage, &sub2APIOpenAIQuotaWindow{
		UsedPercent:       100,
		LimitWindowSecond: int64((7 * 24 * time.Hour) / time.Second),
		ResetAt:           1_800_000_000,
	}, true)

	assert.Nil(t, usage.RateLimit.PrimaryWindow)
	require.NotNil(t, usage.RateLimit.SecondaryWindow)
	assert.Equal(t, float64(100), usage.RateLimit.SecondaryWindow.UsedPercent)
	assert.Equal(t, int64(1_800_000_000), usage.RateLimit.SecondaryWindow.ResetAt)
	assert.True(t, usage.RateLimit.LimitReached)
}

func TestLoadSub2APIAccountCumulativeOutput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/admin/usage/stats", r.URL.Path)
		assert.Equal(t, "42", r.URL.Query().Get("account_id"))
		assert.Equal(t, "2000-01-01", r.URL.Query().Get("start_date"))
		assert.Equal(t, "2099-12-31", r.URL.Query().Get("end_date"))
		assert.Equal(t, "admin-test-key", r.Header.Get("x-api-key"))
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"code":0,"data":{"total_actual_cost":12.3456}}`))
		require.NoError(t, err)
	}))
	t.Cleanup(upstream.Close)

	keyFile := filepath.Join(t.TempDir(), "admin-key")
	require.NoError(t, os.WriteFile(keyFile, []byte("admin-test-key\n"), 0o600))
	t.Setenv("SUB2API_BASE_URL", upstream.URL)
	t.Setenv("SUB2API_ADMIN_KEY_FILE", keyFile)

	account := &sub2APIAccount{ID: 42}
	loadSub2APIAccountCumulativeOutput(t.Context(), account)
	assert.Equal(t, 12.3456, account.CumulativeOutputUSD)
}

func TestArchiveSub2APIAccountStoresCumulativeOutput(t *testing.T) {
	originalDB := model.DB
	t.Cleanup(func() { model.DB = originalDB })
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "sub2api-archive.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.OperationalAsset{}))
	model.DB = db

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		response := ""
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/admin/accounts/42":
			response = `{"code":0,"data":{"id":42,"name":"archived","platform":"openai","type":"oauth","status":"active","credentials":{"email":"archived@example.com","plan_type":"k12"}}}`
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/admin/usage/stats":
			response = `{"code":0,"data":{"total_actual_cost":27.75}}`
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/admin/accounts/42/schedulable":
			response = `{"code":0,"message":"success"}`
		default:
			http.NotFound(w, r)
			return
		}
		_, writeErr := w.Write([]byte(response))
		require.NoError(t, writeErr)
	}))
	t.Cleanup(upstream.Close)

	keyFile := filepath.Join(t.TempDir(), "admin-key")
	require.NoError(t, os.WriteFile(keyFile, []byte("admin-test-key\n"), 0o600))
	t.Setenv("SUB2API_BASE_URL", upstream.URL)
	t.Setenv("SUB2API_ADMIN_KEY_FILE", keyFile)

	require.NoError(t, archiveSub2APIAccount(t.Context(), 42, "test", 1))
	asset, err := model.GetOperationalAsset(model.OperationalAssetTypeSub2APIAccount, model.OperationalSub2APIAssetKey(42))
	require.NoError(t, err)
	require.NotNil(t, asset)
	var snapshot struct {
		CumulativeOutputUSD float64 `json:"cumulative_output_usd"`
	}
	require.NoError(t, common.UnmarshalJsonStr(asset.Snapshot, &snapshot))
	assert.Equal(t, 27.75, snapshot.CumulativeOutputUSD)
}
