package model

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

const (
	TokenSaleStatusActive = "active"
	TokenSaleStatusClosed = "closed"
)

type TokenSale struct {
	ID           int64  `json:"id" gorm:"primaryKey"`
	TokenId      int    `json:"token_id" gorm:"uniqueIndex"`
	UserId       int    `json:"user_id" gorm:"index"`
	Buyer        string `json:"buyer" gorm:"type:varchar(128);index"`
	OrderNo      string `json:"order_no" gorm:"type:varchar(191);index"`
	AmountMinor  int64  `json:"amount_minor" gorm:"bigint"`
	Currency     string `json:"currency" gorm:"type:varchar(8)"`
	GrantedQuota int    `json:"granted_quota"`
	Status       string `json:"status" gorm:"type:varchar(32);index"`
	Note         string `json:"note" gorm:"type:varchar(500)"`
	DeliveredAt  int64  `json:"delivered_at" gorm:"bigint;index"`
	CreatedBy    int    `json:"created_by"`
	UpdatedBy    int    `json:"updated_by"`
	CreatedAt    int64  `json:"created_at" gorm:"bigint;index"`
	UpdatedAt    int64  `json:"updated_at" gorm:"bigint;index"`
	Token        Token  `json:"token" gorm:"foreignKey:TokenId"`
}

type TokenSaleUsage struct {
	TokenId      int   `json:"token_id" gorm:"column:token_id"`
	RequestCount int64 `json:"request_count" gorm:"column:request_count"`
	Quota        int64 `json:"quota" gorm:"column:quota"`
	LastUsedAt   int64 `json:"last_used_at" gorm:"column:last_used_at"`
}

type TokenSaleSummary struct {
	TotalKeys      int64 `json:"total_keys"`
	ActiveKeys     int64 `json:"active_keys"`
	RevenueMinor   int64 `json:"revenue_minor"`
	GrantedQuota   int64 `json:"granted_quota"`
	UsedQuota      int64 `json:"used_quota"`
	RemainingQuota int64 `json:"remaining_quota"`
}

type TokenSaleModelUsage struct {
	ModelName        string  `json:"model_name"`
	RequestCount     int64   `json:"request_count"`
	Quota            int64   `json:"quota"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	ModelRatio       float64 `json:"model_ratio"`
	GroupRatio       float64 `json:"group_ratio"`
	BillingMode      string  `json:"billing_mode"`
	MatchedTier      string  `json:"matched_tier"`
}

func (sale *TokenSale) BeforeCreate(_ *gorm.DB) error {
	now := common.GetTimestamp()
	if sale.Currency == "" {
		sale.Currency = "CNY"
	}
	if sale.Status == "" {
		sale.Status = TokenSaleStatusActive
	}
	if sale.DeliveredAt == 0 {
		sale.DeliveredAt = now
	}
	sale.CreatedAt = now
	sale.UpdatedAt = now
	return nil
}

func CreateTokenSale(token *Token, sale *TokenSale) error {
	if token == nil || sale == nil {
		return errors.New("token sale data is required")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(token).Error; err != nil {
			return err
		}
		sale.TokenId = token.Id
		sale.UserId = token.UserId
		return tx.Create(sale).Error
	})
}

func GetTokenSales(keyword string, status string, pageInfo *common.PageInfo) ([]TokenSale, int64, error) {
	query := DB.Model(&TokenSale{}).Joins("JOIN tokens ON tokens.id = token_sales.token_id")
	keyword = strings.TrimSpace(keyword)
	if keyword != "" {
		pattern := "%" + keyword + "%"
		query = query.Where("token_sales.buyer LIKE ? OR token_sales.order_no LIKE ? OR tokens.name LIKE ?", pattern, pattern, pattern)
	}
	if status != "" {
		query = query.Where("token_sales.status = ?", status)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var sales []TokenSale
	err := query.Preload("Token").Order("token_sales.id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&sales).Error
	return sales, total, err
}

func GetTokenSaleById(id int64) (*TokenSale, error) {
	var sale TokenSale
	if err := DB.Preload("Token").First(&sale, id).Error; err != nil {
		return nil, err
	}
	return &sale, nil
}

func UpdateTokenSale(sale *TokenSale) error {
	if sale == nil || sale.ID == 0 || sale.TokenId == 0 {
		return errors.New("invalid token sale")
	}
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&Token{}).Where("id = ?", sale.TokenId).Select(
			"name", "status", "expired_time", "remain_quota", "unlimited_quota",
			"model_limits_enabled", "model_limits", "allow_ips", "group", "cross_group_retry",
			"max_concurrency", "rpm_rate_limit",
		).Updates(&sale.Token).Error; err != nil {
			return err
		}
		return tx.Model(&TokenSale{}).Where("id = ?", sale.ID).Updates(map[string]any{
			"buyer":         strings.TrimSpace(sale.Buyer),
			"order_no":      strings.TrimSpace(sale.OrderNo),
			"amount_minor":  sale.AmountMinor,
			"granted_quota": sale.GrantedQuota,
			"status":        sale.Status,
			"note":          strings.TrimSpace(sale.Note),
			"updated_by":    sale.UpdatedBy,
			"updated_at":    common.GetTimestamp(),
		}).Error
	})
	if err != nil {
		return err
	}
	if common.RedisEnabled {
		if err := cacheSetToken(sale.Token); err != nil {
			common.SysLog("failed to refresh token sale cache: " + err.Error())
		}
	}
	return nil
}

func DeleteTokenSale(id int64) error {
	if id <= 0 {
		return errors.New("invalid token sale id")
	}
	var token Token
	err := DB.Transaction(func(tx *gorm.DB) error {
		var sale TokenSale
		if err := tx.First(&sale, id).Error; err != nil {
			return err
		}
		if err := tx.First(&token, sale.TokenId).Error; err != nil {
			return err
		}
		if err := tx.Delete(&sale).Error; err != nil {
			return err
		}
		return tx.Delete(&token).Error
	})
	if err != nil {
		return err
	}
	if common.RedisEnabled {
		token.Status = common.TokenStatusDisabled
		if err := cacheSetToken(token); err != nil {
			common.SysLog("failed to disable deleted sales token cache: " + err.Error())
		}
		if err := cacheDeleteToken(token.Key); err != nil {
			common.SysLog("failed to delete sales token cache: " + err.Error())
		}
	}
	return nil
}

func GetTokenSaleUsage(tokenIds []int, since int64) (map[int]TokenSaleUsage, error) {
	result := make(map[int]TokenSaleUsage, len(tokenIds))
	if len(tokenIds) == 0 {
		return result, nil
	}
	var rows []TokenSaleUsage
	err := LOG_DB.Model(&Log{}).
		Select("token_id, count(*) as request_count, coalesce(sum(quota), 0) as quota, max(created_at) as last_used_at").
		Where("type = ? AND token_id IN ? AND created_at >= ?", LogTypeConsume, tokenIds, since).
		Group("token_id").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.TokenId] = row
	}
	return result, nil
}

func GetTokenSaleSummary() (*TokenSaleSummary, error) {
	summary := &TokenSaleSummary{}
	err := DB.Model(&TokenSale{}).
		Joins("JOIN tokens ON tokens.id = token_sales.token_id").
		Select(`
			COUNT(*) AS total_keys,
			COALESCE(SUM(CASE WHEN token_sales.status = ? AND tokens.status = ? THEN 1 ELSE 0 END), 0) AS active_keys,
			COALESCE(SUM(token_sales.amount_minor), 0) AS revenue_minor,
			COALESCE(SUM(token_sales.granted_quota), 0) AS granted_quota,
			COALESCE(SUM(tokens.used_quota), 0) AS used_quota,
			COALESCE(SUM(tokens.remain_quota), 0) AS remaining_quota
		`, TokenSaleStatusActive, common.TokenStatusEnabled).
		Scan(summary).Error
	return summary, err
}

func GetTokenSaleModelUsage(tokenId int, since int64) ([]TokenSaleModelUsage, error) {
	var result []TokenSaleModelUsage
	err := LOG_DB.Model(&Log{}).
		Select(`
			model_name,
			COUNT(*) AS request_count,
			COALESCE(SUM(quota), 0) AS quota,
			COALESCE(SUM(prompt_tokens), 0) AS prompt_tokens,
			COALESCE(SUM(completion_tokens), 0) AS completion_tokens
		`).
		Where("type = ? AND token_id = ? AND created_at >= ?", LogTypeConsume, tokenId, since).
		Group("model_name").
		Order("quota desc").
		Scan(&result).Error
	if err != nil {
		return nil, err
	}

	latestLogTimes := LOG_DB.Model(&Log{}).
		Select("model_name, MAX(created_at) AS max_created_at").
		Where("type = ? AND token_id = ? AND created_at >= ?", LogTypeConsume, tokenId, since).
		Group("model_name")
	var metadataLogs []Log
	if err := LOG_DB.Table("logs AS usage_log").
		Select("usage_log.model_name, usage_log.other").
		Joins("JOIN (?) AS latest ON latest.model_name = usage_log.model_name AND latest.max_created_at = usage_log.created_at", latestLogTimes).
		Where("usage_log.type = ? AND usage_log.token_id = ? AND usage_log.created_at >= ?", LogTypeConsume, tokenId, since).
		Order("usage_log.id desc").
		Find(&metadataLogs).Error; err != nil {
		return nil, err
	}

	metadataByModel := make(map[string]map[string]any, len(metadataLogs))
	for _, log := range metadataLogs {
		if metadataByModel[log.ModelName] != nil {
			continue
		}
		metadata, _ := common.StrToMap(log.Other)
		metadataByModel[log.ModelName] = metadata
	}
	for index := range result {
		metadata := metadataByModel[result[index].ModelName]
		if metadata == nil {
			continue
		}
		result[index].ModelRatio, _ = strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(metadata["model_ratio"])), 64)
		result[index].GroupRatio, _ = strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(metadata["group_ratio"])), 64)
		result[index].BillingMode, _ = metadata["billing_mode"].(string)
		result[index].MatchedTier, _ = metadata["matched_tier"].(string)
	}
	return result, nil
}
