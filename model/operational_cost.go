package model

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	OperationalAssetTypeCPAAccount     = "cpa_account"
	OperationalAssetTypeSub2APIAccount = "sub2api_account"
	OperationalAssetTypeChannel        = "channel"

	OperationalAssetStateActive   = "active"
	OperationalAssetStateArchived = "archived"

	CPAAccountPoolImported  = "cpa_import"
	CPAAccountPoolTemporary = "temporary"
	CPAAccountPoolOfficial  = "official_login"
)

type OperationalAsset struct {
	ID            int64  `json:"id" gorm:"primaryKey"`
	SourceType    string `json:"source_type" gorm:"type:varchar(32);index;uniqueIndex:uk_operational_asset_source,priority:1"`
	SourceKey     string `json:"source_key" gorm:"type:varchar(191);uniqueIndex:uk_operational_asset_source,priority:2"`
	SourceID      int64  `json:"source_id" gorm:"bigint;index"`
	SourceRef     string `json:"-" gorm:"type:varchar(255);index"`
	DisplayName   string `json:"display_name" gorm:"type:varchar(255);index"`
	PoolType      string `json:"pool_type,omitempty" gorm:"type:varchar(32);index"`
	State         string `json:"state" gorm:"type:varchar(32);index"`
	CostMinor     int64  `json:"cost_minor" gorm:"bigint"`
	Currency      string `json:"currency" gorm:"type:varchar(8)"`
	CostDate      int64  `json:"cost_date" gorm:"bigint;index"`
	CostNote      string `json:"cost_note" gorm:"type:varchar(500)"`
	Snapshot      string `json:"snapshot" gorm:"type:text"`
	ArchivedAt    int64  `json:"archived_at" gorm:"bigint;index"`
	ArchiveReason string `json:"archive_reason" gorm:"type:varchar(500)"`
	LastSeenAt    int64  `json:"last_seen_at" gorm:"bigint;index"`
	CreatedBy     int    `json:"created_by"`
	UpdatedBy     int    `json:"updated_by"`
	CreatedAt     int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt     int64  `json:"updated_at" gorm:"bigint;index"`
}

type OperationalCostEntry struct {
	ID          int64  `json:"id" gorm:"primaryKey"`
	Title       string `json:"title" gorm:"type:varchar(255);index"`
	Category    string `json:"category" gorm:"type:varchar(64);index"`
	AmountMinor int64  `json:"amount_minor" gorm:"bigint"`
	Currency    string `json:"currency" gorm:"type:varchar(8)"`
	OccurredAt  int64  `json:"occurred_at" gorm:"bigint;index"`
	Note        string `json:"note" gorm:"type:varchar(500)"`
	CreatedBy   int    `json:"created_by"`
	UpdatedBy   int    `json:"updated_by"`
	VoidedAt    int64  `json:"voided_at" gorm:"bigint;index"`
	CreatedAt   int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt   int64  `json:"updated_at" gorm:"bigint;index"`
}

type OperationalCostSummary struct {
	CurrentAssetCost  int64 `json:"current_asset_cost"`
	ArchivedAssetCost int64 `json:"archived_asset_cost"`
	CustomCost        int64 `json:"custom_cost"`
	TotalCost         int64 `json:"total_cost"`
	CurrentAssets     int64 `json:"current_assets"`
	ArchivedAssets    int64 `json:"archived_assets"`
}

func (asset *OperationalAsset) BeforeCreate(_ *gorm.DB) error {
	now := common.GetTimestamp()
	if asset.State == "" {
		asset.State = OperationalAssetStateActive
	}
	if asset.Currency == "" {
		asset.Currency = "CNY"
	}
	if asset.CreatedAt == 0 {
		asset.CreatedAt = now
	}
	asset.UpdatedAt = now
	return nil
}

func (entry *OperationalCostEntry) BeforeCreate(_ *gorm.DB) error {
	now := common.GetTimestamp()
	if entry.Currency == "" {
		entry.Currency = "CNY"
	}
	if entry.OccurredAt == 0 {
		entry.OccurredAt = now
	}
	entry.CreatedAt = now
	entry.UpdatedAt = now
	return nil
}

func OperationalCPAAssetKey(authIndex string) string {
	hash := sha256.Sum256([]byte(strings.TrimSpace(authIndex)))
	return "cpa:" + hex.EncodeToString(hash[:8])
}

func OperationalChannelAssetKey(channelID int) string {
	return "channel:" + strconv.Itoa(channelID)
}

func OperationalSub2APIAssetKey(accountID int64) string {
	return "sub2api:" + strconv.FormatInt(accountID, 10)
}

func UpsertOperationalAssetSeen(asset *OperationalAsset) error {
	if asset == nil || asset.SourceType == "" || asset.SourceKey == "" {
		return errors.New("operational asset source is required")
	}
	now := common.GetTimestamp()
	asset.LastSeenAt = now
	asset.UpdatedAt = now
	return DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "source_type"}, {Name: "source_key"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"source_id", "source_ref", "display_name", "last_seen_at", "updated_at",
		}),
	}).Create(asset).Error
}

func GetOperationalAsset(sourceType string, sourceKey string) (*OperationalAsset, error) {
	var asset OperationalAsset
	err := DB.Where("source_type = ? AND source_key = ?", sourceType, sourceKey).First(&asset).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &asset, err
}

func GetOperationalAssetsByKeys(sourceType string, sourceKeys []string) (map[string]OperationalAsset, error) {
	result := make(map[string]OperationalAsset, len(sourceKeys))
	if len(sourceKeys) == 0 {
		return result, nil
	}
	var assets []OperationalAsset
	if err := DB.Where("source_type = ? AND source_key IN ?", sourceType, sourceKeys).Find(&assets).Error; err != nil {
		return nil, err
	}
	for _, asset := range assets {
		result[asset.SourceKey] = asset
	}
	return result, nil
}

func SetCPAOperationalAssetPool(asset *OperationalAsset, poolType string, userID int) error {
	if asset == nil || asset.SourceKey == "" {
		return errors.New("CPA operational asset is required")
	}
	if poolType != CPAAccountPoolImported && poolType != CPAAccountPoolTemporary && poolType != CPAAccountPoolOfficial {
		return errors.New("invalid CPA account pool")
	}
	asset.SourceType = OperationalAssetTypeCPAAccount
	if err := UpsertOperationalAssetSeen(asset); err != nil {
		return err
	}
	return DB.Model(&OperationalAsset{}).
		Where("source_type = ? AND source_key = ?", OperationalAssetTypeCPAAccount, asset.SourceKey).
		Updates(map[string]any{
			"pool_type":  poolType,
			"updated_by": userID,
			"updated_at": common.GetTimestamp(),
		}).Error
}

func SetOperationalAssetCost(asset *OperationalAsset, costMinor int64, costDate int64, note string, userID int) error {
	if costMinor < 0 {
		return errors.New("asset cost cannot be negative")
	}
	if costDate == 0 {
		costDate = common.GetTimestamp()
	}
	existing, err := GetOperationalAsset(asset.SourceType, asset.SourceKey)
	if err != nil {
		return err
	}
	if existing == nil {
		if err := UpsertOperationalAssetSeen(asset); err != nil {
			return err
		}
	}
	return DB.Model(&OperationalAsset{}).
		Where("source_type = ? AND source_key = ?", asset.SourceType, asset.SourceKey).
		Updates(map[string]any{
			"cost_minor": costMinor,
			"currency":   "CNY",
			"cost_date":  costDate,
			"cost_note":  strings.TrimSpace(note),
			"updated_by": userID,
			"updated_at": common.GetTimestamp(),
		}).Error
}

func ArchiveOperationalAsset(asset *OperationalAsset, snapshot string, reason string, userID int) error {
	if err := UpsertOperationalAssetSeen(asset); err != nil {
		return err
	}
	now := common.GetTimestamp()
	return DB.Model(&OperationalAsset{}).
		Where("source_type = ? AND source_key = ?", asset.SourceType, asset.SourceKey).
		Updates(map[string]any{
			"state":          OperationalAssetStateArchived,
			"snapshot":       snapshot,
			"archived_at":    now,
			"archive_reason": strings.TrimSpace(reason),
			"updated_by":     userID,
			"updated_at":     now,
		}).Error
}

func RestoreOperationalAsset(id int64, userID int) (*OperationalAsset, error) {
	var asset OperationalAsset
	if err := DB.First(&asset, id).Error; err != nil {
		return nil, err
	}
	err := DB.Model(&OperationalAsset{}).Where("id = ?", id).Updates(map[string]any{
		"state":          OperationalAssetStateActive,
		"archived_at":    0,
		"archive_reason": "",
		"updated_by":     userID,
		"updated_at":     common.GetTimestamp(),
	}).Error
	if err != nil {
		return nil, err
	}
	asset.State = OperationalAssetStateActive
	asset.ArchivedAt = 0
	asset.ArchiveReason = ""
	return &asset, nil
}

func ExcludeArchivedChannels(query *gorm.DB) *gorm.DB {
	subQuery := DB.Model(&OperationalAsset{}).
		Select("source_id").
		Where("source_type = ? AND state = ? AND source_id > 0", OperationalAssetTypeChannel, OperationalAssetStateArchived)
	return query.Where("id NOT IN (?)", subQuery)
}

func AttachOperationalCostsToChannels(channels []*Channel) error {
	keys := make([]string, 0, len(channels))
	for _, channel := range channels {
		if channel != nil {
			key := OperationalChannelAssetKey(channel.Id)
			keys = append(keys, key)
			if err := UpsertOperationalAssetSeen(&OperationalAsset{
				SourceType:  OperationalAssetTypeChannel,
				SourceKey:   key,
				SourceID:    int64(channel.Id),
				DisplayName: channel.Name,
			}); err != nil {
				return err
			}
		}
	}
	assets, err := GetOperationalAssetsByKeys(OperationalAssetTypeChannel, keys)
	if err != nil {
		return err
	}
	for _, channel := range channels {
		if channel == nil {
			continue
		}
		asset, ok := assets[OperationalChannelAssetKey(channel.Id)]
		if ok {
			channel.OperationalCostMinor = asset.CostMinor
			channel.OperationalAssetState = asset.State
		}
	}
	return nil
}

func IsOperationalAssetArchived(sourceType string, sourceKey string) (bool, error) {
	var count int64
	err := DB.Model(&OperationalAsset{}).
		Where("source_type = ? AND source_key = ? AND state = ?", sourceType, sourceKey, OperationalAssetStateArchived).
		Count(&count).Error
	return count > 0, err
}

func ListArchivedOperationalAssets() ([]OperationalAsset, error) {
	var assets []OperationalAsset
	err := DB.Where("state = ?", OperationalAssetStateArchived).Order("archived_at desc, id desc").Find(&assets).Error
	return assets, err
}

func ListArchivedOperationalAssetsPage(pageInfo *common.PageInfo, sourceType string, search string) ([]OperationalAsset, int64, error) {
	query := DB.Model(&OperationalAsset{}).Where("state = ?", OperationalAssetStateArchived)
	if sourceType != "" {
		query = query.Where("source_type = ?", sourceType)
	}
	if search = strings.TrimSpace(search); search != "" {
		pattern := "%" + strings.ToLower(search) + "%"
		query = query.Where(
			"(LOWER(display_name) LIKE ? OR LOWER(archive_reason) LIKE ? OR LOWER(cost_note) LIKE ?)",
			pattern, pattern, pattern,
		)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var assets []OperationalAsset
	err := query.Order("archived_at desc, id desc").
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Find(&assets).Error
	return assets, total, err
}

func GetOperationalCostSummary() (*OperationalCostSummary, error) {
	summary := &OperationalCostSummary{}
	if err := DB.Model(&OperationalAsset{}).Select(`
		COALESCE(SUM(CASE WHEN state = ? THEN cost_minor ELSE 0 END), 0) AS archived_asset_cost,
		COALESCE(SUM(CASE WHEN state <> ? THEN cost_minor ELSE 0 END), 0) AS current_asset_cost,
		COALESCE(SUM(CASE WHEN state = ? THEN 1 ELSE 0 END), 0) AS archived_assets,
		COALESCE(SUM(CASE WHEN state <> ? THEN 1 ELSE 0 END), 0) AS current_assets
	`, OperationalAssetStateArchived, OperationalAssetStateArchived, OperationalAssetStateArchived, OperationalAssetStateArchived).
		Scan(summary).Error; err != nil {
		return nil, err
	}
	if err := DB.Model(&OperationalCostEntry{}).
		Select("COALESCE(SUM(amount_minor), 0)").
		Where("voided_at = 0").
		Scan(&summary.CustomCost).Error; err != nil {
		return nil, err
	}
	summary.TotalCost = summary.CurrentAssetCost + summary.ArchivedAssetCost + summary.CustomCost
	return summary, nil
}

func ReconcileOperationalAssetsFromLocal() error {
	var channels []Channel
	if err := DB.Find(&channels).Error; err != nil {
		return err
	}
	for _, channel := range channels {
		if err := UpsertOperationalAssetSeen(&OperationalAsset{
			SourceType:  OperationalAssetTypeChannel,
			SourceKey:   OperationalChannelAssetKey(channel.Id),
			SourceID:    int64(channel.Id),
			DisplayName: channel.Name,
		}); err != nil {
			return err
		}
	}

	var targets []ActivationTarget
	if err := DB.Where("source_type = ?", ActivationSourceCPAAccount).Find(&targets).Error; err != nil {
		return err
	}
	for _, target := range targets {
		if err := UpsertOperationalAssetSeen(&OperationalAsset{
			SourceType:  OperationalAssetTypeCPAAccount,
			SourceKey:   target.TargetKey,
			SourceRef:   target.SourceID,
			DisplayName: target.DisplayName,
		}); err != nil {
			return err
		}
	}
	return nil
}

func ListOperationalCostEntries() ([]OperationalCostEntry, error) {
	var entries []OperationalCostEntry
	err := DB.Where("voided_at = 0").Order("occurred_at desc, id desc").Find(&entries).Error
	return entries, err
}

func ListOperationalCostEntriesPage(pageInfo *common.PageInfo) ([]OperationalCostEntry, int64, error) {
	query := DB.Model(&OperationalCostEntry{}).Where("voided_at = 0")
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var entries []OperationalCostEntry
	err := query.Order("occurred_at desc, id desc").
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Find(&entries).Error
	return entries, total, err
}

func CreateOperationalCostEntry(entry *OperationalCostEntry) error {
	return DB.Create(entry).Error
}

func UpdateOperationalCostEntry(entry *OperationalCostEntry, userID int) error {
	return DB.Model(&OperationalCostEntry{}).Where("id = ? AND voided_at = 0", entry.ID).Updates(map[string]any{
		"title":        strings.TrimSpace(entry.Title),
		"category":     strings.TrimSpace(entry.Category),
		"amount_minor": entry.AmountMinor,
		"occurred_at":  entry.OccurredAt,
		"note":         strings.TrimSpace(entry.Note),
		"updated_by":   userID,
		"updated_at":   common.GetTimestamp(),
	}).Error
}

func VoidOperationalCostEntry(id int64, userID int) error {
	now := common.GetTimestamp()
	return DB.Model(&OperationalCostEntry{}).Where("id = ? AND voided_at = 0", id).Updates(map[string]any{
		"voided_at":  now,
		"updated_by": userID,
		"updated_at": now,
	}).Error
}
