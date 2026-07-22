package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type createTokenSaleRequest struct {
	Name               string  `json:"name"`
	Buyer              string  `json:"buyer"`
	OrderNo            string  `json:"order_no"`
	AmountMinor        int64   `json:"amount_minor"`
	GrantedQuota       int     `json:"granted_quota"`
	ValidityDays       int     `json:"validity_days"`
	MaxConcurrency     int     `json:"max_concurrency"`
	RPMRateLimit       int     `json:"rpm_rate_limit"`
	ModelLimitsEnabled bool    `json:"model_limits_enabled"`
	ModelLimits        string  `json:"model_limits"`
	AllowIps           *string `json:"allow_ips"`
	Group              string  `json:"group"`
	CrossGroupRetry    bool    `json:"cross_group_retry"`
	Note               string  `json:"note"`
}

type updateTokenSaleRequest struct {
	Name               string  `json:"name"`
	Buyer              string  `json:"buyer"`
	OrderNo            string  `json:"order_no"`
	AmountMinor        int64   `json:"amount_minor"`
	GrantedQuota       int     `json:"granted_quota"`
	ExpiredTime        int64   `json:"expired_time"`
	MaxConcurrency     int     `json:"max_concurrency"`
	RPMRateLimit       int     `json:"rpm_rate_limit"`
	ModelLimitsEnabled bool    `json:"model_limits_enabled"`
	ModelLimits        string  `json:"model_limits"`
	AllowIps           *string `json:"allow_ips"`
	Group              string  `json:"group"`
	CrossGroupRetry    bool    `json:"cross_group_retry"`
	Status             string  `json:"status"`
	Note               string  `json:"note"`
}

type tokenSaleResponse struct {
	ID                   int64  `json:"id"`
	TokenId              int    `json:"token_id"`
	Name                 string `json:"name"`
	Key                  string `json:"key"`
	Buyer                string `json:"buyer"`
	OrderNo              string `json:"order_no"`
	AmountMinor          int64  `json:"amount_minor"`
	Currency             string `json:"currency"`
	GrantedQuota         int    `json:"granted_quota"`
	UsedQuota            int    `json:"used_quota"`
	RemainingQuota       int    `json:"remaining_quota"`
	ExpiredTime          int64  `json:"expired_time"`
	AccessedTime         int64  `json:"accessed_time"`
	TokenStatus          int    `json:"token_status"`
	SaleStatus           string `json:"sale_status"`
	MaxConcurrency       int    `json:"max_concurrency"`
	RPMRateLimit         int    `json:"rpm_rate_limit"`
	ModelLimitsEnabled   bool   `json:"model_limits_enabled"`
	ModelLimits          string `json:"model_limits"`
	AllowIps             string `json:"allow_ips"`
	Group                string `json:"group"`
	CrossGroupRetry      bool   `json:"cross_group_retry"`
	Note                 string `json:"note"`
	DeliveredAt          int64  `json:"delivered_at"`
	CreatedAt            int64  `json:"created_at"`
	TodayRequestCount    int64  `json:"today_request_count"`
	TodayQuota           int64  `json:"today_quota"`
	SevenDayRequestCount int64  `json:"seven_day_request_count"`
	SevenDayQuota        int64  `json:"seven_day_quota"`
	LastUsedAt           int64  `json:"last_used_at"`
}

func validateTokenSaleInput(name string, amountMinor int64, grantedQuota int, expiredTime int64, maxConcurrency int, rpmRateLimit int) error {
	if strings.TrimSpace(name) == "" || len(name) > 50 {
		return errors.New("Key 名称不能为空且不能超过 50 个字符")
	}
	if amountMinor < 0 {
		return errors.New("实收金额不能为负数")
	}
	if grantedQuota <= 0 || grantedQuota > common.MaxQuota {
		return fmt.Errorf("额度必须在 1 到 %d 之间", common.MaxQuota)
	}
	if expiredTime != -1 && expiredTime <= common.GetTimestamp() {
		return errors.New("到期时间必须晚于当前时间")
	}
	if maxConcurrency < 0 || maxConcurrency > 1000 {
		return errors.New("最大并发必须在 0 到 1000 之间")
	}
	if rpmRateLimit < 0 || rpmRateLimit > 100000 {
		return errors.New("RPM 必须在 0 到 100000 之间")
	}
	return nil
}

func buildTokenSaleResponses(sales []model.TokenSale) ([]tokenSaleResponse, error) {
	tokenIds := make([]int, 0, len(sales))
	for _, sale := range sales {
		tokenIds = append(tokenIds, sale.TokenId)
	}
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()
	sevenDayStart := now.AddDate(0, 0, -7).Unix()
	todayUsage, err := model.GetTokenSaleUsage(tokenIds, todayStart)
	if err != nil {
		return nil, err
	}
	sevenDayUsage, err := model.GetTokenSaleUsage(tokenIds, sevenDayStart)
	if err != nil {
		return nil, err
	}

	items := make([]tokenSaleResponse, 0, len(sales))
	for _, sale := range sales {
		allowIps := ""
		if sale.Token.AllowIps != nil {
			allowIps = *sale.Token.AllowIps
		}
		today := todayUsage[sale.TokenId]
		sevenDay := sevenDayUsage[sale.TokenId]
		lastUsedAt := sale.Token.AccessedTime
		if sevenDay.LastUsedAt > lastUsedAt {
			lastUsedAt = sevenDay.LastUsedAt
		}
		items = append(items, tokenSaleResponse{
			ID:                   sale.ID,
			TokenId:              sale.TokenId,
			Name:                 sale.Token.Name,
			Key:                  sale.Token.GetMaskedKey(),
			Buyer:                sale.Buyer,
			OrderNo:              sale.OrderNo,
			AmountMinor:          sale.AmountMinor,
			Currency:             sale.Currency,
			GrantedQuota:         sale.GrantedQuota,
			UsedQuota:            sale.Token.UsedQuota,
			RemainingQuota:       sale.Token.RemainQuota,
			ExpiredTime:          sale.Token.ExpiredTime,
			AccessedTime:         sale.Token.AccessedTime,
			TokenStatus:          sale.Token.Status,
			SaleStatus:           sale.Status,
			MaxConcurrency:       sale.Token.MaxConcurrency,
			RPMRateLimit:         sale.Token.RPMRateLimit,
			ModelLimitsEnabled:   sale.Token.ModelLimitsEnabled,
			ModelLimits:          sale.Token.ModelLimits,
			AllowIps:             allowIps,
			Group:                sale.Token.Group,
			CrossGroupRetry:      sale.Token.CrossGroupRetry,
			Note:                 sale.Note,
			DeliveredAt:          sale.DeliveredAt,
			CreatedAt:            sale.CreatedAt,
			TodayRequestCount:    today.RequestCount,
			TodayQuota:           today.Quota,
			SevenDayRequestCount: sevenDay.RequestCount,
			SevenDayQuota:        sevenDay.Quota,
			LastUsedAt:           lastUsedAt,
		})
	}
	return items, nil
}

func ListTokenSales(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	sales, total, err := model.GetTokenSales(c.Query("keyword"), c.Query("status"), pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items, err := buildTokenSaleResponses(sales)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func GetTokenSaleOverview(c *gin.Context) {
	summary, err := model.GetTokenSaleSummary()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, summary)
}

func GetTokenSaleUsage(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	sale, err := model.GetTokenSaleById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	usage, err := model.GetTokenSaleModelUsage(sale.TokenId, time.Now().AddDate(0, 0, -30).Unix())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, usage)
}

func GetTokenSaleKey(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	sale, err := model.GetTokenSaleById(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "销售 Key 不存在"})
			return
		}
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"key": "sk-" + sale.Token.GetFullKey()})
}

func CreateTokenSale(c *gin.Context) {
	var request createTokenSaleRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	if request.ValidityDays < 1 || request.ValidityDays > 3650 {
		common.ApiError(c, errors.New("有效天数必须在 1 到 3650 天之间"))
		return
	}
	now := common.GetTimestamp()
	expiredTime := now + int64(request.ValidityDays)*24*60*60
	if err := validateTokenSaleInput(request.Name, request.AmountMinor, request.GrantedQuota, expiredTime, request.MaxConcurrency, request.RPMRateLimit); err != nil {
		common.ApiError(c, err)
		return
	}
	key, err := common.GenerateKey()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	token := &model.Token{
		UserId:             c.GetInt("id"),
		Key:                key,
		Status:             common.TokenStatusEnabled,
		Name:               strings.TrimSpace(request.Name),
		CreatedTime:        now,
		AccessedTime:       now,
		ExpiredTime:        expiredTime,
		RemainQuota:        request.GrantedQuota,
		UnlimitedQuota:     false,
		ModelLimitsEnabled: request.ModelLimitsEnabled,
		ModelLimits:        strings.TrimSpace(request.ModelLimits),
		AllowIps:           request.AllowIps,
		Group:              strings.TrimSpace(request.Group),
		CrossGroupRetry:    request.CrossGroupRetry,
		MaxConcurrency:     request.MaxConcurrency,
		RPMRateLimit:       request.RPMRateLimit,
	}
	sale := &model.TokenSale{
		Buyer:        strings.TrimSpace(request.Buyer),
		OrderNo:      strings.TrimSpace(request.OrderNo),
		AmountMinor:  request.AmountMinor,
		Currency:     "CNY",
		GrantedQuota: request.GrantedQuota,
		Note:         strings.TrimSpace(request.Note),
		CreatedBy:    c.GetInt("id"),
		UpdatedBy:    c.GetInt("id"),
	}
	if err := model.CreateTokenSale(token, sale); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"id":           sale.ID,
			"token_id":     token.Id,
			"key":          "sk-" + token.Key,
			"expired_time": expiredTime,
		},
	})
}

func UpdateTokenSale(c *gin.Context) {
	var request updateTokenSaleRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := validateTokenSaleInput(request.Name, request.AmountMinor, request.GrantedQuota, request.ExpiredTime, request.MaxConcurrency, request.RPMRateLimit); err != nil {
		common.ApiError(c, err)
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	sale, err := model.GetTokenSaleById(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "销售 Key 不存在"})
			return
		}
		common.ApiError(c, err)
		return
	}
	if request.GrantedQuota < sale.Token.UsedQuota {
		common.ApiError(c, fmt.Errorf("总额度不能低于已使用额度 %d", sale.Token.UsedQuota))
		return
	}
	if request.Status != model.TokenSaleStatusActive && request.Status != model.TokenSaleStatusClosed {
		common.ApiError(c, errors.New("无效的销售 Key 状态"))
		return
	}
	sale.Buyer = request.Buyer
	sale.OrderNo = request.OrderNo
	sale.AmountMinor = request.AmountMinor
	sale.GrantedQuota = request.GrantedQuota
	sale.Status = request.Status
	sale.Note = request.Note
	sale.UpdatedBy = c.GetInt("id")
	sale.Token.Name = strings.TrimSpace(request.Name)
	sale.Token.Status = common.TokenStatusEnabled
	if request.Status == model.TokenSaleStatusClosed {
		sale.Token.Status = common.TokenStatusDisabled
	}
	sale.Token.ExpiredTime = request.ExpiredTime
	sale.Token.RemainQuota = request.GrantedQuota - sale.Token.UsedQuota
	sale.Token.UnlimitedQuota = false
	sale.Token.MaxConcurrency = request.MaxConcurrency
	sale.Token.RPMRateLimit = request.RPMRateLimit
	sale.Token.ModelLimitsEnabled = request.ModelLimitsEnabled
	sale.Token.ModelLimits = strings.TrimSpace(request.ModelLimits)
	sale.Token.AllowIps = request.AllowIps
	sale.Token.Group = strings.TrimSpace(request.Group)
	sale.Token.CrossGroupRetry = request.CrossGroupRetry
	if err := model.UpdateTokenSale(sale); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func DeleteTokenSale(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.DeleteTokenSale(id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "销售 Key 不存在"})
			return
		}
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}
