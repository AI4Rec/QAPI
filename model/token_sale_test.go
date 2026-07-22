package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestTokenSaleLifecyclePreservesLifetimeUsage(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&Token{}, &TokenSale{}))
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&TokenSale{}).Error)
	require.NoError(t, DB.Where("name LIKE ?", "sale-test-%").Delete(&Token{}).Error)
	t.Cleanup(func() {
		require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&TokenSale{}).Error)
		require.NoError(t, DB.Where("name LIKE ?", "sale-test-%").Delete(&Token{}).Error)
	})

	token := &Token{
		UserId:         1,
		Key:            "sale-test-key-000000000000000000000000000000000001",
		Status:         common.TokenStatusEnabled,
		Name:           "sale-test-initial",
		CreatedTime:    common.GetTimestamp(),
		AccessedTime:   common.GetTimestamp(),
		ExpiredTime:    -1,
		RemainQuota:    1000,
		MaxConcurrency: 1,
		RPMRateLimit:   10,
	}
	sale := &TokenSale{
		Buyer:        "buyer-a",
		AmountMinor:  500,
		GrantedQuota: 1000,
		CreatedBy:    1,
		UpdatedBy:    1,
	}
	require.NoError(t, CreateTokenSale(token, sale))
	assert.NotZero(t, token.Id)
	assert.NotZero(t, sale.ID)

	require.NoError(t, DB.Model(&Token{}).Where("id = ?", token.Id).Updates(map[string]any{
		"used_quota":   400,
		"remain_quota": 600,
	}).Error)

	stored, err := GetTokenSaleById(sale.ID)
	require.NoError(t, err)
	stored.Buyer = "buyer-b"
	stored.AmountMinor = 900
	stored.GrantedQuota = 1500
	stored.Token.Name = "sale-test-updated"
	stored.Token.RemainQuota = stored.GrantedQuota - stored.Token.UsedQuota
	stored.Token.MaxConcurrency = 2
	stored.Token.RPMRateLimit = 20
	require.NoError(t, UpdateTokenSale(stored))

	updated, err := GetTokenSaleById(sale.ID)
	require.NoError(t, err)
	assert.Equal(t, "buyer-b", updated.Buyer)
	assert.Equal(t, int64(900), updated.AmountMinor)
	assert.Equal(t, 1500, updated.GrantedQuota)
	assert.Equal(t, 400, updated.Token.UsedQuota, "editing total quota must not erase lifetime usage")
	assert.Equal(t, 1100, updated.Token.RemainQuota)
	assert.Equal(t, 2, updated.Token.MaxConcurrency)
	assert.Equal(t, 20, updated.Token.RPMRateLimit)
}

func TestGetTokenSaleModelUsageAggregatesBillingMetadata(t *testing.T) {
	const tokenId = 992
	now := common.GetTimestamp()
	require.NoError(t, LOG_DB.Where("token_id = ?", tokenId).Delete(&Log{}).Error)
	t.Cleanup(func() {
		require.NoError(t, LOG_DB.Where("token_id = ?", tokenId).Delete(&Log{}).Error)
	})

	logs := []Log{
		{
			CreatedAt:        now - 5,
			Type:             LogTypeConsume,
			TokenId:          tokenId,
			ModelName:        "gpt-sale",
			Quota:            200,
			PromptTokens:     20,
			CompletionTokens: 10,
			Other: common.MapToJsonStr(map[string]interface{}{
				"model_ratio":  2.5,
				"group_ratio":  1.2,
				"billing_mode": "tiered_expr",
				"matched_tier": "base",
			}),
		},
		{
			CreatedAt:        now - 10,
			Type:             LogTypeConsume,
			TokenId:          tokenId,
			ModelName:        "gpt-sale",
			Quota:            100,
			PromptTokens:     12,
			CompletionTokens: 8,
			Other: common.MapToJsonStr(map[string]interface{}{
				"model_ratio": 1.5,
				"group_ratio": 1.0,
			}),
		},
		{
			CreatedAt:        now - 8,
			Type:             LogTypeConsume,
			TokenId:          tokenId,
			ModelName:        "claude-sale",
			Quota:            500,
			PromptTokens:     30,
			CompletionTokens: 15,
			Other: common.MapToJsonStr(map[string]interface{}{
				"model_ratio": 3.0,
				"group_ratio": 1.1,
			}),
		},
		{
			CreatedAt: now - 3,
			Type:      LogTypeRefund,
			TokenId:   tokenId,
			ModelName: "gpt-sale",
			Quota:     999,
		},
		{
			CreatedAt: now - 120,
			Type:      LogTypeConsume,
			TokenId:   tokenId,
			ModelName: "old-sale",
			Quota:     999,
		},
	}
	require.NoError(t, LOG_DB.Create(&logs).Error)

	usage, err := GetTokenSaleModelUsage(tokenId, now-60)
	require.NoError(t, err)
	require.Len(t, usage, 2)

	assert.Equal(t, "claude-sale", usage[0].ModelName)
	assert.Equal(t, int64(1), usage[0].RequestCount)
	assert.Equal(t, int64(500), usage[0].Quota)
	assert.InDelta(t, 3.0, usage[0].ModelRatio, 0.0001)
	assert.InDelta(t, 1.1, usage[0].GroupRatio, 0.0001)

	assert.Equal(t, "gpt-sale", usage[1].ModelName)
	assert.Equal(t, int64(2), usage[1].RequestCount)
	assert.Equal(t, int64(300), usage[1].Quota)
	assert.Equal(t, int64(32), usage[1].PromptTokens)
	assert.Equal(t, int64(18), usage[1].CompletionTokens)
	assert.InDelta(t, 2.5, usage[1].ModelRatio, 0.0001)
	assert.InDelta(t, 1.2, usage[1].GroupRatio, 0.0001)
	assert.Equal(t, "tiered_expr", usage[1].BillingMode)
	assert.Equal(t, "base", usage[1].MatchedTier)
}
