package controller

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type createdTokenSaleResponse struct {
	ID          int64  `json:"id"`
	TokenId     int    `json:"token_id"`
	Key         string `json:"key"`
	ExpiredTime int64  `json:"expired_time"`
}

func TestCreateTokenSaleCalculatesExpirationFromValidityDays(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.TokenSale{}))

	before := common.GetTimestamp()
	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token-sales/", map[string]interface{}{
		"name":            "sale-validity-test",
		"amount_minor":    500,
		"granted_quota":   1000,
		"validity_days":   30,
		"max_concurrency": 5,
		"rpm_rate_limit":  60,
		"group":           "default",
	}, 1)
	CreateTokenSale(ctx)
	after := common.GetTimestamp()

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var created createdTokenSaleResponse
	require.NoError(t, common.Unmarshal(response.Data, &created))
	assert.True(t, created.ExpiredTime >= before+30*24*60*60)
	assert.True(t, created.ExpiredTime <= after+30*24*60*60)
	assert.NotEmpty(t, created.Key)

	sale, err := model.GetTokenSaleById(created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ExpiredTime, sale.Token.ExpiredTime)
	assert.Equal(t, 5, sale.Token.MaxConcurrency)
	assert.Equal(t, 60, sale.Token.RPMRateLimit)
}

func TestGetTokenSaleKeyReturnsFullDeliveryKey(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.TokenSale{}))

	token := seedToken(t, db, 1, "sale-copy-test", "sale-copy-full-key")
	sale := model.TokenSale{TokenId: token.Id, UserId: token.UserId, GrantedQuota: 1000}
	require.NoError(t, db.Create(&sale).Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/token-sales/"+strconv.FormatInt(sale.ID, 10)+"/key", nil, 1)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.FormatInt(sale.ID, 10)}}
	GetTokenSaleKey(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var keyData tokenKeyResponse
	require.NoError(t, common.Unmarshal(response.Data, &keyData))
	assert.Equal(t, "sk-"+token.GetFullKey(), keyData.Key)
	assert.NotEqual(t, "sk-"+token.GetMaskedKey(), keyData.Key)
}

func TestCreateTokenSaleRejectsInvalidValidityDays(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.TokenSale{}))

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token-sales/", map[string]interface{}{
		"name":          "sale-invalid-validity",
		"amount_minor":  500,
		"granted_quota": 1000,
		"validity_days": 0,
	}, 1)
	CreateTokenSale(ctx)

	response := decodeAPIResponse(t, recorder)
	assert.False(t, response.Success)
	var count int64
	require.NoError(t, db.Model(&model.TokenSale{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestDeleteTokenSaleRemovesSaleAndToken(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.TokenSale{}))

	token := seedToken(t, db, 1, "sale-delete-test", "sale-delete-full-key")
	sale := model.TokenSale{TokenId: token.Id, UserId: token.UserId, GrantedQuota: 1000}
	require.NoError(t, db.Create(&sale).Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodDelete, "/api/token-sales/"+strconv.FormatInt(sale.ID, 10), nil, 1)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.FormatInt(sale.ID, 10)}}
	DeleteTokenSale(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	assert.ErrorIs(t, model.DB.First(&model.TokenSale{}, sale.ID).Error, gorm.ErrRecordNotFound)
	assert.ErrorIs(t, model.DB.First(&model.Token{}, token.Id).Error, gorm.ErrRecordNotFound)
}
